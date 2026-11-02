package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"

	"github.com/gin-gonic/gin"
)

// 活动授权的双类型限流：rpm 按固定窗口限每分钟请求数，concurrency 限制同时在途请求数。
// 两者都按「用户 + 实际分组」聚合计数：同一用户在同一分组的多张授权（含多令牌）共享额度，
// 授权续期、换码、合并都不会改变计数 key。

const (
	// activityRateLimitWindowSeconds 是活动 RPM 的固定窗口长度：一分钟。
	activityRateLimitWindowSeconds = 60
	// activityConcurrencyLeaseSeconds 是并发槽的租约时长：请求异常退出后
	// 槽位最迟 60 秒自动归还，不会永久占用额度。
	activityConcurrencyLeaseSeconds = 60
	// activityConcurrencyRenewIntervalSeconds 是续租周期：长流请求每 20 秒续租一次，
	// 小于租约时长的一半；续租一旦失败立即判定失租并取消请求（没有重试窗口）。
	activityConcurrencyRenewIntervalSeconds = 20
	// activityConcurrencyKeyTTLSeconds 是并发计数 key 的 TTL，至少覆盖两倍租约时长，
	// 保证 key 不会在租约有效期内消失。
	activityConcurrencyKeyTTLSeconds = 120
	// activityLimitReleaseTimeout 是获取/续租/释放这类限流状态操作的独立超时，
	// 不依赖请求 context：请求可能已被取消，而释放必须完成。
	activityLimitReleaseTimeout = 2 * time.Second
)

// 活动限流拒绝的错误码，随响应体返回给客户端。
const (
	ActivityLimitCodeRpmExceeded            = "activity_rpm_exceeded"
	ActivityLimitCodeConcurrencyExceeded    = "activity_concurrency_exceeded"
	ActivityLimitCodeConcurrencyUnavailable = "activity_concurrency_unavailable"
)

// ActivityLimitError 是活动限流的拒绝结果：
// 429 表示额度已用尽（RPM 超限带窗口剩余秒数的 Retry-After，并发超限不伪造），
// 503 表示暂时无法验证限制（Redis/数据库故障、失租时 fail closed，绝不放行）。
type ActivityLimitError struct {
	StatusCode        int
	Code              string
	Message           string
	RetryAfterSeconds int64
}

func (e *ActivityLimitError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func activityConcurrencyExceededError(c *gin.Context) *ActivityLimitError {
	return &ActivityLimitError{
		StatusCode: http.StatusTooManyRequests,
		Code:       ActivityLimitCodeConcurrencyExceeded,
		Message:    i18n.T(c, i18n.MsgActivityConcurrencyExceeded),
	}
}

func activityConcurrencyUnavailableError(c *gin.Context) *ActivityLimitError {
	return &ActivityLimitError{
		StatusCode: http.StatusServiceUnavailable,
		Code:       ActivityLimitCodeConcurrencyUnavailable,
		Message:    i18n.T(c, i18n.MsgActivityConcurrencyUnavailable),
	}
}

func activityRateLimitExceededError(c *gin.Context, limit int, retryAfterSeconds int64) *ActivityLimitError {
	return &ActivityLimitError{
		StatusCode:        http.StatusTooManyRequests,
		Code:              ActivityLimitCodeRpmExceeded,
		Message:           i18n.T(c, i18n.MsgRateLimitReached, map[string]any{"Minutes": activityRateLimitWindowSeconds / 60, "Max": limit}),
		RetryAfterSeconds: retryAfterSeconds,
	}
}

// activityLimitStore 是活动限流的共享状态存储：RPM 固定窗口 + 并发槽租约。
// 内存与 Redis 实现都必须满足：acquire/release 幂等、limit 每次按最新值判断
// （限额下调立即对新请求生效）、release 归零后清理状态、续租不复活已丢失的租约。
type activityLimitStore interface {
	// RateTake 在窗口内取一次请求额度，返回是否允许与 Retry-After 秒数。
	RateTake(ctx context.Context, key string, limit int) (bool, int64, error)
	// Acquire 尝试占用一个并发槽，leaseID 是本次占用的唯一租约标识；同一 leaseID 重复调用幂等。
	Acquire(ctx context.Context, key string, limit int, leaseID string) (bool, error)
	// Renew 续租；只对仍存在且未过期的自己成功，不复活已丢失的租约。
	Renew(ctx context.Context, key string, leaseID string) (bool, error)
	// Release 归还并发槽，重复调用无副作用。
	Release(ctx context.Context, key string, leaseID string) error
}

var (
	activityLimitStoreMu     sync.RWMutex
	activityLimitStoreForced activityLimitStore // 测试注入
	activityLimitStoreAuto   activityLimitStore // 按 common.RedisEnabled 自动选择
)

// activityLimits 返回当前生效的活动限流存储：Redis 启用时必须走 Redis，
// Redis 出错直接 fail closed，绝不回退内存实现（两套状态并存会超发额度）。
// Redis 已启用但客户端未初始化时返回「不可用」存储，让请求 503 而不是放行。
func activityLimits() activityLimitStore {
	activityLimitStoreMu.RLock()
	forced, auto := activityLimitStoreForced, activityLimitStoreAuto
	activityLimitStoreMu.RUnlock()
	if forced != nil {
		return forced
	}
	if !common.RedisEnabled {
		if _, ok := auto.(*memoryActivityLimitStore); ok {
			return auto
		}
	} else if common.RDB == nil {
		if _, ok := auto.(*brokenActivityLimitStore); ok {
			return auto
		}
	} else if store, ok := auto.(*redisActivityLimitStore); ok && store.client == common.RDB {
		return store
	}

	activityLimitStoreMu.Lock()
	defer activityLimitStoreMu.Unlock()
	if !common.RedisEnabled {
		if _, ok := activityLimitStoreAuto.(*memoryActivityLimitStore); ok {
			return activityLimitStoreAuto
		}
		activityLimitStoreAuto = newMemoryActivityLimitStore()
		return activityLimitStoreAuto
	}
	if common.RDB == nil {
		if _, ok := activityLimitStoreAuto.(*brokenActivityLimitStore); ok {
			return activityLimitStoreAuto
		}
		activityLimitStoreAuto = &brokenActivityLimitStore{}
		return activityLimitStoreAuto
	}
	if store, ok := activityLimitStoreAuto.(*redisActivityLimitStore); ok && store.client == common.RDB {
		return store
	}
	activityLimitStoreAuto = newRedisActivityLimitStore(common.RDB)
	return activityLimitStoreAuto
}

// setActivityLimitStoreForTest 注入测试用存储，返回恢复函数。
func setActivityLimitStoreForTest(store activityLimitStore) func() {
	activityLimitStoreMu.Lock()
	previous := activityLimitStoreForced
	activityLimitStoreForced = store
	activityLimitStoreMu.Unlock()
	return func() {
		activityLimitStoreMu.Lock()
		activityLimitStoreForced = previous
		activityLimitStoreMu.Unlock()
	}
}

// ---------------------------------------------------------------------------
// 不可用存储（fail closed）
// ---------------------------------------------------------------------------

// brokenActivityLimitStore 用于「Redis 已启用但客户端不可用」：所有操作都报错，
// 请求 fail closed 为 503。绝不能悄悄换用内存实现——多实例部署下那会超发额度。
type brokenActivityLimitStore struct{}

func (s *brokenActivityLimitStore) RateTake(context.Context, string, int) (bool, int64, error) {
	return false, 0, errors.New("activity limit store unavailable: Redis is enabled but its client is not initialized")
}

func (s *brokenActivityLimitStore) Acquire(context.Context, string, int, string) (bool, error) {
	return false, errors.New("activity limit store unavailable: Redis is enabled but its client is not initialized")
}

func (s *brokenActivityLimitStore) Renew(context.Context, string, string) (bool, error) {
	return false, errors.New("activity limit store unavailable: Redis is enabled but its client is not initialized")
}

func (s *brokenActivityLimitStore) Release(context.Context, string, string) error {
	return errors.New("activity limit store unavailable: Redis is enabled but its client is not initialized")
}

// ---------------------------------------------------------------------------
// 内存实现
// ---------------------------------------------------------------------------

// memoryActivityLimitStore 是进程内实现：RPM 沿用 common.InMemoryRateLimiter 的
// 滑动窗口算法，与既有活动 RPM 行为保持一致；并发槽是 mutex 计数（动态 limit、
// 幂等 acquire/release、归零清理），槽只在请求走完统一释放时归还，不做时间过期——
// 进程内请求一定伴随 handler 结束而释放，进程崩溃时内存自身清零；若按时间过期，
// 被调度暂停的长请求会把还在用的槽判死并超发。多实例部署请启用 Redis。
type memoryActivityLimitStore struct {
	rates *common.InMemoryRateLimiter

	mu     sync.Mutex
	leases map[string]map[string]struct{} // key -> leaseID 集合
}

func newMemoryActivityLimitStore() *memoryActivityLimitStore {
	rates := &common.InMemoryRateLimiter{}
	rates.Init(common.RateLimitKeyExpirationDuration)
	return &memoryActivityLimitStore{
		rates:  rates,
		leases: make(map[string]map[string]struct{}),
	}
}

func (s *memoryActivityLimitStore) RateTake(_ context.Context, key string, limit int) (bool, int64, error) {
	if key == "" {
		return false, 0, errors.New("activity rate limit key is empty")
	}
	if limit <= 0 {
		return false, 0, errors.New("activity rate limit must be positive")
	}
	// 内存窗口无法报告剩余时间，Retry-After 沿用整窗长度作为保守上界。
	if !s.rates.Request(key, limit, activityRateLimitWindowSeconds) {
		return false, activityRateLimitWindowSeconds, nil
	}
	return true, activityRateLimitWindowSeconds, nil
}

func (s *memoryActivityLimitStore) Acquire(_ context.Context, key string, limit int, leaseID string) (bool, error) {
	if key == "" || leaseID == "" {
		return false, errors.New("activity concurrency key or lease is empty")
	}
	if limit <= 0 {
		return false, errors.New("activity concurrency limit must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	leases := s.leases[key]
	if leases == nil {
		leases = make(map[string]struct{})
		s.leases[key] = leases
	}
	if _, exists := leases[leaseID]; exists {
		return true, nil // 幂等：同一租约重试不重复占槽
	}
	// 动态 limit：每次拿当前授权的限额判断，限额下调立即对新请求生效。
	if len(leases) >= limit {
		return false, nil
	}
	leases[leaseID] = struct{}{}
	return true, nil
}

func (s *memoryActivityLimitStore) Renew(_ context.Context, key string, leaseID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	leases := s.leases[key]
	if leases == nil {
		return false, nil
	}
	_, exists := leases[leaseID]
	return exists, nil // 只对仍持有槽位的自己成功；释放后不复活
}

func (s *memoryActivityLimitStore) Release(_ context.Context, key string, leaseID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	leases := s.leases[key]
	if leases == nil {
		return nil
	}
	delete(leases, leaseID)
	if len(leases) == 0 {
		delete(s.leases, key)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 请求级会话
// ---------------------------------------------------------------------------

// activitySessionContextKey 是会话在 gin context 中的存储键。
const activitySessionContextKey = "activity_concurrency_session"

type activityGroupState struct {
	rpmCounted bool
	slot       *activityConcurrencySlot
}

type activityConcurrencySlot struct {
	key     string
	leaseID string
	// store 是获取该槽时的存储实例：续租/释放必须回到同一份状态，
	// 运行中切换或重建客户端不能让旧槽泄漏在之前的存储里。
	store activityLimitStore
	stop  chan struct{}
	done  chan struct{}
}

// ActivityConcurrencySession 跟踪单个请求的活动限流状态：每个实际分组至多
// 计一次 RPM、占一个并发槽，请求结束统一释放；续租失败（失租）会取消请求。
type ActivityConcurrencySession struct {
	userId int
	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	states map[string]*activityGroupState
	lost   bool
	closed bool
}

// BeginActivityConcurrencySession 在 Distribute 之前建立请求级限流状态：
// 给请求套一层可取消的 context（并发槽失租即取消请求），并把会话挂到 gin context，
// 供实际定组后的 ActivityLimitGuard 使用。未认证请求不需要会话。
func BeginActivityConcurrencySession(c *gin.Context) *ActivityConcurrencySession {
	if c == nil || c.Request == nil {
		return nil
	}
	if existing := activitySessionFromContext(c); existing != nil {
		return existing
	}
	userId := c.GetInt("id")
	if userId <= 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	session := &ActivityConcurrencySession{
		userId: userId,
		ctx:    ctx,
		cancel: cancel,
		states: make(map[string]*activityGroupState),
	}
	c.Set(activitySessionContextKey, session)
	return session
}

func activitySessionFromContext(c *gin.Context) *ActivityConcurrencySession {
	if c == nil {
		return nil
	}
	session, _ := c.Get(activitySessionContextKey)
	typed, _ := session.(*ActivityConcurrencySession)
	return typed
}

// activitySessionForRequest 只读取由活动限流中间件建立的请求级会话，不静默创建：
// 会话必须有明确的生命周期所有者（中间件的 defer 释放），否则并发槽无人归还，
// 兜底创建的会话还会在请求取消时被提前释放。缺少会话的路由应当挂上
// ActivityGrantRateLimit 中间件，而不是在转发途中隐式创建。
func activitySessionForRequest(c *gin.Context) *ActivityConcurrencySession {
	return activitySessionFromContext(c)
}

// Release 释放本请求持有的全部并发槽并停止续租（幂等）。释放使用独立短超时
// context 与获取时的存储实例：请求 context 可能已被取消，租约仍必须归还。
func (s *ActivityConcurrencySession) Release() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	slots := make([]*activityConcurrencySlot, 0, len(s.states))
	for _, state := range s.states {
		if state.slot != nil {
			slots = append(slots, state.slot)
		}
	}
	s.states = nil
	s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	for _, slot := range slots {
		close(slot.stop)
		<-slot.done
		ctx, cancel := context.WithTimeout(context.Background(), activityLimitReleaseTimeout)
		if err := slot.store.Release(ctx, slot.key, slot.leaseID); err != nil {
			common.SysError(fmt.Sprintf("failed to release activity concurrency slot (key=%s): %s", slot.key, err.Error()))
		}
		cancel()
	}
}

// unusable 报告会话是否已不能继续放行请求：失租、已释放或请求已取消。
// guard 必须先做这个检查，否则已占槽的 fast-path 会让失租后的 retry 绕过限流。
func (s *ActivityConcurrencySession) unusable() bool {
	if s == nil {
		return true
	}
	if s.ctx.Err() != nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lost || s.closed
}

// Lost 报告本请求是否已丢失并发租约。
func (s *ActivityConcurrencySession) Lost() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lost
}

// markLost 标记失租并取消请求：没有租约的请求不允许继续占用上游。
func (s *ActivityConcurrencySession) markLost() {
	s.mu.Lock()
	if s.lost {
		s.mu.Unlock()
		return
	}
	s.lost = true
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *ActivityConcurrencySession) groupState(group string) *activityGroupState {
	if s.states == nil {
		s.states = make(map[string]*activityGroupState)
	}
	state := s.states[group]
	if state == nil {
		state = &activityGroupState{}
		s.states[group] = state
	}
	return state
}

// acquire 占用该分组的并发槽；同组已持有则直接成功（同组 retry 不重复占用）。
func (s *ActivityConcurrencySession) acquire(group string, limit int) (bool, error) {
	s.mu.Lock()
	if s.closed || s.lost {
		s.mu.Unlock()
		return false, errors.New("activity concurrency session is no longer usable")
	}
	state := s.groupState(group)
	if state.slot != nil {
		s.mu.Unlock()
		return true, nil
	}
	s.mu.Unlock()

	leaseID := common.GetUUID()
	key := activityConcurrencyKey(s.userId, group)
	store := activityLimits()
	ctx, cancel := context.WithTimeout(context.Background(), activityLimitReleaseTimeout)
	acquired, err := store.Acquire(ctx, key, limit, leaseID)
	cancel()
	if err != nil || !acquired {
		return false, err
	}

	slot := &activityConcurrencySlot{key: key, leaseID: leaseID, store: store, stop: make(chan struct{}), done: make(chan struct{})}
	s.mu.Lock()
	if s.closed || s.lost {
		s.mu.Unlock()
		go releaseActivityConcurrencySlot(slot)
		return false, errors.New("activity concurrency session is no longer usable")
	}
	state = s.groupState(group)
	if state.slot != nil {
		s.mu.Unlock()
		go releaseActivityConcurrencySlot(slot)
		return true, nil
	}
	state.slot = slot
	s.mu.Unlock()
	go s.renewLoop(slot)
	return true, nil
}

// releaseActivityConcurrencySlot 归还一个并发槽（独立短超时 context，幂等）。
func releaseActivityConcurrencySlot(slot *activityConcurrencySlot) {
	ctx, cancel := context.WithTimeout(context.Background(), activityLimitReleaseTimeout)
	defer cancel()
	if err := slot.store.Release(ctx, slot.key, slot.leaseID); err != nil {
		common.SysError(fmt.Sprintf("failed to release activity concurrency slot (key=%s): %s", slot.key, err.Error()))
	}
}

// renewLoop 长流请求按固定周期续租，直到请求结束或失租。
func (s *ActivityConcurrencySession) renewLoop(slot *activityConcurrencySlot) {
	defer close(slot.done)
	ticker := time.NewTicker(activityConcurrencyRenewIntervalSeconds * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-slot.stop:
			return
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if !s.renewSlot(slot) {
				return
			}
		}
	}
}

// renewSlot 续租一个并发槽；租约已不存在或已过期时立即标记失租并取消请求。
func (s *ActivityConcurrencySession) renewSlot(slot *activityConcurrencySlot) bool {
	ctx, cancel := context.WithTimeout(context.Background(), activityLimitReleaseTimeout)
	defer cancel()
	renewed, err := slot.store.Renew(ctx, slot.key, slot.leaseID)
	if err != nil {
		common.SysError(fmt.Sprintf("failed to renew activity concurrency lease (key=%s): %s", slot.key, err.Error()))
	}
	if err != nil || !renewed {
		s.markLost()
		return false
	}
	return true
}

func activityConcurrencyKey(userId int, group string) string {
	return fmt.Sprintf("activity:concurrency:v1:%d:%s", userId, group)
}

func activityRateKey(userId int, group string) string {
	return fmt.Sprintf("activity:rpm:v1:%d:%s", userId, group)
}

// actualActivityGroup 返回当前实际生效的分组：auto/chain 选组后是实际组，
// 其余场景是 using group，最后回退用户分组。
func actualActivityGroup(c *gin.Context) string {
	if group := common.GetContextKeyString(c, constant.ContextKeyAutoGroup); group != "" {
		return group
	}
	if group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup); group != "" {
		return group
	}
	return common.GetContextKeyString(c, constant.ContextKeyUserGroup)
}

// ActivityLimitGuard 在实际分组确定后、上游调用前执行活动限流检查，幂等：
// 同一请求对同一分组只计一次 RPM、只占一个并发槽，同组 retry 不重复计数/占用，
// 跨组 fallback 或 auto/chain 换组时对新分组各获取一次。返回非 nil 的
// *ActivityLimitError 表示拒绝；返回 nil 表示放行或该分组没有活动授权
// （普通分组不受活动限流）。
func ActivityLimitGuard(c *gin.Context, group string) *ActivityLimitError {
	if c == nil || c.Request == nil {
		return nil
	}
	if assistantops.IsInternal(c.Request.Context()) {
		return nil
	}
	userId := c.GetInt("id")
	if userId <= 0 {
		return nil
	}
	if group == "" {
		group = actualActivityGroup(c)
	}
	if group == "" {
		return nil
	}
	session := activitySessionForRequest(c)
	if session == nil {
		// 没有会话生命周期（路由漏挂活动限流中间件）：fail closed，
		// 而不是静默创建一个无人释放的会话或绕过限流。
		return activityConcurrencyUnavailableError(c)
	}
	if session.unusable() {
		// 失租/已释放/请求已取消：必须拒绝，否则已占槽的 fast-path 会放行失租后的 retry。
		return activityConcurrencyUnavailableError(c)
	}

	session.mu.Lock()
	state := session.groupState(group)
	needRPM := !state.rpmCounted
	needConcurrency := state.slot == nil
	session.mu.Unlock()
	if !needRPM && !needConcurrency {
		return nil // 同组 retry：RPM 已计数、并发槽已持有，不再重复检查
	}

	view, err := GetRequestActivityViewChecked(c, userId, group)
	if err != nil {
		// 授权查询失败绝不当作「没有授权」放行，否则数据库故障会绕过限流。
		common.SysError("failed to load activity grants for limit check: " + err.Error())
		return activityConcurrencyUnavailableError(c)
	}
	if view == nil {
		session.markRPMCounted(group)
		return nil
	}

	if needRPM && view.RpmLimit > 0 {
		allowed, retryAfter, rateErr := activityLimits().RateTake(c.Request.Context(), activityRateKey(userId, group), view.RpmLimit)
		if rateErr != nil {
			common.SysError("failed to take activity rpm window: " + rateErr.Error())
			return activityConcurrencyUnavailableError(c)
		}
		session.markRPMCounted(group)
		if !allowed {
			return activityRateLimitExceededError(c, view.RpmLimit, retryAfter)
		}
	} else if needRPM {
		session.markRPMCounted(group)
	}

	if needConcurrency && view.ConcurrencyLimit > 0 {
		acquired, acquireErr := session.acquire(group, view.ConcurrencyLimit)
		if acquireErr != nil {
			common.SysError("failed to acquire activity concurrency slot: " + acquireErr.Error())
			return activityConcurrencyUnavailableError(c)
		}
		if !acquired {
			return activityConcurrencyExceededError(c)
		}
	}
	return nil
}

func (s *ActivityConcurrencySession) markRPMCounted(group string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.states == nil {
		return
	}
	s.groupState(group).rpmCounted = true
}

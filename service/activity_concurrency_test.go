package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	goredis "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupActivityLimitTest(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, i18n.Init())
	previousDB := model.DB
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// 内存 sqlite 每个连接各自一个库：固定单连接，保证建表与查询在同一个库。
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.ActivityGrant{}))
	model.DB = db
	common.RedisEnabled = false
	restore := setActivityLimitStoreForTest(newMemoryActivityLimitStore())
	t.Cleanup(func() {
		restore()
		model.DB = previousDB
		common.RedisEnabled = previousRedis
	})
	return db
}

func createActivityLimitGrant(t *testing.T, db *gorm.DB, userId int, group, limitMode string, rpmLimit, concurrencyLimit int) {
	t.Helper()
	require.NoError(t, db.Create(&model.ActivityGrant{
		UserId: userId, CodeId: 1, Code: "act-limit", CodeName: "limit code", GroupName: group,
		RatioMode: model.ActivityRatioModeInherit,
		LimitMode: limitMode, RpmLimit: rpmLimit, ConcurrencyLimit: concurrencyLimit,
		StartTime: common.GetTimestamp(), ExpireTime: common.GetTimestamp() + 3600,
		Status: model.ActivityGrantStatusActive,
	}).Error)
}

func newActivityLimitContext(userId int, group string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", userId)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
	return c
}

// activityLimitRequest 模拟一条完整请求的限流生命周期：
// 中间件建立会话 → 实际定组后 guard → 请求结束统一释放。
type activityLimitRequest struct {
	c       *gin.Context
	session *ActivityConcurrencySession
}

func beginActivityLimitRequest(userId int, group string) *activityLimitRequest {
	c := newActivityLimitContext(userId, group)
	return &activityLimitRequest{c: c, session: BeginActivityConcurrencySession(c)}
}

func (r *activityLimitRequest) guard(group string) *ActivityLimitError {
	return ActivityLimitGuard(r.c, group)
}

func (r *activityLimitRequest) finish() {
	r.session.Release()
}

// ---------------------------------------------------------------------------
// 聚合：按限流模式分流，混合授权的两种约束都保留
// ---------------------------------------------------------------------------

func TestActivityViewAggregatesLimitModesByKind(t *testing.T) {
	tests := []struct {
		name            string
		grants          []model.ActivityGrant
		wantMode        string
		wantRpm         int
		wantConcurrency int
	}{
		{
			name: "旧RPM数值仍取最大",
			grants: []model.ActivityGrant{
				{GroupName: "g", RpmLimit: 4},
				{GroupName: "g", RpmLimit: 7},
			},
			wantMode: model.ActivityLimitModeRPM, wantRpm: 7,
		},
		{
			name: "RPM型的0不能取消并发约束",
			grants: []model.ActivityGrant{
				{GroupName: "g", LimitMode: model.ActivityLimitModeRPM, RpmLimit: 0},
				{GroupName: "g", LimitMode: model.ActivityLimitModeConcurrency, ConcurrencyLimit: 3},
			},
			wantMode: ActivityLimitModeMixed, wantRpm: 0, wantConcurrency: 3,
		},
		{
			name: "混合授权两种约束都生效",
			grants: []model.ActivityGrant{
				{GroupName: "g", LimitMode: model.ActivityLimitModeRPM, RpmLimit: 10},
				{GroupName: "g", LimitMode: model.ActivityLimitModeConcurrency, ConcurrencyLimit: 3},
			},
			wantMode: ActivityLimitModeMixed, wantRpm: 10, wantConcurrency: 3,
		},
		{
			name: "并发取最宽松",
			grants: []model.ActivityGrant{
				{GroupName: "g", LimitMode: model.ActivityLimitModeConcurrency, ConcurrencyLimit: 2},
				{GroupName: "g", LimitMode: model.ActivityLimitModeConcurrency, ConcurrencyLimit: 5},
			},
			wantMode: model.ActivityLimitModeConcurrency, wantConcurrency: 5,
		},
		{
			name: "显式并发0胜出",
			grants: []model.ActivityGrant{
				{GroupName: "g", LimitMode: model.ActivityLimitModeConcurrency, ConcurrencyLimit: 0},
				{GroupName: "g", LimitMode: model.ActivityLimitModeConcurrency, ConcurrencyLimit: 5},
			},
			wantMode: model.ActivityLimitModeConcurrency, wantConcurrency: 0,
		},
		{
			name: "concurrency型未启用的RPM数值不参与聚合",
			grants: []model.ActivityGrant{
				{GroupName: "g", LimitMode: model.ActivityLimitModeConcurrency, RpmLimit: 100, ConcurrencyLimit: 1},
			},
			wantMode: model.ActivityLimitModeConcurrency, wantRpm: 0, wantConcurrency: 1,
		},
		{
			name: "非法模式按RPM保守处理",
			grants: []model.ActivityGrant{
				{GroupName: "g", LimitMode: "broken", RpmLimit: 6},
			},
			wantMode: model.ActivityLimitModeRPM, wantRpm: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			views := mergeActivityViews(tt.grants)
			require.Len(t, views, 1)
			assert.Equal(t, tt.wantMode, views[0].LimitMode)
			assert.Equal(t, tt.wantRpm, views[0].RpmLimit)
			assert.Equal(t, tt.wantConcurrency, views[0].ConcurrencyLimit)
		})
	}
}

// ---------------------------------------------------------------------------
// 内存存储：动态 limit、幂等 release、归零清理、租约到期
// ---------------------------------------------------------------------------

func TestMemoryActivityLimitStoreDynamicLimitAndRelease(t *testing.T) {
	store := newMemoryActivityLimitStore()
	ctx := t.Context()

	acquired, err := store.Acquire(ctx, "k", 2, "lease-1")
	require.NoError(t, err)
	assert.True(t, acquired)
	acquired, err = store.Acquire(ctx, "k", 2, "lease-2")
	require.NoError(t, err)
	assert.True(t, acquired)
	acquired, err = store.Acquire(ctx, "k", 2, "lease-3")
	require.NoError(t, err)
	assert.False(t, acquired, "达到并发上限后拒绝")

	// 同一租约重试幂等：不重复占槽。
	acquired, err = store.Acquire(ctx, "k", 2, "lease-1")
	require.NoError(t, err)
	assert.True(t, acquired)

	// 限额下降：新请求按新限额判断，已持有的槽继续有效。
	acquired, err = store.Acquire(ctx, "k", 1, "lease-4")
	require.NoError(t, err)
	assert.False(t, acquired, "限额下调后不再发放新槽")

	// 幂等 release：同一租约重复释放只归还一个槽。
	require.NoError(t, store.Release(ctx, "k", "lease-1"))
	require.NoError(t, store.Release(ctx, "k", "lease-1"))
	acquired, err = store.Acquire(ctx, "k", 1, "lease-5")
	require.NoError(t, err)
	assert.False(t, acquired, "重复释放不能多还一个槽（仍占 1 个）")

	require.NoError(t, store.Release(ctx, "k", "lease-2"))
	acquired, err = store.Acquire(ctx, "k", 1, "lease-6")
	require.NoError(t, err)
	assert.True(t, acquired)

	// 归零删除：槽位全部归还后不残留状态。
	require.NoError(t, store.Release(ctx, "k", "lease-6"))
	store.mu.Lock()
	remaining := len(store.leases)
	store.mu.Unlock()
	assert.Zero(t, remaining, "并发状态归零后必须删除")
}

func TestMemoryActivityLimitStoreHoldsUntilRelease(t *testing.T) {
	store := newMemoryActivityLimitStore()
	ctx := t.Context()

	acquired, err := store.Acquire(ctx, "k", 1, "lease-long")
	require.NoError(t, err)
	require.True(t, acquired)

	// 长请求反复续租（等价于长时间运行/被调度暂停）：槽位一直归它所有，
	// 内存计数不做时间过期，避免长请求的槽被误判回收后超发。
	for i := 0; i < 3; i++ {
		renewed, renewErr := store.Renew(ctx, "k", "lease-long")
		require.NoError(t, renewErr)
		assert.True(t, renewed)
	}
	acquired, err = store.Acquire(ctx, "k", 1, "lease-other")
	require.NoError(t, err)
	assert.False(t, acquired, "长请求未结束前槽位必须一直占用")

	// 释放后不复活：同一租约的续租失败，额度让给下一个请求。
	require.NoError(t, store.Release(ctx, "k", "lease-long"))
	renewed, err := store.Renew(ctx, "k", "lease-long")
	require.NoError(t, err)
	assert.False(t, renewed, "释放后的租约不能复活")
	acquired, err = store.Acquire(ctx, "k", 1, "lease-other")
	require.NoError(t, err)
	assert.True(t, acquired)
}

func TestMemoryActivityLimitStoreRateWindow(t *testing.T) {
	store := newMemoryActivityLimitStore()
	ctx := t.Context()
	for i := 0; i < 2; i++ {
		allowed, _, err := store.RateTake(ctx, "r", 2)
		require.NoError(t, err)
		assert.True(t, allowed)
	}
	allowed, retryAfter, err := store.RateTake(ctx, "r", 2)
	require.NoError(t, err)
	assert.False(t, allowed, "窗口超限")
	assert.Equal(t, int64(activityRateLimitWindowSeconds), retryAfter,
		"内存窗口无法报告剩余时间，Retry-After 用整窗长度作保守上界")
}

// ---------------------------------------------------------------------------
// 会话：同用户多令牌共享、同组不重复、跨组各一次、统一释放
// ---------------------------------------------------------------------------

func TestActivityConcurrencySessionPerUserGroupSharing(t *testing.T) {
	setupActivityLimitTest(t)

	first := beginActivityLimitRequest(7001, "vip")
	require.NotNil(t, first.session)
	acquired, err := first.session.acquire("vip", 1)
	require.NoError(t, err)
	assert.True(t, acquired)

	// 同用户换个令牌（新的请求/会话）共享同一 user+group 额度。
	second := beginActivityLimitRequest(7001, "vip")
	require.NotNil(t, second.session)
	acquired, err = second.session.acquire("vip", 1)
	require.NoError(t, err)
	assert.False(t, acquired, "同用户多令牌必须共享同一并发额度")

	// 同组 retry：不重复占用，幂等成功。
	acquired, err = first.session.acquire("vip", 1)
	require.NoError(t, err)
	assert.True(t, acquired)

	// 跨组 fallback：新分组各获取一次。
	acquired, err = second.session.acquire("svip", 1)
	require.NoError(t, err)
	assert.True(t, acquired)

	// 请求结束统一释放：两个分组的槽都归还。
	first.finish()
	first.finish() // 幂等
	second.finish()

	third := beginActivityLimitRequest(7001, "vip")
	acquired, err = third.session.acquire("vip", 1)
	require.NoError(t, err)
	assert.True(t, acquired, "请求结束后额度必须归还")
	third.finish()
}

func TestActivityConcurrencySessionReleaseUsesIndependentContext(t *testing.T) {
	setupActivityLimitTest(t)

	req := beginActivityLimitRequest(7002, "vip")
	acquired, err := req.session.acquire("vip", 1)
	require.NoError(t, err)
	require.True(t, acquired)

	// 异常取消（客户端断开/失租）后释放仍必须归还租约。
	req.session.cancel()
	req.finish()

	next := beginActivityLimitRequest(7002, "vip")
	acquired, err = next.session.acquire("vip", 1)
	require.NoError(t, err)
	assert.True(t, acquired, "请求取消后的释放不能依赖请求 context")
	next.finish()
}

func TestActivityConcurrencySlotKeepsItsStore(t *testing.T) {
	setupActivityLimitTest(t)

	storeA := newMemoryActivityLimitStore()
	restore := setActivityLimitStoreForTest(storeA)
	req := beginActivityLimitRequest(7003, "vip")
	acquired, err := req.session.acquire("vip", 1)
	require.NoError(t, err)
	require.True(t, acquired)

	// 运行中切换存储（重建客户端/切实现）：旧槽必须仍按获取时的存储归还。
	setActivityLimitStoreForTest(newMemoryActivityLimitStore())
	req.finish()
	restore()

	acquired, err = storeA.Acquire(t.Context(), activityConcurrencyKey(7003, "vip"), 1, "lease-after")
	require.NoError(t, err)
	assert.True(t, acquired, "释放必须作用于获取该槽时的存储实例")
}

// ---------------------------------------------------------------------------
// Redis 租约：服务端 TIME、续租、过期不复活、TTL、同租约幂等
// ---------------------------------------------------------------------------

func newActivityRedisStore(t *testing.T) (*miniredis.Miniredis, *redisActivityLimitStore) {
	t.Helper()
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return server, newRedisActivityLimitStore(client)
}

func TestRedisActivityLimitStoreLeaseLifecycle(t *testing.T) {
	server, store := newActivityRedisStore(t)
	ctx := t.Context()
	key := activityConcurrencyKey(8001, "vip")

	acquired, err := store.Acquire(ctx, key, 2, "lease-a")
	require.NoError(t, err)
	assert.True(t, acquired)
	acquired, err = store.Acquire(ctx, key, 2, "lease-b")
	require.NoError(t, err)
	assert.True(t, acquired)
	acquired, err = store.Acquire(ctx, key, 2, "lease-c")
	require.NoError(t, err)
	assert.False(t, acquired, "Redis 侧同样按 limit 拒绝")

	// 同一租约的获取重试幂等：不占双份容量。
	acquired, err = store.Acquire(ctx, key, 2, "lease-a")
	require.NoError(t, err)
	assert.True(t, acquired)

	// TTL 至少覆盖两倍租约时长，key 不会在租约有效期内消失。
	assert.GreaterOrEqual(t, server.TTL(key), time.Duration(activityConcurrencyKeyTTLSeconds)*time.Second)

	// 幂等 release。
	require.NoError(t, store.Release(ctx, key, "lease-a"))
	require.NoError(t, store.Release(ctx, key, "lease-a"))
	acquired, err = store.Acquire(ctx, key, 1, "lease-d")
	require.NoError(t, err)
	assert.False(t, acquired, "重复释放不能多还一个槽")

	// 只有自己的租约能续租，不存在的租约续租失败。
	renewed, err := store.Renew(ctx, key, "lease-b")
	require.NoError(t, err)
	assert.True(t, renewed)
	renewed, err = store.Renew(ctx, key, "lease-ghost")
	require.NoError(t, err)
	assert.False(t, renewed, "不存在的租约不能续租")
}

func TestRedisActivityLimitStoreRenewExtendsLease(t *testing.T) {
	server, store := newActivityRedisStore(t)
	ctx := t.Context()
	key := activityConcurrencyKey(8002, "vip")
	start := time.Now()

	_, err := store.Acquire(ctx, key, 1, "lease-long")
	require.NoError(t, err)

	// 长流请求：租约 60s，期间每次续租把过期时间往后推（时间取服务端 TIME）。
	server.SetTime(start.Add(30 * time.Second))
	renewed, err := store.Renew(ctx, key, "lease-long")
	require.NoError(t, err)
	assert.True(t, renewed)

	// 距首次获取已 70s（超过 60s 租约）：仍能续租，说明续租确实延长了租约。
	server.SetTime(start.Add(70 * time.Second))
	renewed, err = store.Renew(ctx, key, "lease-long")
	require.NoError(t, err)
	assert.True(t, renewed, "持续续租的长流请求不丢租")

	// 断租后不再续租：过期的租约不能复活。
	server.SetTime(start.Add(time.Duration(70+activityConcurrencyLeaseSeconds+1) * time.Second))
	renewed, err = store.Renew(ctx, key, "lease-long")
	require.NoError(t, err)
	assert.False(t, renewed, "租约过期后不能复活")

	// 过期租约不占容量。
	acquired, err := store.Acquire(ctx, key, 1, "lease-next")
	require.NoError(t, err)
	assert.True(t, acquired)
}

func TestRedisActivityLimitStoreConcurrentAcquireIsAtomic(t *testing.T) {
	_, store := newActivityRedisStore(t)
	ctx := t.Context()
	key := activityConcurrencyKey(8005, "vip")

	const workers = 16
	const limit = 4
	start := make(chan struct{})
	results := make(chan bool, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start // 同步屏障：全部就绪后同时发起
			acquired, err := store.Acquire(ctx, key, limit, fmt.Sprintf("lease-%d", index))
			assert.NoError(t, err)
			results <- acquired
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	success := 0
	for acquired := range results {
		if acquired {
			success++
		}
	}
	assert.Equal(t, limit, success, "并发获取必须恰好放行 limit 个，不能超发")
}

func TestRedisActivityLimitStorePrunesExpiredBeforeIdempotentCheck(t *testing.T) {
	server, store := newActivityRedisStore(t)
	ctx := t.Context()
	key := activityConcurrencyKey(8004, "vip")
	start := time.Now()

	_, err := store.Acquire(ctx, key, 1, "lease-a")
	require.NoError(t, err)

	// lease-a 已过期但成员尚未剪枝：获取必须先剪枝再做幂等判断，
	// 否则过期成员会被误判为仍持槽，实际容量已空却拒掉新请求。
	server.SetTime(start.Add(time.Duration(activityConcurrencyLeaseSeconds+1) * time.Second))
	acquired, err := store.Acquire(ctx, key, 1, "lease-b")
	require.NoError(t, err)
	assert.True(t, acquired, "过期租约剪枝后容量应重新可用")
	acquired, err = store.Acquire(ctx, key, 1, "lease-c")
	require.NoError(t, err)
	assert.False(t, acquired, "重新占用后容量已满")
}

func TestActivityConcurrencySessionLostLeaseCancelsRequest(t *testing.T) {
	server, store := newActivityRedisStore(t)
	restore := setActivityLimitStoreForTest(store)
	defer restore()
	start := time.Now()

	req := beginActivityLimitRequest(8003, "vip")
	acquired, err := req.session.acquire("vip", 1)
	require.NoError(t, err)
	require.True(t, acquired)
	require.False(t, req.session.Lost())

	// 长流期间正常续租不取消请求。
	assert.True(t, req.session.renewSlot(req.session.states["vip"].slot))
	assert.False(t, req.session.Lost())
	assert.NoError(t, req.session.ctx.Err())

	// 租约丢失（过期）：取消请求，防止无租请求继续占用上游。
	server.SetTime(start.Add(time.Duration(activityConcurrencyLeaseSeconds+1) * time.Second))
	assert.False(t, req.session.renewSlot(req.session.states["vip"].slot))
	assert.True(t, req.session.Lost())
	assert.Error(t, req.session.ctx.Err(), "失租必须取消请求")
	req.finish()
}

// ---------------------------------------------------------------------------
// guard：RPM/并发/混合 + fail closed + 会话失效
// ---------------------------------------------------------------------------

func TestActivityLimitGuardEnforcesConcurrency(t *testing.T) {
	db := setupActivityLimitTest(t)
	createActivityLimitGrant(t, db, 8101, "conc-group", model.ActivityLimitModeConcurrency, 0, 1)

	first := beginActivityLimitRequest(8101, "conc-group")
	require.Nil(t, first.guard("conc-group"), "并发未满应放行")

	second := beginActivityLimitRequest(8101, "conc-group")
	limitErr := second.guard("conc-group")
	require.NotNil(t, limitErr, "并发满必须拒绝")
	assert.Equal(t, http.StatusTooManyRequests, limitErr.StatusCode)
	assert.Equal(t, ActivityLimitCodeConcurrencyExceeded, limitErr.Code)
	assert.Zero(t, limitErr.RetryAfterSeconds, "并发超限不得伪造 Retry-After")

	first.finish()
	second.finish()
	third := beginActivityLimitRequest(8101, "conc-group")
	require.Nil(t, third.guard("conc-group"), "释放后恢复可用")
	third.finish()
}

func TestActivityLimitGuardEnforcesRpmWithRetryAfter(t *testing.T) {
	db := setupActivityLimitTest(t)
	createActivityLimitGrant(t, db, 8102, "rpm-group", model.ActivityLimitModeRPM, 1, 0)

	first := beginActivityLimitRequest(8102, "rpm-group")
	require.Nil(t, first.guard("rpm-group"))
	first.finish()

	second := beginActivityLimitRequest(8102, "rpm-group")
	limitErr := second.guard("rpm-group")
	require.NotNil(t, limitErr, "RPM 超限必须拒绝")
	assert.Equal(t, http.StatusTooManyRequests, limitErr.StatusCode)
	assert.Equal(t, ActivityLimitCodeRpmExceeded, limitErr.Code)
	assert.GreaterOrEqual(t, limitErr.RetryAfterSeconds, int64(1))
	assert.LessOrEqual(t, limitErr.RetryAfterSeconds, int64(activityRateLimitWindowSeconds))
	second.finish()
}

func TestActivityLimitGuardSameGroupRetryDoesNotDoubleCount(t *testing.T) {
	db := setupActivityLimitTest(t)
	createActivityLimitGrant(t, db, 8103, "retry-group", model.ActivityLimitModeRPM, 1, 0)

	req := beginActivityLimitRequest(8103, "retry-group")
	require.Nil(t, req.guard("retry-group"))
	require.Nil(t, req.guard("retry-group"), "同组 retry 不得重复计数")
	require.Nil(t, req.guard(""), "空分组回退 using group，同组幂等")
	req.finish()
}

func TestActivityLimitGuardMixedLimitsBothApply(t *testing.T) {
	db := setupActivityLimitTest(t)
	createActivityLimitGrant(t, db, 8104, "mixed-group", model.ActivityLimitModeRPM, 2, 0)
	createActivityLimitGrant(t, db, 8104, "mixed-group", model.ActivityLimitModeConcurrency, 0, 1)

	first := beginActivityLimitRequest(8104, "mixed-group")
	require.Nil(t, first.guard("mixed-group"))

	// 并发约束生效：新请求被并发上限拒绝。
	concurrencyErr := beginActivityLimitRequest(8104, "mixed-group").guard("mixed-group")
	require.NotNil(t, concurrencyErr)
	assert.Equal(t, ActivityLimitCodeConcurrencyExceeded, concurrencyErr.Code)

	// RPM 约束也生效：两条请求已用完 RPM=2 的窗口。
	first.finish()
	rpmErr := beginActivityLimitRequest(8104, "mixed-group").guard("mixed-group")
	require.NotNil(t, rpmErr)
	assert.Equal(t, ActivityLimitCodeRpmExceeded, rpmErr.Code)
}

func TestActivityLimitGuardSkipsGroupsWithoutGrant(t *testing.T) {
	db := setupActivityLimitTest(t)
	createActivityLimitGrant(t, db, 8105, "other-group", model.ActivityLimitModeConcurrency, 0, 1)

	for i := 0; i < 3; i++ {
		req := beginActivityLimitRequest(8105, "plain-group")
		require.Nil(t, req.guard("plain-group"), "没有活动授权的分组不受活动限流")
		req.finish()
	}
}

func TestActivityLimitGuardSkipsInternalAssistantRequests(t *testing.T) {
	db := setupActivityLimitTest(t)
	createActivityLimitGrant(t, db, 8108, "internal-group", model.ActivityLimitModeConcurrency, 0, 1)

	for i := 0; i < 3; i++ {
		c := newActivityLimitContext(8108, "internal-group")
		c.Request = c.Request.WithContext(assistantops.WithActor(c.Request.Context(), assistantops.Actor{UserID: 8108}))
		require.Nil(t, ActivityLimitGuard(c, "internal-group"), "内部助手请求沿用限流旁路")
	}
}

func TestActivityLimitGuardFailsClosedOnGrantQueryError(t *testing.T) {
	db := setupActivityLimitTest(t)
	createActivityLimitGrant(t, db, 8106, "broken-group", model.ActivityLimitModeConcurrency, 0, 1)
	first := beginActivityLimitRequest(8106, "broken-group")
	require.Nil(t, first.guard("broken-group"))
	first.finish()

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	second := beginActivityLimitRequest(8106, "broken-group")
	limitErr := second.guard("broken-group")
	require.NotNil(t, limitErr, "授权查询失败不得当作没有授权放行")
	assert.Equal(t, http.StatusServiceUnavailable, limitErr.StatusCode)
	assert.Equal(t, ActivityLimitCodeConcurrencyUnavailable, limitErr.Code)
}

func TestActivityLimitGuardFailsClosedOnRedisError(t *testing.T) {
	setupActivityLimitTest(t)
	db := model.DB
	createActivityLimitGrant(t, db, 8107, "redis-group", model.ActivityLimitModeConcurrency, 0, 1)

	server, store := newActivityRedisStore(t)
	restore := setActivityLimitStoreForTest(store)
	defer restore()

	first := beginActivityLimitRequest(8107, "redis-group")
	require.Nil(t, first.guard("redis-group"))
	server.Close()

	second := beginActivityLimitRequest(8107, "redis-group")
	limitErr := second.guard("redis-group")
	require.NotNil(t, limitErr, "Redis 故障必须 fail closed")
	assert.Equal(t, http.StatusServiceUnavailable, limitErr.StatusCode)
	assert.Equal(t, ActivityLimitCodeConcurrencyUnavailable, limitErr.Code)

	// 绝不回退内存实现：再次尝试依旧拒绝。
	third := beginActivityLimitRequest(8107, "redis-group")
	limitErr = third.guard("redis-group")
	require.NotNil(t, limitErr)
	assert.Equal(t, http.StatusServiceUnavailable, limitErr.StatusCode)
}

func TestActivityLimitGuardFailsClosedWithoutRedisClient(t *testing.T) {
	setupActivityLimitTest(t)
	createActivityLimitGrant(t, model.DB, 8109, "no-client-group", model.ActivityLimitModeConcurrency, 0, 1)

	// 清除测试注入，走自动选择；Redis 已启用但客户端未初始化 → 不可用存储。
	restore := setActivityLimitStoreForTest(nil)
	defer restore()
	previousRedis := common.RedisEnabled
	previousRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = previousRedis
		common.RDB = previousRDB
	})

	// 限流状态同样不回退内存：Redis 启用却不可用时只有「不可用」存储。
	assert.IsType(t, &brokenActivityLimitStore{}, activityLimits())

	req := beginActivityLimitRequest(8109, "no-client-group")
	limitErr := req.guard("no-client-group")
	require.NotNil(t, limitErr, "Redis 启用却不可用时必须 fail closed，不能回退内存")
	assert.Equal(t, http.StatusServiceUnavailable, limitErr.StatusCode)
	assert.Equal(t, ActivityLimitCodeConcurrencyUnavailable, limitErr.Code)
}

func TestActivityLimitGuardFailsClosedWithoutSession(t *testing.T) {
	db := setupActivityLimitTest(t)
	createActivityLimitGrant(t, db, 8111, "no-session-group", model.ActivityLimitModeConcurrency, 0, 1)

	// 路由漏挂活动限流中间件（没有会话生命周期）时必须 fail closed，
	// 而不是静默创建无人释放的会话或绕过限流。
	c := newActivityLimitContext(8111, "no-session-group")
	limitErr := ActivityLimitGuard(c, "no-session-group")
	require.NotNil(t, limitErr)
	assert.Equal(t, http.StatusServiceUnavailable, limitErr.StatusCode)
}

func TestActivityLimitGuardRejectsAfterLostLease(t *testing.T) {
	db := setupActivityLimitTest(t)
	createActivityLimitGrant(t, db, 8110, "lost-group", model.ActivityLimitModeConcurrency, 0, 1)

	req := beginActivityLimitRequest(8110, "lost-group")
	require.Nil(t, req.guard("lost-group"), "首次检查占槽成功")

	// 失租后 fast-path 不得放行 retry。
	req.session.markLost()
	limitErr := req.guard("lost-group")
	require.NotNil(t, limitErr, "失租后的请求必须拒绝")
	assert.Equal(t, http.StatusServiceUnavailable, limitErr.StatusCode)

	// 请求被取消同样拒绝。
	req.finish()
	cancelled := beginActivityLimitRequest(8110, "lost-group")
	cancelled.session.cancel()
	limitErr = cancelled.guard("lost-group")
	require.NotNil(t, limitErr, "请求取消后不得继续放行")
	assert.Equal(t, http.StatusServiceUnavailable, limitErr.StatusCode)

	// 会话已释放（states 清空）后继续 guard 不 panic 且拒绝。
	cancelled.finish()
	limitErr = cancelled.guard("lost-group")
	require.NotNil(t, limitErr)
	assert.Equal(t, http.StatusServiceUnavailable, limitErr.StatusCode)
}

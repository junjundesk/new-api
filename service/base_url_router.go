package service

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// 多渠道地址自动选路（方案 A）：进程内按健康度/延迟为同一渠道的多个 API 地址选路。
// 规范文档：.scratch/base-urls-spec.md 第二节。
const (
	baseURLEWMAAlpha        = 0.3              // EWMA 平滑系数
	baseURLStickyTTL        = 30 * time.Second // sticky 保持窗口
	baseURLCooldownBase     = 30 * time.Second // 连续失败 cooldown 基数：30s * 2^(n-1)
	baseURLCooldownMax      = 5 * time.Minute  // cooldown 上限
	baseURLProbeCooldownTTL = 10 * time.Second // 探测失败的短 cooldown
)

// 无真实样本地址的排序键：正常选路时最后才选，保证已知更快的地址优先。
const baseURLUnknownLatency = time.Duration(1 << 62)

type baseURLStats struct {
	ttfbEWMA         time.Duration // EWMA 首字延迟
	hasTTFB          bool
	probeLatency     time.Duration // 探测延迟（无真实样本时参与排序）
	hasProbe         bool
	consecutiveFails int
	cooldownUntil    time.Time
	samples          int // 真实请求样本数（成功/失败上报）
}

// sortLatency 返回参与排序的延迟：真实样本优先，其次探测延迟，都没有则排最后。
func (s *baseURLStats) sortLatency() time.Duration {
	if s.hasTTFB {
		return s.ttfbEWMA
	}
	if s.hasProbe {
		return s.probeLatency
	}
	return baseURLUnknownLatency
}

func (s *baseURLStats) cooling(now time.Time) bool {
	return now.Before(s.cooldownUntil)
}

type channelRoute struct {
	urls        []string
	stats       map[string]*baseURLStats
	stickyURL   string
	stickyUntil time.Time
	rr          int // 冷启动轮询计数
}

type baseURLRouter struct {
	mu     sync.Mutex
	routes map[int]*channelRoute
	now    func() time.Time
}

func newBaseURLRouter() *baseURLRouter {
	return &baseURLRouter{
		routes: make(map[int]*channelRoute),
		now:    time.Now,
	}
}

var defaultBaseURLRouter = newBaseURLRouter()

func SelectChannelBaseURL(channel *model.Channel) string {
	return defaultBaseURLRouter.selectBaseURL(channel)
}

func ReportBaseURLSuccess(channelID int, baseURL string, ttfb time.Duration) {
	defaultBaseURLRouter.reportSuccess(channelID, baseURL, ttfb)
}

func ReportBaseURLFailure(channelID int, baseURL string) {
	defaultBaseURLRouter.reportFailure(channelID, baseURL)
}

func ReportProbeLatency(channelID int, baseURL string, d time.Duration, ok bool) {
	defaultBaseURLRouter.reportProbe(channelID, baseURL, d, ok)
}

func NextBaseURL(channelID int, current string, tried []string) (string, bool) {
	return defaultBaseURLRouter.nextBaseURL(channelID, current, tried)
}

func InvalidateChannelBaseURLs(channelID int) {
	defaultBaseURLRouter.invalidateChannelBaseURLs(channelID)
}

func (r *baseURLRouter) selectBaseURL(channel *model.Channel) string {
	urls := channel.GetBaseURLs()
	if len(urls) == 0 {
		// 无地址（如测试或尚未配置的渠道）返回空串，交给调用方回退到默认地址。
		return ""
	}
	if len(urls) == 1 {
		// 单地址无需选路；路由表由渠道更新/删除时显式清理，
		// 避免每个请求都为单地址渠道争用全局锁。
		return urls[0]
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	route, ok := r.routes[channel.Id]
	if !ok || !sameBaseURLList(route.urls, urls) {
		route = r.newRouteLocked(channel.Id, urls)
	}
	now := r.now()

	// 冷启动：把还没有真实样本的地址先轮询一遍，保证每个地址都被均匀评估。
	unsampled := make([]string, 0, len(urls))
	for _, u := range urls {
		if route.stats[u].samples == 0 && !route.stats[u].cooling(now) {
			unsampled = append(unsampled, u)
		}
	}
	if len(unsampled) > 0 {
		pick := unsampled[route.rr%len(unsampled)]
		route.rr++
		return pick
	}

	// sticky 窗口内且未进入 cooldown 的地址保持不变，避免在相近地址间抖动。
	if route.stickyURL != "" && now.Before(route.stickyUntil) {
		if st := route.stats[route.stickyURL]; st != nil && !st.cooling(now) {
			return route.stickyURL
		}
	}

	candidates := make([]string, 0, len(urls))
	for _, u := range urls {
		if !route.stats[u].cooling(now) {
			candidates = append(candidates, u)
		}
	}
	if len(candidates) == 0 {
		// 全部在 cooldown 中时退化为轮询，保证请求仍可发出。
		candidates = urls
	}
	pick := candidates[0]
	best := route.stats[pick].sortLatency()
	for _, u := range candidates[1:] {
		if lat := route.stats[u].sortLatency(); lat < best {
			pick, best = u, lat
		}
	}
	route.stickyURL = pick
	route.stickyUntil = now.Add(baseURLStickyTTL)
	return pick
}

func (r *baseURLRouter) reportSuccess(channelID int, baseURL string, ttfb time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.existingStatLocked(channelID, baseURL)
	if st == nil {
		return
	}
	st.samples++
	st.consecutiveFails = 0
	st.cooldownUntil = time.Time{}
	if !st.hasTTFB {
		st.ttfbEWMA = ttfb
		st.hasTTFB = true
		return
	}
	st.ttfbEWMA = time.Duration(baseURLEWMAAlpha*float64(ttfb) + (1-baseURLEWMAAlpha)*float64(st.ttfbEWMA))
}

func (r *baseURLRouter) reportFailure(channelID int, baseURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.existingStatLocked(channelID, baseURL)
	if st == nil {
		return
	}
	route := r.routes[channelID]
	st.samples++
	st.consecutiveFails++
	cooldown := baseURLCooldownBase
	for i := 1; i < st.consecutiveFails && cooldown < baseURLCooldownMax; i++ {
		cooldown *= 2
	}
	if cooldown > baseURLCooldownMax {
		cooldown = baseURLCooldownMax
	}
	st.cooldownUntil = r.now().Add(cooldown)
	if route.stickyURL == baseURL {
		route.stickyURL = ""
		route.stickyUntil = time.Time{}
	}
}

func (r *baseURLRouter) reportProbe(channelID int, baseURL string, d time.Duration, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.existingStatLocked(channelID, baseURL)
	if st == nil {
		return
	}
	if ok {
		st.probeLatency = d
		st.hasProbe = true
		return
	}
	if until := r.now().Add(baseURLProbeCooldownTTL); until.After(st.cooldownUntil) {
		st.cooldownUntil = until
	}
}

func (r *baseURLRouter) nextBaseURL(channelID int, current string, tried []string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	route, ok := r.routes[channelID]
	if !ok || len(route.urls) <= 1 {
		return "", false
	}
	excluded := make(map[string]struct{}, len(tried)+1)
	for _, t := range tried {
		excluded[t] = struct{}{}
	}
	excluded[current] = struct{}{}
	now := r.now()
	best := ""
	found := false
	var bestLat time.Duration
	for _, u := range route.urls {
		if _, skip := excluded[u]; skip {
			continue
		}
		st := route.stats[u]
		if st == nil || st.cooling(now) {
			continue
		}
		lat := st.sortLatency()
		if !found || lat < bestLat {
			best, bestLat, found = u, lat, true
		}
	}
	return best, found
}

// ensureRoute 确保渠道路由表项与地址列表一致；探测任务用它注册尚未被请求
// 触达的渠道，避免只有发生过请求的渠道才会被探测。
func (r *baseURLRouter) ensureRoute(channelID int, baseURLs []string) {
	if len(baseURLs) <= 1 {
		r.invalidateChannelBaseURLs(channelID)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	route, ok := r.routes[channelID]
	if !ok || !sameBaseURLList(route.urls, baseURLs) {
		r.newRouteLocked(channelID, baseURLs)
	}
}
func (r *baseURLRouter) invalidateChannelBaseURLs(channelID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, channelID)
}

// newRouteLocked 按地址列表新建路由表项并替换旧项（调用方持锁）。
func (r *baseURLRouter) newRouteLocked(channelID int, baseURLs []string) *channelRoute {
	route := &channelRoute{
		urls:  append([]string(nil), baseURLs...),
		stats: make(map[string]*baseURLStats, len(baseURLs)),
	}
	for _, u := range route.urls {
		route.stats[u] = &baseURLStats{}
	}
	r.routes[channelID] = route
	return route
}

// existingStatLocked 返回已注册路由中该地址的统计项；路由不存在或地址不在
// 列表内时返回 nil。上报路径不能据此新建路由，否则会用单地址覆盖多地址列表。
func (r *baseURLRouter) existingStatLocked(channelID int, baseURL string) *baseURLStats {
	route, ok := r.routes[channelID]
	if !ok {
		return nil
	}
	st, ok := route.stats[baseURL]
	if !ok {
		return nil
	}
	return st
}

func sameBaseURLList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

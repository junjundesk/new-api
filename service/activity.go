package service

import (
	"errors"
	"maps"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// ActivityTokenBudget 是某个 Token 套餐权益在当前请求上可用的剩余额度。
// Remaining 为 0 时表示套餐已用尽，计费应回退到非套餐倍率或分组原价。
type ActivityTokenBudget struct {
	GrantId   int
	Remaining int64
}

// ActivityLimitModeMixed 表示同一分组同时存在 rpm 型与 concurrency 型授权，
// 两种限流约束都会生效；它只用于聚合视图展示，不是授权行的取值。
const ActivityLimitModeMixed = "mixed"

// ActivityView 是用户在某一个分组上的有效活动授权（同一分组的多张授权合并后的结果）：
// 倍率取最小值、RPM 取最大值、并发取最宽松（显式 0 = 不限 优先），到期时间取最晚
// （有永久授权则永久）。限流按模式分流聚合，混合授权的两种约束同时生效。
// Token 套餐按权益 grant 单独计量；合并后的 TokenTotal/TokenUsed 只用于展示。
type ActivityView struct {
	GrantId       int     `json:"grant_id"`
	GroupName     string  `json:"group_name"`
	Code          string  `json:"code"`
	CodeName      string  `json:"code_name"`
	RatioMode     string  `json:"ratio_mode"`
	RatioOverride float64 `json:"ratio_override"`
	// ModelRatioOverrides 是「指定模型 → 倍率」覆盖：命中的模型优先于 RatioOverride，
	// 未命中的模型继续沿用 RatioMode/RatioOverride 与分组默认倍率。
	ModelRatioOverrides map[string]float64 `json:"model_ratio_overrides"`
	// LimitMode 是该分组限流模式的聚合结果：rpm / concurrency / mixed（两种授权都有）。
	LimitMode string `json:"limit_mode"`
	// RpmLimit 是 rpm 型授权的 RPM 上限（取最大值，0 表示不限）。
	RpmLimit int `json:"rpm_limit"`
	// ConcurrencyLimit 是 concurrency 型授权的并发上限（取最宽松值，0 表示不限）。
	// rpm 型授权不参与该聚合：其 ConcurrencyLimit 恒为 0（未启用），
	// 若被当作「不限」并参与取最宽松，会把并发约束错误地取消掉。
	ConcurrencyLimit int   `json:"concurrency_limit"`
	ExpireTime       int64 `json:"expire_time"`

	// Token 套餐展示字段：同一分组所有 Token 套餐权益的总量/已用量。
	TokenTotal   int64 `json:"token_total"`
	TokenUsed    int64 `json:"token_used"`
	TokenLimited bool  `json:"token_limited"`

	// 以下字段只用于计费解析，不参与 JSON 序列化。
	groupUnlimitedFree  bool
	groupTokenBudget    *ActivityTokenBudget
	groupFallbackRatio  *float64
	modelUnlimitedFree  map[string]bool
	modelTokenBudgets   map[string]*ActivityTokenBudget
	modelFallbackRatios map[string]float64
}

type activityPackageCandidate struct {
	GrantId    int
	Remaining  int64
	ExpireTime int64
}

func activityPackageCandidateFromGrant(grant model.ActivityGrant) activityPackageCandidate {
	remaining := grant.TokenTotal - grant.TokenUsed
	if remaining < 0 {
		remaining = 0
	}
	return activityPackageCandidate{GrantId: grant.Id, Remaining: remaining, ExpireTime: grant.EffectiveExpire()}
}

// aggregateActivityPackageCandidates 汇总同组/同模型的套餐剩余量，并选出本次扣减的授权：
// 有剩余且到期时间最早者优先（永久排最后）。remaining 为总剩余，0 表示全部用尽。
func aggregateActivityPackageCandidates(candidates []activityPackageCandidate) (remaining int64, grantId int) {
	chosen := -1
	var chosenExpire int64
	for _, candidate := range candidates {
		remaining += candidate.Remaining
		if candidate.Remaining <= 0 {
			continue
		}
		if chosen == -1 || packageCandidateEarlier(candidate.ExpireTime, chosenExpire) {
			chosen = candidate.GrantId
			chosenExpire = candidate.ExpireTime
		}
	}
	if chosen == -1 {
		return remaining, 0
	}
	return remaining, chosen
}

func packageCandidateEarlier(left, right int64) bool {
	if left == 0 {
		return false
	}
	if right == 0 {
		return true
	}
	return left < right
}

// GetActiveActivityViewsE 读取用户当前有效的活动授权并按分组合并。
// 查询失败时返回错误而不是空列表：把失败当作「没有授权」会让限流路径
// 在数据库故障时直接绕过限制，因此限流侧必须使用本函数并 fail closed。
func GetActiveActivityViewsE(userId int) ([]ActivityView, error) {
	if userId <= 0 {
		return nil, nil
	}
	if common.RedisEnabled && common.RDB == nil {
		// Redis 已启用但客户端未初始化：授权缓存读取会因空客户端崩溃，
		// 而故障期间绝不能当作「没有授权」。返回错误让限流路径 fail closed。
		return nil, errors.New("activity grant store unavailable: Redis is enabled but its client is not initialized")
	}
	grants, err := model.GetActiveActivityGrants(userId)
	if err != nil {
		return nil, err
	}
	return mergeActivityViews(grants), nil
}

// GetActiveActivityViews 读取用户当前有效的活动授权并按分组合并；
// 查询失败只记日志并返回空列表，仅用于展示与计费等「无授权即无优惠」安全的路径。
func GetActiveActivityViews(userId int) []ActivityView {
	views, err := GetActiveActivityViewsE(userId)
	if err != nil {
		common.SysError("failed to load active activity grants: " + err.Error())
		return nil
	}
	return views
}

type activityViewBuilder struct {
	view                 ActivityView
	seen                 bool
	rpmSeen              bool
	rpmMax               int
	concurrencySeen      bool
	concurrencyMax       int
	concurrencyUnlimited bool
	groupUnlimitedFree   bool
	groupPackages        []activityPackageCandidate
	groupFallbackRatio   *float64

	modelUnlimitedFree  map[string]bool
	modelPackages       map[string][]activityPackageCandidate
	modelFallbackRatios map[string]float64

	tokenGrantIds map[int]struct{}
}

func newActivityViewBuilder(groupName string) *activityViewBuilder {
	return &activityViewBuilder{
		view:                ActivityView{GroupName: groupName},
		modelUnlimitedFree:  make(map[string]bool),
		modelPackages:       make(map[string][]activityPackageCandidate),
		modelFallbackRatios: make(map[string]float64),
		tokenGrantIds:       make(map[int]struct{}),
	}
}

func (builder *activityViewBuilder) merge(grant model.ActivityGrant) {
	// 生效到期时间取 min(expire_time, activity_end_time)：活动到期后授权立即失效，
	// 展示的到期时间不能停在 activity 结束之后。
	expireTime := grant.EffectiveExpire()
	if !builder.seen {
		builder.view.GrantId = grant.Id
		builder.view.Code = grant.Code
		builder.view.CodeName = grant.CodeName
		builder.view.RatioMode = grant.RatioMode
		builder.view.RatioOverride = grant.RatioOverride
		builder.view.ExpireTime = expireTime
		builder.seen = true
	} else {
		if grant.RatioMode == model.ActivityRatioModeOverride &&
			(builder.view.RatioMode != model.ActivityRatioModeOverride || grant.RatioOverride < builder.view.RatioOverride) {
			builder.view.RatioMode = model.ActivityRatioModeOverride
			builder.view.RatioOverride = grant.RatioOverride
			builder.view.GrantId = grant.Id
			builder.view.Code = grant.Code
			builder.view.CodeName = grant.CodeName
		}

		if builder.view.ExpireTime == 0 || expireTime == 0 {
			builder.view.ExpireTime = 0
		} else if expireTime > builder.view.ExpireTime {
			builder.view.ExpireTime = expireTime
		}
	}

	// 限流聚合按模式分流：rpm 型授权只参与 RPM 聚合（数值取最大），
	// concurrency 型授权只参与并发聚合（取最宽松，显式的 0 = 不限 优先）。
	// 存量数据 LimitMode 为空，NormalizeActivityLimitMode 会按 rpm 处理，
	// 因此旧 RPM 数值的 max 聚合行为保持不变。
	switch model.NormalizeActivityLimitMode(grant.LimitMode) {
	case model.ActivityLimitModeConcurrency:
		builder.concurrencySeen = true
		if grant.ConcurrencyLimit <= 0 {
			builder.concurrencyUnlimited = true
		} else if grant.ConcurrencyLimit > builder.concurrencyMax {
			builder.concurrencyMax = grant.ConcurrencyLimit
		}
	default:
		// 非法 LimitMode 也按 rpm 处理：限流配置损坏不能导致用户完全不限流。
		builder.rpmSeen = true
		if grant.RpmLimit > builder.rpmMax {
			builder.rpmMax = grant.RpmLimit
		}
	}

	if grant.TokenTotal > 0 {
		if _, exists := builder.tokenGrantIds[grant.Id]; !exists {
			builder.tokenGrantIds[grant.Id] = struct{}{}
			builder.view.TokenTotal += grant.TokenTotal
			builder.view.TokenUsed += grant.TokenUsed
			builder.view.TokenLimited = true
		}
	}

	for modelName, ratio := range grant.ModelRatioOverrides {
		if ratio == 0 {
			if grant.TokenTotal > 0 {
				builder.modelPackages[modelName] = append(builder.modelPackages[modelName], activityPackageCandidateFromGrant(grant))
			} else {
				builder.modelUnlimitedFree[modelName] = true
			}
			continue
		}
		if existing, ok := builder.modelFallbackRatios[modelName]; !ok || ratio < existing {
			builder.modelFallbackRatios[modelName] = ratio
		}
	}
	if grant.RatioScope == model.ActivityRatioScopeModels {
		return
	}

	if grant.RatioMode != model.ActivityRatioModeOverride {
		return
	}
	if grant.RatioOverride != 0 {
		if builder.groupFallbackRatio == nil || grant.RatioOverride < *builder.groupFallbackRatio {
			ratio := grant.RatioOverride
			builder.groupFallbackRatio = &ratio
		}
		return
	}
	if grant.TokenTotal > 0 {
		builder.groupPackages = append(builder.groupPackages, activityPackageCandidateFromGrant(grant))
		return
	}
	builder.groupUnlimitedFree = true
}

func (builder *activityViewBuilder) finalize() ActivityView {
	view := builder.view
	view.RpmLimit = builder.rpmMax
	if builder.concurrencySeen && !builder.concurrencyUnlimited {
		view.ConcurrencyLimit = builder.concurrencyMax
	}
	switch {
	case builder.rpmSeen && builder.concurrencySeen:
		view.LimitMode = ActivityLimitModeMixed
	case builder.concurrencySeen:
		view.LimitMode = model.ActivityLimitModeConcurrency
	default:
		view.LimitMode = model.ActivityLimitModeRPM
	}
	view.groupUnlimitedFree = builder.groupUnlimitedFree
	view.groupFallbackRatio = builder.groupFallbackRatio
	view.modelUnlimitedFree = builder.modelUnlimitedFree
	view.modelFallbackRatios = builder.modelFallbackRatios
	view.modelTokenBudgets = make(map[string]*ActivityTokenBudget)

	if builder.groupUnlimitedFree {
		view.RatioMode = model.ActivityRatioModeOverride
		view.RatioOverride = 0
		view.ModelRatioOverrides = nil
		return view
	}

	if len(builder.groupPackages) > 0 {
		remaining, grantId := aggregateActivityPackageCandidates(builder.groupPackages)
		if remaining > 0 {
			view.RatioMode = model.ActivityRatioModeOverride
			view.RatioOverride = 0
			view.groupTokenBudget = &ActivityTokenBudget{GrantId: grantId, Remaining: remaining}
		} else if builder.groupFallbackRatio != nil {
			view.RatioMode = model.ActivityRatioModeOverride
			view.RatioOverride = *builder.groupFallbackRatio
		} else {
			// 套餐已用尽且没有其他分组级优惠：回退分组原价，避免展示字段停在 0
			// 被计费解析误判成不限量免费。
			view.RatioMode = model.ActivityRatioModeInherit
			view.RatioOverride = 0
		}
	} else if builder.groupFallbackRatio != nil {
		view.RatioMode = model.ActivityRatioModeOverride
		view.RatioOverride = *builder.groupFallbackRatio
	}

	modelRatios := make(map[string]float64)
	for modelName := range builder.modelUnlimitedFree {
		modelRatios[modelName] = 0
	}
	for modelName, packages := range builder.modelPackages {
		remaining, grantId := aggregateActivityPackageCandidates(packages)
		if remaining > 0 {
			modelRatios[modelName] = 0
			view.modelTokenBudgets[modelName] = &ActivityTokenBudget{GrantId: grantId, Remaining: remaining}
			continue
		}
		if fallback, ok := builder.modelFallbackRatios[modelName]; ok {
			modelRatios[modelName] = fallback
		}
	}
	for modelName, ratio := range builder.modelFallbackRatios {
		if _, exists := modelRatios[modelName]; exists {
			continue
		}
		if _, packageOnly := builder.modelPackages[modelName]; packageOnly {
			continue
		}
		modelRatios[modelName] = ratio
	}
	if len(modelRatios) > 0 {
		view.ModelRatioOverrides = modelRatios
	} else {
		view.ModelRatioOverrides = nil
	}
	return view
}

func mergeActivityViews(grants []model.ActivityGrant) []ActivityView {
	views := make([]ActivityView, 0, len(grants))
	order := make([]string, 0, len(grants))
	builders := make(map[string]*activityViewBuilder, len(grants))
	for _, grant := range grants {
		builder := builders[grant.GroupName]
		if builder == nil {
			builder = newActivityViewBuilder(grant.GroupName)
			builders[grant.GroupName] = builder
			order = append(order, grant.GroupName)
		}
		builder.merge(grant)
	}
	for _, groupName := range order {
		views = append(views, builders[groupName].finalize())
	}
	return views
}

// requestActivityViews 按请求缓存读取用户的活动授权聚合；只有查询成功才写缓存，
// 失败不缓存，避免一次失败把「未知」固化成「没有授权」。
func requestActivityViews(c *gin.Context, userId int) ([]ActivityView, error) {
	views, ok := common.GetContextKeyType[[]ActivityView](c, constant.ContextKeyActivityGrants)
	if ok {
		return views, nil
	}
	views, err := GetActiveActivityViewsE(userId)
	if err != nil {
		return nil, err
	}
	common.SetContextKey(c, constant.ContextKeyActivityGrants, views)
	return views, nil
}

// GetRequestActivityView 返回请求用户在某分组上的有效活动授权，结果按请求缓存在
// gin context 中，一次请求只查一次（鉴权、限流、计费共用）。查询失败只记日志，
// 仅用于「无授权即无优惠/无额外能力」不会造成绕过的路径。
func GetRequestActivityView(c *gin.Context, userId int, group string) *ActivityView {
	view, err := GetRequestActivityViewChecked(c, userId, group)
	if err != nil {
		common.SysError("failed to load activity grants: " + err.Error())
		return nil
	}
	return view
}

// GetRequestActivityViewChecked 与 GetRequestActivityView 相同，但把查询失败返回给调用方。
// 限流路径必须使用本函数：数据库故障时拒绝请求，而不是当作没有活动授权绕过限制。
func GetRequestActivityViewChecked(c *gin.Context, userId int, group string) (*ActivityView, error) {
	if c == nil || userId <= 0 || group == "" {
		return nil, nil
	}
	views, err := requestActivityViews(c, userId)
	if err != nil {
		return nil, err
	}
	for i := range views {
		if views[i].GroupName == group {
			return &views[i], nil
		}
	}
	return nil, nil
}

// BillingRatioOverride 把该授权的倍率覆盖转换成计费解析所需形态。
// 非 nil 的 view 始终返回非 nil 结构体：即使没有任何优惠（继承型授权），
// 非 nil 也代表「活动授权有效」，计费解析器据此让活动授权保持最高优先级。
// allowTokenPackages 为 false 时（按次/任务计费）忽略 Token 套餐免费，避免
// 没有 token 扣减的请求被套餐变成免费；不限量免费权益不受该开关影响。
func (view *ActivityView) BillingRatioOverride(allowTokenPackages bool) *ratio_setting.ActivityRatioOverride {
	if view == nil {
		return nil
	}
	override := &ratio_setting.ActivityRatioOverride{}

	switch {
	case view.groupUnlimitedFree:
		ratio := 0.0
		override.GroupRatio = &ratio
	case view.groupTokenBudget != nil:
		if allowTokenPackages && view.groupTokenBudget.Remaining > 0 {
			ratio := 0.0
			override.GroupRatio = &ratio
			override.GroupTokenGrantId = view.groupTokenBudget.GrantId
		} else if view.groupFallbackRatio != nil {
			ratio := *view.groupFallbackRatio
			override.GroupRatio = &ratio
		}
	case view.RatioMode == model.ActivityRatioModeOverride:
		ratio := view.RatioOverride
		override.GroupRatio = &ratio
	}

	if !view.groupUnlimitedFree {
		if len(view.modelUnlimitedFree) > 0 {
			override.ModelRatios = make(map[string]float64, len(view.modelUnlimitedFree))
			for modelName := range view.modelUnlimitedFree {
				override.ModelRatios[modelName] = 0
			}
		}
		for modelName, budget := range view.modelTokenBudgets {
			if budget == nil {
				continue
			}
			if allowTokenPackages && budget.Remaining > 0 {
				if override.ModelRatios == nil {
					override.ModelRatios = make(map[string]float64)
				}
				override.ModelRatios[modelName] = 0
				if override.ModelTokenGrantIds == nil {
					override.ModelTokenGrantIds = make(map[string]int)
				}
				override.ModelTokenGrantIds[modelName] = budget.GrantId
				continue
			}
			if ratio, ok := view.modelFallbackRatios[modelName]; ok {
				if override.ModelRatios == nil {
					override.ModelRatios = make(map[string]float64)
				}
				override.ModelRatios[modelName] = ratio
			}
		}
		for modelName, ratio := range view.modelFallbackRatios {
			if _, exists := override.ModelRatios[modelName]; exists {
				continue
			}
			if _, packageOnly := view.modelTokenBudgets[modelName]; packageOnly {
				continue
			}
			if override.ModelRatios == nil {
				override.ModelRatios = make(map[string]float64)
			}
			override.ModelRatios[modelName] = ratio
		}
	}

	// 单测/手动构造的 ActivityView 没有内部状态，回退到展示字段。
	if override.ModelRatios == nil && view.modelUnlimitedFree == nil &&
		view.modelTokenBudgets == nil && len(view.ModelRatioOverrides) > 0 {
		override.ModelRatios = maps.Clone(view.ModelRatioOverrides)
	}
	return override
}

// ActivityOverrides 是「分组 -> 倍率覆盖」，同时承载分组级 override 与指定模型倍率；
// 继承型授权以空覆盖保留在映射中，表示该分组有有效活动授权。
type ActivityOverrides map[string]ratio_setting.ActivityRatioOverride

// GetActivityOverrides 把有效授权列表转换成「分组 -> 倍率覆盖」。
// 没有倍率覆盖的继承型授权也会保留为空覆盖，供计费解析器识别活动授权。
func GetActivityOverrides(views []ActivityView) ActivityOverrides {
	overrides := make(ActivityOverrides, len(views))
	for i := range views {
		if override := views[i].BillingRatioOverride(true); override != nil {
			overrides[views[i].GroupName] = *override
		}
	}
	return overrides
}

// ActivityRatioFor 返回倍率覆盖映射中某个分组的分组级倍率，未命中返回 nil。
func ActivityRatioFor(overrides ActivityOverrides, group string) *float64 {
	override, ok := overrides[group]
	if !ok {
		return nil
	}
	return override.GroupRatio
}

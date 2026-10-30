package service

import (
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

// ActivityView 是用户在某一个分组上的有效活动授权（同一分组的多张授权合并后的结果）：
// 倍率取最小值、RPM 取最大值（两项都对用户最有利），到期时间取最晚（有永久授权则永久）。
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
	RpmLimit            int                `json:"rpm_limit"`
	ExpireTime          int64              `json:"expire_time"`

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
	return activityPackageCandidate{GrantId: grant.Id, Remaining: remaining, ExpireTime: grant.ExpireTime}
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

// GetActiveActivityViews 读取用户当前有效的活动授权并按分组合并。
func GetActiveActivityViews(userId int) []ActivityView {
	if userId <= 0 {
		return nil
	}
	grants, err := model.GetActiveActivityGrants(userId)
	if err != nil {
		common.SysError("failed to load active activity grants: " + err.Error())
		return nil
	}
	return mergeActivityViews(grants)
}

type activityViewBuilder struct {
	view               ActivityView
	seen               bool
	groupUnlimitedFree bool
	groupPackages      []activityPackageCandidate
	groupFallbackRatio *float64

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
	if !builder.seen {
		builder.view.GrantId = grant.Id
		builder.view.Code = grant.Code
		builder.view.CodeName = grant.CodeName
		builder.view.RatioMode = grant.RatioMode
		builder.view.RatioOverride = grant.RatioOverride
		builder.view.RpmLimit = grant.RpmLimit
		builder.view.ExpireTime = grant.ExpireTime
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
		if grant.RpmLimit > builder.view.RpmLimit {
			builder.view.RpmLimit = grant.RpmLimit
		}
		if builder.view.ExpireTime == 0 || grant.ExpireTime == 0 {
			builder.view.ExpireTime = 0
		} else if grant.ExpireTime > builder.view.ExpireTime {
			builder.view.ExpireTime = grant.ExpireTime
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

// GetRequestActivityView 返回请求用户在某分组上的有效活动授权，结果按请求缓存在
// gin context 中，一次请求只查一次（鉴权、限流、计费共用）。
func GetRequestActivityView(c *gin.Context, userId int, group string) *ActivityView {
	if c == nil || userId <= 0 || group == "" {
		return nil
	}
	views, ok := common.GetContextKeyType[[]ActivityView](c, constant.ContextKeyActivityGrants)
	if !ok {
		views = GetActiveActivityViews(userId)
		common.SetContextKey(c, constant.ContextKeyActivityGrants, views)
	}
	for i := range views {
		if views[i].GroupName == group {
			return &views[i]
		}
	}
	return nil
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

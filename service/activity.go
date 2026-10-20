package service

import (
	"maps"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// ActivityView 是用户在某一个分组上的有效活动授权（同一分组的多张授权合并后的结果）：
// 倍率取最小值、RPM 取最大值（两项都对用户最有利），到期时间取最晚（有永久授权则永久）。
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

func mergeActivityViews(grants []model.ActivityGrant) []ActivityView {
	views := make([]ActivityView, 0, len(grants))
	indexByGroup := make(map[string]int, len(grants))
	for _, grant := range grants {
		index, exists := indexByGroup[grant.GroupName]
		if !exists {
			views = append(views, ActivityView{
				GrantId:       grant.Id,
				GroupName:     grant.GroupName,
				Code:          grant.Code,
				CodeName:      grant.CodeName,
				RatioMode:     grant.RatioMode,
				RatioOverride: grant.RatioOverride,
				// 复制而不是共享：合并时可能继续写入，授权本身（可能来自缓存）不能被改。
				ModelRatioOverrides: maps.Clone(grant.ModelRatioOverrides),
				RpmLimit:            grant.RpmLimit,
				ExpireTime:          grant.ExpireTime,
			})
			indexByGroup[grant.GroupName] = len(views) - 1
			continue
		}
		view := &views[index]
		if grant.RatioMode == model.ActivityRatioModeOverride {
			if view.RatioMode != model.ActivityRatioModeOverride || grant.RatioOverride < view.RatioOverride {
				view.RatioMode = model.ActivityRatioModeOverride
				view.RatioOverride = grant.RatioOverride
				view.GrantId = grant.Id
				view.Code = grant.Code
				view.CodeName = grant.CodeName
			}
		}
		// 指定模型倍率同样按「对用户最优」合并：并集 + 同名取最小倍率。
		for modelName, ratio := range grant.ModelRatioOverrides {
			if view.ModelRatioOverrides == nil {
				view.ModelRatioOverrides = make(map[string]float64, len(grant.ModelRatioOverrides))
			}
			if existing, ok := view.ModelRatioOverrides[modelName]; !ok || ratio < existing {
				view.ModelRatioOverrides[modelName] = ratio
			}
		}
		if grant.RpmLimit > view.RpmLimit {
			view.RpmLimit = grant.RpmLimit
		}
		if view.ExpireTime == 0 || grant.ExpireTime == 0 {
			view.ExpireTime = 0
		} else if grant.ExpireTime > view.ExpireTime {
			view.ExpireTime = grant.ExpireTime
		}
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
// 非 nil 的 view 始终返回非 nil 结构体：即使两个字段都为空（继承型授权），
// 非 nil 也代表「活动授权有效」，计费解析器据此让活动授权保持最高优先级。
// 名称不能是 RatioOverride：ActivityView 已有同名字段。
func (view *ActivityView) BillingRatioOverride() *ratio_setting.ActivityRatioOverride {
	if view == nil {
		return nil
	}
	override := &ratio_setting.ActivityRatioOverride{}
	if view.RatioMode == model.ActivityRatioModeOverride {
		ratio := view.RatioOverride
		override.GroupRatio = &ratio
	}
	if len(view.ModelRatioOverrides) > 0 {
		override.ModelRatios = view.ModelRatioOverrides
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
		if override := views[i].BillingRatioOverride(); override != nil {
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

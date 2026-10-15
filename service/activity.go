package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

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
	RpmLimit      int     `json:"rpm_limit"`
	ExpireTime    int64   `json:"expire_time"`
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
				RpmLimit:      grant.RpmLimit,
				ExpireTime:    grant.ExpireTime,
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

// ActivityViewRatioPointer 返回该授权的倍率覆盖指针；非 override 模式返回 nil。
func ActivityViewRatioPointer(view *ActivityView) *float64 {
	if view == nil || view.RatioMode != model.ActivityRatioModeOverride {
		return nil
	}
	ratio := view.RatioOverride
	return &ratio
}

// GetActivityRatioOverrides 把授权列表转换成「分组 -> 活动倍率」，只含 override 模式的授权。
func GetActivityRatioOverrides(views []ActivityView) map[string]float64 {
	overrides := make(map[string]float64, len(views))
	for _, view := range views {
		if view.RatioMode != model.ActivityRatioModeOverride {
			continue
		}
		overrides[view.GroupName] = view.RatioOverride
	}
	return overrides
}

// ActivityRatioFor 返回倍率覆盖映射中某个分组的值，未命中返回 nil。
func ActivityRatioFor(ratios map[string]float64, group string) *float64 {
	if len(ratios) == 0 {
		return nil
	}
	ratio, ok := ratios[group]
	if !ok {
		return nil
	}
	return &ratio
}

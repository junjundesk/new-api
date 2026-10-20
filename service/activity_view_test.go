package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 同一分组的多张授权合并时：分组级倍率取最小、指定模型倍率取并集且同名取最小、
// RPM 取最大、到期时间取最晚；合并不得改写授权快照本身（快照可能来自缓存）。
func TestMergeActivityViewsCombinesActivityOverrides(t *testing.T) {
	grants := []model.ActivityGrant{
		{
			Id: 1, GroupName: "vip", Code: "code-a", RatioMode: model.ActivityRatioModeOverride,
			RatioOverride: 0.8, RpmLimit: 10, ExpireTime: 1000,
			ModelRatioOverrides: model.ActivityModelRatioOverrides{"gpt-5": 0.5, "o3": 1},
		},
		{
			Id: 2, GroupName: "vip", Code: "code-b", RatioMode: model.ActivityRatioModeInherit,
			RpmLimit: 20, ExpireTime: 2000,
			ModelRatioOverrides: model.ActivityModelRatioOverrides{"gpt-5": 0.2},
		},
		{
			Id: 3, GroupName: "svip", Code: "code-c", RatioMode: model.ActivityRatioModeInherit,
		},
	}

	views := mergeActivityViews(grants)
	require.Len(t, views, 2)

	vip := views[0]
	assert.Equal(t, "vip", vip.GroupName)
	assert.Equal(t, 0.8, vip.RatioOverride)
	assert.Equal(t, 20, vip.RpmLimit)
	assert.Equal(t, int64(2000), vip.ExpireTime)
	assert.Equal(t, map[string]float64{"gpt-5": 0.2, "o3": 1}, vip.ModelRatioOverrides)

	assert.Equal(t, 0.5, grants[0].ModelRatioOverrides["gpt-5"], "合并不得改写授权快照")
	require.Len(t, grants[0].ModelRatioOverrides, 2)

	assert.Empty(t, views[1].ModelRatioOverrides, "没有指定模型的授权保持空表")
}

func TestActivityViewBillingRatioOverride(t *testing.T) {
	var nilView *ActivityView
	assert.Nil(t, nilView.BillingRatioOverride(), "nil 视图不 panic")

	inherited := &ActivityView{GroupName: "vip", RatioMode: model.ActivityRatioModeInherit}
	inheritedOverride := inherited.BillingRatioOverride()
	require.NotNil(t, inheritedOverride, "继承授权仍然标记授权有效，供计费保护最高优先级")
	assert.Nil(t, inheritedOverride.GroupRatio)
	assert.Empty(t, inheritedOverride.ModelRatios)

	groupOnly := (&ActivityView{GroupName: "vip", RatioMode: model.ActivityRatioModeOverride}).BillingRatioOverride()
	require.NotNil(t, groupOnly)
	require.NotNil(t, groupOnly.GroupRatio)
	assert.Equal(t, float64(0), *groupOnly.GroupRatio)
	assert.Empty(t, groupOnly.ModelRatios)

	modelsOnly := (&ActivityView{
		GroupName:           "vip",
		RatioMode:           model.ActivityRatioModeInherit,
		ModelRatioOverrides: map[string]float64{"gpt-5": 0},
	}).BillingRatioOverride()
	require.NotNil(t, modelsOnly)
	assert.Nil(t, modelsOnly.GroupRatio, "指定模型类型不产生分组级覆盖")
	assert.Equal(t, map[string]float64{"gpt-5": 0}, modelsOnly.ModelRatios)
}

func TestGetActivityOverridesAndActivityRatioFor(t *testing.T) {
	views := []ActivityView{
		{GroupName: "vip", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0.5},
		{GroupName: "svip", RatioMode: model.ActivityRatioModeInherit, ModelRatioOverrides: map[string]float64{"gpt-5": 0}},
		{GroupName: "plain", RatioMode: model.ActivityRatioModeInherit},
	}

	overrides := GetActivityOverrides(views)
	assert.Len(t, overrides, 3, "继承型授权也保留在映射中，表示活动授权有效")

	groupRatio := ActivityRatioFor(overrides, "vip")
	require.NotNil(t, groupRatio)
	assert.Equal(t, 0.5, *groupRatio)

	assert.Nil(t, ActivityRatioFor(overrides, "svip"), "指定模型类型没有分组级倍率")
	assert.Nil(t, ActivityRatioFor(overrides, "plain"))
	assert.NotEmpty(t, overrides["svip"].ModelRatios)
	assert.Empty(t, overrides["plain"].ModelRatios)
	assert.Nil(t, ActivityRatioFor(nil, "vip"))
}

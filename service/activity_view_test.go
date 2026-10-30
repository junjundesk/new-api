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
	assert.Nil(t, nilView.BillingRatioOverride(true), "nil 视图不 panic")

	inherited := &ActivityView{GroupName: "vip", RatioMode: model.ActivityRatioModeInherit}
	inheritedOverride := inherited.BillingRatioOverride(true)
	require.NotNil(t, inheritedOverride, "继承授权仍然标记授权有效，供计费保护最高优先级")
	assert.Nil(t, inheritedOverride.GroupRatio)
	assert.Empty(t, inheritedOverride.ModelRatios)

	groupOnly := (&ActivityView{GroupName: "vip", RatioMode: model.ActivityRatioModeOverride}).BillingRatioOverride(true)
	require.NotNil(t, groupOnly)
	require.NotNil(t, groupOnly.GroupRatio)
	assert.Equal(t, float64(0), *groupOnly.GroupRatio)
	assert.Empty(t, groupOnly.ModelRatios)

	modelsOnly := (&ActivityView{
		GroupName:           "vip",
		RatioMode:           model.ActivityRatioModeInherit,
		ModelRatioOverrides: map[string]float64{"gpt-5": 0},
	}).BillingRatioOverride(true)
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

func TestMergeActivityViewsTokenPackages(t *testing.T) {
	grants := []model.ActivityGrant{
		{Id: 1, GroupName: "vip", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0, TokenTotal: 1000, TokenUsed: 400, ExpireTime: 2000},
		{Id: 2, GroupName: "vip", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0, TokenTotal: 500, TokenUsed: 100, ExpireTime: 1000},
		{Id: 3, GroupName: "vip", RatioScope: model.ActivityRatioScopeModels, ModelRatioOverrides: model.ActivityModelRatioOverrides{"gpt-5": 0}, TokenTotal: 100, TokenUsed: 30, ExpireTime: 3000},
	}

	views := mergeActivityViews(grants)
	require.Len(t, views, 1)
	view := views[0]

	require.NotNil(t, view.groupTokenBudget)
	assert.Equal(t, int64(1000), view.groupTokenBudget.Remaining, "多个分组套餐的剩余量求和")
	assert.Equal(t, 2, view.groupTokenBudget.GrantId, "到期最早且有剩余的套餐优先扣减")
	assert.Equal(t, int64(1600), view.TokenTotal, "分组视图汇总该分组内所有套餐权益")
	assert.Equal(t, int64(530), view.TokenUsed)

	require.NotNil(t, view.modelTokenBudgets["gpt-5"])
	assert.Equal(t, int64(70), view.modelTokenBudgets["gpt-5"].Remaining)
	assert.Equal(t, 3, view.modelTokenBudgets["gpt-5"].GrantId)

	override := view.BillingRatioOverride(true)
	require.NotNil(t, override)
	require.NotNil(t, override.GroupRatio)
	assert.Equal(t, float64(0), *override.GroupRatio)
	assert.Equal(t, 2, override.GroupTokenGrantId)
	assert.Equal(t, float64(0), override.ModelRatios["gpt-5"])
	assert.Equal(t, 3, override.ModelTokenGrantIds["gpt-5"])

	withoutPackages := view.BillingRatioOverride(false)
	assert.Nil(t, withoutPackages.GroupRatio, "按次计费不允许套餐免费")
	_, hasModelFree := withoutPackages.ModelRatios["gpt-5"]
	assert.False(t, hasModelFree)
}

func TestMergeActivityViewsExhaustedPackageFallsBack(t *testing.T) {
	grants := []model.ActivityGrant{
		{Id: 1, GroupName: "vip", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0, TokenTotal: 100, TokenUsed: 100},
		{Id: 2, GroupName: "vip", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0.5},
	}

	view := mergeActivityViews(grants)[0]
	assert.True(t, view.TokenLimited)
	assert.Nil(t, view.groupTokenBudget, "用尽的套餐不再产生免费预算")
	assert.Equal(t, model.ActivityRatioModeOverride, view.RatioMode)
	assert.Equal(t, 0.5, view.RatioOverride)

	override := view.BillingRatioOverride(true)
	require.NotNil(t, override.GroupRatio)
	assert.Equal(t, 0.5, *override.GroupRatio)
	assert.Equal(t, 0, override.GroupTokenGrantId)
}

func TestMergeActivityViewsUnlimitedFreeOutranksPackages(t *testing.T) {
	grants := []model.ActivityGrant{
		{Id: 1, GroupName: "vip", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0, TokenTotal: 100},
		{Id: 2, GroupName: "vip", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0},
	}

	view := mergeActivityViews(grants)[0]
	assert.Nil(t, view.groupTokenBudget, "不限量免费优先，套餐不参与扣减")
	override := view.BillingRatioOverride(true)
	require.NotNil(t, override.GroupRatio)
	assert.Equal(t, float64(0), *override.GroupRatio)
	assert.Equal(t, 0, override.GroupTokenGrantId)
}

func TestAggregateActivityPackageCandidates(t *testing.T) {
	remaining, grantId := aggregateActivityPackageCandidates([]activityPackageCandidate{
		{GrantId: 1, Remaining: 100, ExpireTime: 3000},
		{GrantId: 2, Remaining: 0, ExpireTime: 1000},
		{GrantId: 3, Remaining: 50, ExpireTime: 0},
	})
	assert.Equal(t, int64(150), remaining)
	assert.Equal(t, 1, grantId, "永久套餐排在有期限套餐之后")

	remaining, grantId = aggregateActivityPackageCandidates([]activityPackageCandidate{
		{GrantId: 4, Remaining: 10, ExpireTime: 0},
		{GrantId: 5, Remaining: 10, ExpireTime: 500},
	})
	assert.Equal(t, int64(20), remaining)
	assert.Equal(t, 5, grantId)

	remaining, grantId = aggregateActivityPackageCandidates(nil)
	assert.Equal(t, int64(0), remaining)
	assert.Equal(t, 0, grantId)
}

func TestMergeActivityViewsExhaustedPackageWithoutFallbackIsNotFree(t *testing.T) {
	grants := []model.ActivityGrant{
		{Id: 1, GroupName: "vip", RatioMode: model.ActivityRatioModeOverride, RatioOverride: 0, TokenTotal: 100, TokenUsed: 100},
		{Id: 2, GroupName: "vip", RatioMode: model.ActivityRatioModeInherit},
	}

	view := mergeActivityViews(grants)[0]
	assert.Nil(t, view.groupTokenBudget)
	assert.Equal(t, model.ActivityRatioModeInherit, view.RatioMode, "用尽后展示字段不能停在 0 被误判为免费")
	assert.Equal(t, float64(0), view.RatioOverride)

	override := view.BillingRatioOverride(true)
	require.NotNil(t, override, "活动授权仍然保留优先权")
	assert.Nil(t, override.GroupRatio, "没有其他优惠时按分组原价计费")
}

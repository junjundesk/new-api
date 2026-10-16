package ratio_setting

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func activityGroupOverride(ratio float64) *ActivityRatioOverride {
	return &ActivityRatioOverride{GroupRatio: &ratio}
}

func activityModelOverride(ratios map[string]float64) *ActivityRatioOverride {
	return &ActivityRatioOverride{ModelRatios: ratios}
}

// 活动码倍率必须插在渠道-模型覆盖价之后、用户分组倍率矩阵之前。
func TestResolveModelGroupRatioActivityPriority(t *testing.T) {
	savedGroups := GroupRatio2JSONString()
	savedSpecialGroups := GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		assert.NoError(t, UpdateGroupRatioByJSONString(savedGroups))
		assert.NoError(t, UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":2,"activity-group":1.5}`))
	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"member":{"activity-group":0.6}}`))

	t.Run("activity ratio outranks the user group matrix", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, activityGroupOverride(0.2))
		assert.Equal(t, 0.2, info.GroupRatio)
		assert.Equal(t, 0.2, info.GroupSpecialRatio)
		assert.True(t, info.HasSpecialRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ChannelModelRatio)
	})

	t.Run("zero activity ratio means the granted group is free", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, activityGroupOverride(0))
		assert.Equal(t, float64(0), info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
	})

	t.Run("the price without the grant is frozen for the consume log", func(t *testing.T) {
		fromMatrix := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, activityGroupOverride(0))
		require.NotNil(t, fromMatrix.ActivityOriginalRatio)
		assert.Equal(t, 0.6, *fromMatrix.ActivityOriginalRatio, "the user group matrix is what the request would have cost")

		fromGroup := ResolveModelGroupRatio("other", "activity-group", "billing-model", nil, activityGroupOverride(0))
		require.NotNil(t, fromGroup.ActivityOriginalRatio)
		assert.Equal(t, 1.5, *fromGroup.ActivityOriginalRatio, "without a matrix entry the group's own ratio applies")
	})

	t.Run("no original price is frozen without an activity ratio", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, nil)
		assert.Nil(t, info.ActivityOriginalRatio)
	})

	t.Run("channel model override still wins over an activity ratio", func(t *testing.T) {
		channelRatio := 0.05
		overrides := map[string]*float64{"billing-model": &channelRatio}
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", overrides, activityGroupOverride(0.2))
		assert.Equal(t, 0.05, info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceChannelModel, info.Source)
		require.NotNil(t, info.ChannelModelRatio)
		assert.Equal(t, 0.05, *info.ChannelModelRatio)
	})

	t.Run("without an activity ratio the user group matrix still applies", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, nil)
		assert.Equal(t, 0.6, info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceUserGroup, info.Source)
	})

	t.Run("invalid activity ratios are ignored", func(t *testing.T) {
		invalidRatios := []float64{math.NaN(), math.Inf(1), -1, float64(dto.MaxChannelModelGroupRatio) + 1}
		for _, ratio := range invalidRatios {
			info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, activityGroupOverride(ratio))
			assert.Equal(t, 0.6, info.GroupRatio, "invalid activity ratio %v must be ignored", ratio)
			assert.Equal(t, types.GroupRatioSourceUserGroup, info.Source)
		}
	})
}

// 「指定模型」类型：命中模型使用模型倍率（可无视分组定价），未命中模型回落到分组级倍率或矩阵。
func TestResolveModelGroupRatioActivityModelOverride(t *testing.T) {
	savedGroups := GroupRatio2JSONString()
	savedSpecialGroups := GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		assert.NoError(t, UpdateGroupRatioByJSONString(savedGroups))
		assert.NoError(t, UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":2,"activity-group":1.5}`))
	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"member":{"activity-group":0.6}}`))

	t.Run("a listed model ignores the group pricing", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "gpt-5", nil,
			activityModelOverride(map[string]float64{"gpt-5": 0}))
		assert.Equal(t, float64(0), info.GroupRatio)
		assert.Equal(t, float64(0), info.GroupSpecialRatio)
		assert.True(t, info.HasSpecialRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ChannelModelRatio)
		require.NotNil(t, info.ActivityOriginalRatio)
		assert.Equal(t, 0.6, *info.ActivityOriginalRatio, "the price without the grant stays visible in the log")
	})

	t.Run("an unlisted model keeps the user group matrix", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "o3", nil,
			activityModelOverride(map[string]float64{"gpt-5": 0}))
		assert.Equal(t, 0.6, info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceUserGroup, info.Source)
		assert.Nil(t, info.ActivityOriginalRatio)
	})

	t.Run("a listed model outranks the group-level activity ratio", func(t *testing.T) {
		activity := activityModelOverride(map[string]float64{"gpt-5": 0.1})
		groupRatio := 0.8
		activity.GroupRatio = &groupRatio

		listed := ResolveModelGroupRatio("member", "activity-group", "gpt-5", nil, activity)
		assert.Equal(t, 0.1, listed.GroupRatio)

		unlisted := ResolveModelGroupRatio("member", "activity-group", "o3", nil, activity)
		assert.Equal(t, 0.8, unlisted.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, unlisted.Source)
	})

	t.Run("the billing model name is what matches", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "gpt-5-2026", nil,
			activityModelOverride(map[string]float64{"gpt-5-2026": 0.25}))
		assert.Equal(t, 0.25, info.GroupRatio)
	})

	t.Run("channel model override still wins over a listed model", func(t *testing.T) {
		channelRatio := 0.05
		overrides := map[string]*float64{"gpt-5": &channelRatio}
		info := ResolveModelGroupRatio("member", "activity-group", "gpt-5", overrides,
			activityModelOverride(map[string]float64{"gpt-5": 0}))
		assert.Equal(t, 0.05, info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceChannelModel, info.Source)
	})

	t.Run("invalid model ratios are ignored", func(t *testing.T) {
		invalidRatios := []float64{math.NaN(), math.Inf(1), -1, float64(dto.MaxChannelModelGroupRatio) + 1}
		for _, ratio := range invalidRatios {
			info := ResolveModelGroupRatio("member", "activity-group", "gpt-5", nil,
				activityModelOverride(map[string]float64{"gpt-5": ratio}))
			assert.Equal(t, 0.6, info.GroupRatio, "invalid model ratio %v must be ignored", ratio)
			assert.Equal(t, types.GroupRatioSourceUserGroup, info.Source)
		}
	})

	t.Run("an empty override changes nothing", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "gpt-5", nil, &ActivityRatioOverride{})
		assert.Equal(t, 0.6, info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceUserGroup, info.Source)
		assert.Nil(t, info.ActivityOriginalRatio)
	})
}

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

// 活动码授权是最高优先级：活动倍率（指定模型 > 分组级 > 继承分组倍率）
// 优先于渠道模型专属倍率；没有活动授权时才轮到渠道倍率。
func TestResolveModelGroupRatioActivityPriority(t *testing.T) {
	savedGroups := GroupRatio2JSONString()
	savedSpecialGroups := GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		assert.NoError(t, UpdateGroupRatioByJSONString(savedGroups))
		assert.NoError(t, UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":2,"activity-group":1.5,"free-group":0}`))
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

	t.Run("activity ratio outranks a channel model override", func(t *testing.T) {
		channelRatio := 0.05
		overrides := map[string]*float64{"billing-model": &channelRatio}
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", overrides, activityGroupOverride(0.2))
		assert.Equal(t, 0.2, info.GroupRatio)
		assert.Equal(t, 0.2, info.GroupSpecialRatio)
		assert.True(t, info.HasSpecialRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ChannelModelRatio)
		require.NotNil(t, info.ActivityOriginalRatio)
		assert.Equal(t, 0.05, *info.ActivityOriginalRatio, "the channel price is the price without the grant")
	})

	t.Run("without an activity ratio the user group matrix still applies", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, nil)
		assert.Equal(t, 0.6, info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceUserGroup, info.Source)
	})

	t.Run("invalid activity ratios fall back to the group matrix under the grant", func(t *testing.T) {
		invalidRatios := []float64{math.NaN(), math.Inf(1), -1, float64(dto.MaxChannelModelGroupRatio) + 1}
		for _, ratio := range invalidRatios {
			info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, activityGroupOverride(ratio))
			assert.Equal(t, 0.6, info.GroupRatio, "invalid activity ratio %v must be ignored", ratio)
			assert.Equal(t, 0.6, info.GroupSpecialRatio)
			assert.True(t, info.HasSpecialRatio)
			assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
			assert.Nil(t, info.ActivityOriginalRatio)
		}
	})
}

// 「指定模型」类型：命中模型使用模型倍率（可无视分组定价），未命中模型按活动授权继承分组倍率；
// 活动授权存在时渠道模型专属倍率一律不参与。
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

	t.Run("an unlisted model inherits the user group matrix under the grant", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "o3", nil,
			activityModelOverride(map[string]float64{"gpt-5": 0}))
		assert.Equal(t, 0.6, info.GroupRatio)
		assert.Equal(t, 0.6, info.GroupSpecialRatio)
		assert.True(t, info.HasSpecialRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
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

	t.Run("a listed model outranks a channel model override", func(t *testing.T) {
		channelRatio := 0.05
		overrides := map[string]*float64{"gpt-5": &channelRatio}
		info := ResolveModelGroupRatio("member", "activity-group", "gpt-5", overrides,
			activityModelOverride(map[string]float64{"gpt-5": 0.2}))
		assert.Equal(t, 0.2, info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ChannelModelRatio)
		require.NotNil(t, info.ActivityOriginalRatio)
		assert.Equal(t, 0.05, *info.ActivityOriginalRatio)
	})

	t.Run("invalid model ratios fall back to the group matrix under the grant", func(t *testing.T) {
		invalidRatios := []float64{math.NaN(), math.Inf(1), -1, float64(dto.MaxChannelModelGroupRatio) + 1}
		for _, ratio := range invalidRatios {
			info := ResolveModelGroupRatio("member", "activity-group", "gpt-5", nil,
				activityModelOverride(map[string]float64{"gpt-5": ratio}))
			assert.Equal(t, 0.6, info.GroupRatio, "invalid model ratio %v must be ignored", ratio)
			assert.Equal(t, 0.6, info.GroupSpecialRatio)
			assert.True(t, info.HasSpecialRatio)
			assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
			assert.Nil(t, info.ActivityOriginalRatio)
		}
	})

	t.Run("an inherit grant keeps the user group matrix", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "gpt-5", nil, &ActivityRatioOverride{})
		assert.Equal(t, 0.6, info.GroupRatio)
		assert.Equal(t, 0.6, info.GroupSpecialRatio)
		assert.True(t, info.HasSpecialRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ActivityOriginalRatio)
	})
}

func TestResolveModelGroupRatioActivityFreePrecedence(t *testing.T) {
	savedGroups := GroupRatio2JSONString()
	savedSpecialGroups := GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		assert.NoError(t, UpdateGroupRatioByJSONString(savedGroups))
		assert.NoError(t, UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":2,"activity-group":1.5,"free-group":0}`))
	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"member":{"activity-group":0.6}}`))

	channelRatio := 0.05
	overrides := map[string]*float64{"billing-model": &channelRatio}

	t.Run("a free group-level grant outranks the channel override", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", overrides, activityGroupOverride(0))
		assert.Equal(t, float64(0), info.GroupRatio)
		assert.Equal(t, float64(0), info.GroupSpecialRatio)
		assert.True(t, info.HasSpecialRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ChannelModelRatio)
		require.NotNil(t, info.ActivityOriginalRatio)
		assert.Equal(t, 0.05, *info.ActivityOriginalRatio)
	})

	t.Run("a free model-level grant outranks the channel override", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", overrides,
			activityModelOverride(map[string]float64{"billing-model": 0}))
		assert.Equal(t, float64(0), info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ChannelModelRatio)
		require.NotNil(t, info.ActivityOriginalRatio)
		assert.Equal(t, 0.05, *info.ActivityOriginalRatio)
	})

	t.Run("an inherit grant on a free group outranks the channel override", func(t *testing.T) {
		info := ResolveModelGroupRatio("other", "free-group", "billing-model", overrides, &ActivityRatioOverride{})
		assert.Equal(t, float64(0), info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ChannelModelRatio)
		require.NotNil(t, info.ActivityOriginalRatio)
		assert.Equal(t, 0.05, *info.ActivityOriginalRatio)
	})

	t.Run("an inherit grant on a paid group keeps the group ratio", func(t *testing.T) {
		info := ResolveModelGroupRatio("other", "activity-group", "billing-model", overrides, &ActivityRatioOverride{})
		assert.Equal(t, 1.5, info.GroupRatio)
		assert.Equal(t, 1.5, info.GroupSpecialRatio)
		assert.True(t, info.HasSpecialRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ChannelModelRatio)
		require.NotNil(t, info.ActivityOriginalRatio)
		assert.Equal(t, 0.05, *info.ActivityOriginalRatio)
	})

	t.Run("without a grant the channel override still wins", func(t *testing.T) {
		info := ResolveModelGroupRatio("other", "activity-group", "billing-model", overrides, nil)
		assert.Equal(t, 0.05, info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceChannelModel, info.Source)
		require.NotNil(t, info.ChannelModelRatio)
	})
}

func TestResolveModelGroupRatioRecordsTokenPackageGrant(t *testing.T) {
	savedGroups := GroupRatio2JSONString()
	t.Cleanup(func() {
		assert.NoError(t, UpdateGroupRatioByJSONString(savedGroups))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":2,"package-group":1.5}`))

	zero := 0.0
	activity := &ActivityRatioOverride{
		GroupRatio:        &zero,
		GroupTokenGrantId: 11,
		ModelRatios:       map[string]float64{"gpt-5": 0},
		ModelTokenGrantIds: map[string]int{
			"gpt-5": 12,
		},
	}

	modelGranted := ResolveModelGroupRatio("default", "package-group", "gpt-5", nil, activity)
	assert.Equal(t, float64(0), modelGranted.GroupRatio)
	assert.Equal(t, 12, modelGranted.ActivityGrantId, "命中的单模型套餐记录自己的授权 id")

	groupGranted := ResolveModelGroupRatio("default", "package-group", "o3", nil, activity)
	assert.Equal(t, float64(0), groupGranted.GroupRatio)
	assert.Equal(t, 11, groupGranted.ActivityGrantId, "未命中模型使用分组套餐授权 id")

	unlimited := &ActivityRatioOverride{GroupRatio: &zero}
	unlimitedInfo := ResolveModelGroupRatio("default", "package-group", "o3", nil, unlimited)
	assert.Equal(t, float64(0), unlimitedInfo.GroupRatio)
	assert.Equal(t, 0, unlimitedInfo.ActivityGrantId, "不限量免费不扣减任何套餐")

	withoutGrant := ResolveModelGroupRatio("default", "package-group", "o3", nil, nil)
	assert.Equal(t, 0, withoutGrant.ActivityGrantId)
}

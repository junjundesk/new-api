package ratio_setting

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	pointerTo := func(value float64) *float64 { return &value }

	t.Run("activity ratio outranks the user group matrix", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, pointerTo(0.2))
		assert.Equal(t, 0.2, info.GroupRatio)
		assert.Equal(t, 0.2, info.GroupSpecialRatio)
		assert.True(t, info.HasSpecialRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
		assert.Nil(t, info.ChannelModelRatio)
	})

	t.Run("zero activity ratio means the granted group is free", func(t *testing.T) {
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, pointerTo(0))
		assert.Equal(t, float64(0), info.GroupRatio)
		assert.Equal(t, types.GroupRatioSourceActivity, info.Source)
	})

	t.Run("channel model override still wins over an activity ratio", func(t *testing.T) {
		channelRatio := 0.05
		overrides := map[string]*float64{"billing-model": &channelRatio}
		info := ResolveModelGroupRatio("member", "activity-group", "billing-model", overrides, pointerTo(0.2))
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
			info := ResolveModelGroupRatio("member", "activity-group", "billing-model", nil, pointerTo(ratio))
			assert.Equal(t, 0.6, info.GroupRatio, "invalid activity ratio %v must be ignored", ratio)
			assert.Equal(t, types.GroupRatioSourceUserGroup, info.Source)
		}
	})
}

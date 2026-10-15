package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveChannelModelRatioPriority(t *testing.T) {
	savedGroups := GroupRatio2JSONString()
	savedSpecialGroups := GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		assert.NoError(t, UpdateGroupRatioByJSONString(savedGroups))
		assert.NoError(t, UpdateGroupGroupRatioByJSONString(savedSpecialGroups))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":1.7}`))
	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"member":{"default":0.6}}`))

	tests := []struct {
		name          string
		userGroup     string
		modelName     string
		overridesJSON string
		wantRatio     float64
		wantSpecial   float64
		wantSource    string
	}{
		{
			name: "missing override uses user group special", userGroup: "member", modelName: "billing-model",
			overridesJSON: `{}`, wantRatio: 0.6, wantSpecial: 0.6, wantSource: types.GroupRatioSourceUserGroup,
		},
		{
			name: "missing user special uses group default", userGroup: "ordinary", modelName: "billing-model",
			overridesJSON: `{}`, wantRatio: 1.7, wantSpecial: -1, wantSource: types.GroupRatioSourceGroup,
		},
		{
			name: "zero replaces both group ratios", userGroup: "member", modelName: "billing-model",
			overridesJSON: `{"billing-model":0}`, wantRatio: 0, wantSpecial: -1, wantSource: types.GroupRatioSourceChannelModel,
		},
		{
			name: "one replaces rather than inherits", userGroup: "member", modelName: "billing-model",
			overridesJSON: `{"billing-model":1}`, wantRatio: 1, wantSpecial: -1, wantSource: types.GroupRatioSourceChannelModel,
		},
		{
			name: "exact model only without pricing wildcard fallback", userGroup: "member", modelName: "gpt-4-gizmo-channel-test",
			overridesJSON: `{"gpt-4-gizmo-channel-test":0.8,"gpt-4-gizmo-*":0}`, wantRatio: 0.8, wantSpecial: -1, wantSource: types.GroupRatioSourceChannelModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var overrides map[string]*float64
			require.NoError(t, common.UnmarshalJsonStr(tt.overridesJSON, &overrides))

			got := ResolveModelGroupRatio(tt.userGroup, "default", tt.modelName, overrides, nil)

			assert.Equal(t, tt.wantRatio, got.GroupRatio)
			assert.Equal(t, tt.wantSource, got.Source)
			assert.Equal(t, tt.wantSpecial, got.GroupSpecialRatio)
			assert.Equal(t, tt.wantSource == types.GroupRatioSourceUserGroup, got.HasSpecialRatio)
			if tt.wantSource != types.GroupRatioSourceChannelModel {
				assert.Nil(t, got.ChannelModelRatio)
				return
			}
			require.NotNil(t, got.ChannelModelRatio)
			assert.Equal(t, tt.wantRatio, *got.ChannelModelRatio)
			*overrides[tt.modelName] = 99
			assert.Equal(t, tt.wantRatio, *got.ChannelModelRatio, "a resolved attempt freezes the override value")
			assert.Equal(t, tt.wantRatio, got.GroupRatio)

			if tt.modelName == "gpt-4-gizmo-channel-test" {
				delete(overrides, tt.modelName)
				withoutExact := ResolveModelGroupRatio(tt.userGroup, "default", tt.modelName, overrides, nil)
				assert.Equal(t, 0.6, withoutExact.GroupRatio, "the wildcard used for base pricing must not supply a channel override")
				assert.Equal(t, types.GroupRatioSourceUserGroup, withoutExact.Source)
				assert.Nil(t, withoutExact.ChannelModelRatio)
			}
		})
	}
}

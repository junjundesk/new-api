package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerformanceGroupMetadataHidesDisabledGroupsFromUsers(t *testing.T) {
	originalGroups := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","disabled":"Disabled"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
	})

	userMetadata := getUserPerformanceGroupMetadata("default")
	_, userCanSeeDisabled := userMetadata["disabled"]
	assert.False(t, userCanSeeDisabled)
	assert.Contains(t, userMetadata, "default")

	adminMetadata := getAllPerformanceGroupMetadata()
	disabled, adminCanSeeDisabled := adminMetadata["disabled"]
	require.True(t, adminCanSeeDisabled)
	assert.Equal(t, "Disabled", disabled.Description)
	assert.Equal(t, float64(0), disabled.Ratio)
}

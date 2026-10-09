package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"quota_saturation": map[string]interface{}{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}

func TestFormatUserLogsStripsAdminRequestIP(t *testing.T) {
	logs := []*Log{{
		Other: common.MapToJsonStr(map[string]interface{}{
			"admin_info": map[string]interface{}{
				"request_ip": "203.0.113.42",
			},
		}),
	}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, parsed, "admin_info")
}

func TestFormatLogsPreservesAdminRequestIPForAdmin(t *testing.T) {
	logs := []*Log{{
		Other: common.MapToJsonStr(map[string]interface{}{
			"admin_info": map[string]interface{}{
				"request_ip": "203.0.113.42",
			},
		}),
	}}

	formatLogs(logs, 0, true)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	adminInfo, ok := parsed["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "203.0.113.42", adminInfo["request_ip"])
}

func TestAttachRequestIPForAdminPreservesExistingAdminInfo(t *testing.T) {
	other := map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"usage_billing_path": "upstream",
		},
	}

	attachRequestIPForAdmin(other, "2001:db8::42")

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "upstream", adminInfo["usage_billing_path"])
	assert.Equal(t, "2001:db8::42", adminInfo["request_ip"])
}

func TestAttachRequestIPForAdminCreatesAdminInfo(t *testing.T) {
	other := attachRequestIPForAdmin(nil, "203.0.113.42")

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "203.0.113.42", adminInfo["request_ip"])
}

// TestFormatUserLogsStripsModelMapping verifies the channel model mapping
// details (nested under other.admin_info) never reach non-admin log views:
// mapping is channel configuration, not user-facing request information.
func TestFormatUserLogsStripsModelMapping(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"is_model_mapped":     true,
			"upstream_model_name": "qwen-turbo",
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, parsed, "admin_info")
	assert.NotContains(t, parsed, "is_model_mapped")
	assert.NotContains(t, parsed, "upstream_model_name")
}

// TestFormatLogsKeepsModelMappingForAdmin verifies admins still receive the
// mapping details nested under admin_info.
func TestFormatLogsKeepsModelMappingForAdmin(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"admin_info": map[string]interface{}{
			"is_model_mapped":     true,
			"upstream_model_name": "qwen-turbo",
		},
	})
	logs := []*Log{{Other: other}}

	formatLogs(logs, 0, true)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	adminInfo, ok := parsed["admin_info"].(map[string]interface{})
	require.True(t, ok, "admin views keep admin_info")
	assert.Equal(t, true, adminInfo["is_model_mapped"])
	assert.Equal(t, "qwen-turbo", adminInfo["upstream_model_name"])
}

// TestFormatUserLogsStripsLegacyTopLevelModelMapping verifies the legacy layout
// of the model mapping fields (written before they moved under admin_info) is
// also removed for non-admin views. Only the UI read path was gated before,
// which still leaked the upstream model name over the API.
func TestFormatUserLogsStripsLegacyTopLevelModelMapping(t *testing.T) {
	logs := []*Log{{
		Other: common.MapToJsonStr(map[string]interface{}{
			"model_price":         0.004,
			"is_model_mapped":     true,
			"upstream_model_name": "qwen-turbo",
		}),
	}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, parsed, "is_model_mapped")
	assert.NotContains(t, parsed, "upstream_model_name")
	// Non-admin billing fields remain visible.
	assert.Contains(t, parsed, "model_price")
}

// TestFormatLogsKeepsLegacyTopLevelModelMappingForAdmin verifies admins still
// see the legacy top-level mapping fields on historical logs.
func TestFormatLogsKeepsLegacyTopLevelModelMappingForAdmin(t *testing.T) {
	logs := []*Log{{
		Other: common.MapToJsonStr(map[string]interface{}{
			"is_model_mapped":     true,
			"upstream_model_name": "qwen-turbo",
		}),
	}}

	formatLogs(logs, 0, true)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.Equal(t, true, parsed["is_model_mapped"])
	assert.Equal(t, "qwen-turbo", parsed["upstream_model_name"])
}

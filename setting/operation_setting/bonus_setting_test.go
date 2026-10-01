package operation_setting

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBonusSettingExportedOptionKeys pins the option keys the frontend reads
// (web/src/features/system-settings reads `settings['bonus_setting.<name>']`).
// Renaming a json tag here silently breaks the settings form, so the mapping is
// asserted explicitly.
func TestBonusSettingExportedOptionKeys(t *testing.T) {
	exported := config.GlobalConfig.ExportAllConfigs()
	require.NotEmpty(t, exported)

	for _, key := range []string{
		"bonus_setting.signup_bonus_enabled",
		"bonus_setting.signup_bonus_amount",
		"bonus_setting.signup_bonus_duration",
		"bonus_setting.bonus_email_subject",
		"bonus_setting.bonus_email_content",
	} {
		assert.Contains(t, exported, key, "frontend reads this option key")
	}
}

// TestDefaultBonusEmailContentUsesSupportedPlaceholders guards the shipped
// template against placeholders that renderBonusEmail does not substitute.
func TestDefaultBonusEmailContentUsesSupportedPlaceholders(t *testing.T) {
	supported := []string{
		"$username", "$display_name", "$amount",
		"$expire_time", "$expire_text",
		"$remark_block", "$remark", "$site_name",
	}

	remaining := DefaultBonusEmailContent
	for _, placeholder := range supported {
		remaining = strings.ReplaceAll(remaining, placeholder, "")
	}
	assert.NotContains(t, remaining, "$", "default template references an unknown placeholder")
}

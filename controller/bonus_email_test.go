package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderBonusEmailSubstitutesPlaceholders pins the template contract that
// the admin-facing settings UI documents.
func TestRenderBonusEmailSubstitutesPlaceholders(t *testing.T) {
	vars := bonusEmailVars{
		Username:    "alice",
		DisplayName: "Alice",
		Amount:      500000,
		ExpireTime:  1791000000,
		Remark:      "新用户福利",
	}

	got := renderBonusEmail(
		"$username|$display_name|$amount|$expire_time|$expire_text|$remark|$site_name",
		vars,
	)
	assert.Contains(t, got, "alice")
	assert.Contains(t, got, "Alice")
	assert.Contains(t, got, "新用户福利")
	assert.Contains(t, got, "2026-")
	// $expire_text carries the full sentence, $expire_time only the stamp.
	assert.Contains(t, got, "到期")
	assert.NotContains(t, got, "$", "every placeholder must be substituted")
}

// TestRenderBonusEmailEmptyValues covers the never-expires and no-remark paths,
// where the template must not leave dangling labels like "有效期：".
func TestRenderBonusEmailEmptyValues(t *testing.T) {
	vars := bonusEmailVars{
		Username:    "bob",
		DisplayName: "",
		Amount:      1000,
		ExpireTime:  0,
		Remark:      "",
	}

	got := renderBonusEmail("$display_name|$expire_time|$expire_text|$remark_block|$remark", vars)
	require.NotContains(t, got, "$")

	parts := splitOnPipe(t, got)
	assert.Equal(t, "bob", parts[0], "empty display name falls back to username")
	assert.Equal(t, "", parts[1], "never-expiring grants leave $expire_time empty")
	assert.Equal(t, "永久有效", parts[2])
	assert.Equal(t, "", parts[3], "no remark means no remark block at all")
	assert.Equal(t, "", parts[4])
}

func TestRenderBonusEmailRemarkBlock(t *testing.T) {
	vars := bonusEmailVars{Username: "carol", Amount: 5, Remark: "  活动赠送  "}
	got := renderBonusEmail("$remark_block", vars)
	assert.Equal(t, "<p>备注：活动赠送</p>", got)
}

// TestRenderBonusEmailDoesNotRescanValues guards against a remark that contains
// placeholder syntax being expanded a second time.
func TestRenderBonusEmailDoesNotRescanValues(t *testing.T) {
	vars := bonusEmailVars{Username: "dave", Amount: 5, Remark: "$amount"}
	got := renderBonusEmail("$remark", vars)
	assert.Equal(t, "$amount", got, "values must be inserted literally, never re-expanded")
}

func splitOnPipe(t *testing.T, s string) []string {
	t.Helper()
	parts := make([]string, 0, 5)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

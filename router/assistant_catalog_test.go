package router

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssistantOperationCatalogCoversDashboardAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	total := 0
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/") || route.Path == "/api" {
			total++
		}
	}
	require.Greater(t, total, 0)
	operations := assistantops.All()
	excluded := assistantops.ExcludedCount(engine.Routes())
	require.Equal(t, total, len(operations)+excluded, "every /api route must be catalogued or explicitly excluded")

	userOperations := 0
	adminOperations := 0
	for _, op := range operations {
		assert.False(t, strings.HasPrefix(op.Path, "/api/assistant"), op.Path)
		assert.NotContains(t, op.Path, "/webhook")
		assert.NotContains(t, op.Path, "/passkey")
		assert.NotContains(t, op.Path, "/2fa")
		assert.NotContains(t, op.Path, "/token/:id/key")
		if assistantops.Visible(op, "user", common.RoleCommonUser) {
			userOperations++
			assert.Equal(t, assistantops.ScopeUser, op.Scope, op.Path)
			for _, adminPrefix := range []string{"/api/channel", "/api/option", "/api/redemption", "/api/performance", "/api/system-task", "/api/system-info", "/api/custom-oauth-provider", "/api/ratio_sync", "/api/deployments", "/api/vendors", "/api/prefill_group"} {
				assert.False(t, strings.HasPrefix(op.Path, adminPrefix), op.Path)
			}
			assert.NotEqual(t, "GET /api/user/", op.ID)
			assert.NotEqual(t, "PUT /api/user/", op.ID)
			assert.NotEqual(t, "DELETE /api/user/:id", op.ID)
			assert.NotEqual(t, "GET /api/models/", op.ID)
		}
		if assistantops.Visible(op, "admin", common.RoleAdminUser) {
			adminOperations++
		}
	}
	assert.Greater(t, userOperations, 0)
	assert.Greater(t, adminOperations, userOperations)

	adminUsers, ok := assistantops.Find("GET /api/user/")
	require.True(t, ok)
	assert.False(t, assistantops.Visible(adminUsers, "user", common.RoleCommonUser))
	assert.True(t, assistantops.Visible(adminUsers, "admin", common.RoleAdminUser))

	rootOption, ok := assistantops.Find("PUT /api/option/")
	require.True(t, ok)
	assert.False(t, assistantops.Visible(rootOption, "admin", common.RoleAdminUser))
	assert.True(t, assistantops.Visible(rootOption, "admin", common.RoleRootUser))

	bonusGrant, ok := assistantops.Find("POST /api/user/:id/bonus")
	require.True(t, ok)
	assert.Equal(t, assistantops.ScopeAdmin, bonusGrant.Scope)
	assert.Equal(t, assistantops.RiskWrite, bonusGrant.Risk)
	bonusFields := map[string]assistantops.BodyField{}
	for _, field := range bonusGrant.BodyFields {
		bonusFields[field.Name] = field
	}
	require.Contains(t, bonusFields, "amount")
	require.Contains(t, bonusFields, "expire_input")
	require.Contains(t, bonusFields, "send_email")
	assert.True(t, bonusFields["amount"].Required)

	bonusRevoke, ok := assistantops.Find("POST /api/user/:id/bonus/:grant_id/revoke")
	require.True(t, ok)
	assert.Equal(t, assistantops.RiskDestructive, bonusRevoke.Risk)
	assert.False(t, bonusRevoke.Batchable)
	require.Len(t, bonusRevoke.BodyFields, 1)
	assert.Equal(t, "remark", bonusRevoke.BodyFields[0].Name)

	// Documentation must not drift away from the router.
	assert.Empty(t, assistantops.UnmatchedDocs(), "documented operations that no longer exist in the catalog")

	// Risk drives confirmation: deletions, site-wide writes, and GET routes
	// with side effects must not be classified as plain reads or writes.
	expectedRisk := map[string]assistantops.Risk{
		"DELETE /api/user/:id":            assistantops.RiskDestructive,
		"DELETE /api/channel/:id":         assistantops.RiskDestructive,
		"POST /api/channel/batch":         assistantops.RiskDestructive,
		"POST /api/user/manage":           assistantops.RiskDestructive,
		"POST /api/user/bonus/grant_all":  assistantops.RiskDestructive,
		"PUT /api/option/":                assistantops.RiskDestructive,
		"GET /api/channel/test":           assistantops.RiskWrite,
		"GET /api/channel/update_balance": assistantops.RiskWrite,
		"POST /api/channel/:id/status":    assistantops.RiskWrite,
		"GET /api/channel/":               assistantops.RiskRead,
	}
	for id, risk := range expectedRisk {
		op, found := assistantops.Find(id)
		require.True(t, found, id)
		assert.Equal(t, risk, op.Risk, id)
	}

	// Routes that hand out secrets, send account emails, or end the session
	// are never callable by the assistant.
	for _, id := range []string{
		"POST /api/token/batch/keys", "POST /api/token/:id/key", "POST /api/channel/:id/key",
		"GET /api/reset_password", "POST /api/verify", "DELETE /api/user/self", "GET /api/user/token",
	} {
		_, found := assistantops.Find(id)
		assert.False(t, found, id)
	}

	// The admin-guarded activity revoke is not offered to the user profile,
	// while the user's own activity routes are.
	activityRevoke, ok := assistantops.Find("POST /api/activity/grants/:grant_id/revoke")
	require.True(t, ok)
	assert.False(t, assistantops.Visible(activityRevoke, "user", common.RoleCommonUser))
	activityRedeem, ok := assistantops.Find("POST /api/user/activity/redeem")
	require.True(t, ok)
	assert.True(t, assistantops.Visible(activityRedeem, "user", common.RoleCommonUser))

	// Only operations that address one user per call can be batched.
	for _, op := range operations {
		assert.Equal(t, op.BatchTarget != "", op.Batchable, op.ID)
	}
	optionUpdate, ok := assistantops.Find("PUT /api/option/")
	require.True(t, ok)
	assert.False(t, optionUpdate.Batchable)

	bonusList, ok := assistantops.Find("GET /api/user/:id/bonus")
	require.True(t, ok)
	assert.Equal(t, assistantops.RiskRead, bonusList.Risk)
}

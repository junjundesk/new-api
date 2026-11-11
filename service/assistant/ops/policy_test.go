package ops

import (
	"context"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvokeKeepsIntegerParametersAndCallerIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/user/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"id":     c.Param("id"),
			"start":  c.Query("start_timestamp"),
			"ratio":  c.Query("ratio"),
			"groups": c.QueryArray("group"),
			"ip":     c.ClientIP(),
			"ua":     c.Request.UserAgent(),
		}})
	})
	Initialize(engine)
	t.Cleanup(ResetForTest)
	op, ok := Find("GET /api/user/:id")
	require.True(t, ok)

	// JSON numbers decode to float64; ids and unix timestamps must not turn
	// into scientific notation on the way to the endpoint.
	result := Invoke(context.Background(),
		Actor{UserID: 1, Role: common.RoleAdminUser, IP: "203.0.113.9", UserAgent: "Mozilla/Test"},
		op,
		map[string]any{"id": float64(1234567)},
		map[string]any{"start_timestamp": float64(1730000000), "ratio": 1.5, "group": []any{"a", "b"}},
		nil,
	)
	require.True(t, result.Success, result.Message)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1234567", data["id"])
	assert.Equal(t, "1730000000", data["start"])
	assert.Equal(t, "1.5", data["ratio"])
	assert.Equal(t, []any{"a", "b"}, data["groups"])
	assert.Equal(t, "203.0.113.9", data["ip"])
	assert.Equal(t, "Mozilla/Test", data["ua"])
}

func TestInvokeReportsFailureEnvelopeAndOversizedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/channel/failing", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": ""})
	})
	engine.GET("/api/channel/huge", func(c *gin.Context) {
		c.String(http.StatusOK, `{"success":true,"data":"`+string(make([]byte, maxResponseBytes))+`"}`)
	})
	Initialize(engine)
	t.Cleanup(ResetForTest)

	failing, ok := Find("GET /api/channel/failing")
	require.True(t, ok)
	result := Invoke(context.Background(), Actor{UserID: 1}, failing, nil, nil, nil)
	assert.False(t, result.Success, "success:false with an empty message is still a failure")

	huge, ok := Find("GET /api/channel/huge")
	require.True(t, ok)
	result = Invoke(context.Background(), Actor{UserID: 1}, huge, nil, nil, nil)
	assert.True(t, result.Truncated)
	assert.Empty(t, result.Raw, "a cut-off document must not be forwarded as raw text")
	assert.Contains(t, result.Message, "page_size")
}

func TestClassifyRouteRisk(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   Risk
	}{
		{"GET", "/api/user/", RiskRead},
		{"GET", "/api/log/", RiskRead},
		// GET routes that call upstream providers or rewrite state.
		{"GET", "/api/channel/test", RiskWrite},
		{"GET", "/api/channel/test/:id", RiskWrite},
		{"GET", "/api/channel/update_balance", RiskWrite},
		// POST routes that only preview.
		{"POST", "/api/user/activity/verify", RiskRead},
		{"POST", "/api/deployments/price-estimation", RiskRead},
		{"POST", "/api/user/:id/bonus", RiskWrite},
		{"POST", "/api/channel/:id/status", RiskWrite},
		{"POST", "/api/channel/status/batch", RiskWrite},
		{"POST", "/api/channel/tag/disabled", RiskWrite},
		{"PUT", "/api/user/", RiskWrite},
		// Every DELETE, plus writes that delete or are site-wide.
		{"DELETE", "/api/user/:id", RiskDestructive},
		{"DELETE", "/api/channel/:id", RiskDestructive},
		{"DELETE", "/api/token/:id", RiskDestructive},
		{"POST", "/api/channel/batch", RiskDestructive},
		{"POST", "/api/token/batch", RiskDestructive},
		{"POST", "/api/user/manage", RiskDestructive},
		{"POST", "/api/user/bonus/grant_all", RiskDestructive},
		{"PUT", "/api/option/", RiskDestructive},
		{"POST", "/api/user/:id/bonus/:grant_id/revoke", RiskDestructive},
		{"POST", "/api/system-task/log-cleanup", RiskDestructive},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, classifyRoute(tt.method, tt.path).Risk)
		})
	}
}

func TestClassifyRouteScopeAndExclusions(t *testing.T) {
	scopes := []struct {
		method string
		path   string
		want   string
	}{
		{"GET", "/api/user/self", ScopeUser},
		{"GET", "/api/task/self", ScopeUser},
		{"GET", "/api/mj/self", ScopeUser},
		{"GET", "/api/data/flow/self", ScopeUser},
		{"POST", "/api/user/activity/redeem", ScopeUser},
		{"POST", "/api/user/aff_transfer", ScopeUser},
		{"GET", "/api/task/", ScopeAdmin},
		{"GET", "/api/user/topup", ScopeAdmin},
		// Admin-guarded even though the path looks like a user route.
		{"POST", "/api/activity/grants/:grant_id/revoke", ScopeAdmin},
		{"GET", "/api/activity/", ScopeAdmin},
		{"PUT", "/api/option/", ScopeRoot},
	}
	for _, tt := range scopes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			op := classifyRoute(tt.method, tt.path)
			require.Empty(t, op.ExcludedReason)
			assert.Equal(t, tt.want, op.Scope)
		})
	}

	excluded := []struct {
		method string
		path   string
	}{
		{"GET", "/api/reset_password"},    // sends a password reset email
		{"POST", "/api/verify"},           // interactive secure verification
		{"POST", "/api/token/batch/keys"}, // returns full API keys
		{"POST", "/api/token/:id/key"},    // returns a full API key
		{"POST", "/api/channel/:id/key"},  // returns a channel key
		{"DELETE", "/api/user/self"},      // deletes the signed-in account
		{"GET", "/api/log/search"},        // deprecated endpoint
		{"POST", "/api/user/2fa/disable"},
	}
	for _, tt := range excluded {
		t.Run("excluded "+tt.method+" "+tt.path, func(t *testing.T) {
			assert.NotEmpty(t, classifyRoute(tt.method, tt.path).ExcludedReason)
		})
	}
}

func TestBatchTargetOnlyForSingleUserOperations(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{"POST", "/api/user/:id/bonus", "id"},
		{"GET", "/api/user/:id", "id"},
		{"POST", "/api/subscription/admin/users/:id/subscriptions", "id"},
		{"POST", "/api/user/manage", "body:id"},
		{"PUT", "/api/user/", "body:id"},
		{"POST", "/api/user/:id/bonus/:grant_id/revoke", ""},
		{"POST", "/api/channel/", ""},
		{"PUT", "/api/option/", ""},
		{"DELETE", "/api/token/:id", ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			op := classifyRoute(tt.method, tt.path)
			assert.Equal(t, tt.want, op.BatchTarget)
			assert.Equal(t, tt.want != "", op.Batchable)
		})
	}
}

func TestSecretFieldRules(t *testing.T) {
	option := classifyRoute("PUT", "/api/option/")
	channel := classifyRoute("PUT", "/api/channel/")
	token := classifyRoute("GET", "/api/token/")

	// "key" names the option being changed; hiding it would make the
	// confirmation card unreadable.
	assert.False(t, option.IsSecretField("key"))
	assert.True(t, channel.IsSecretField("key"))
	assert.True(t, token.IsSecretField("key"))

	for _, secret := range []string{"password", "original_password", "access_token", "webhook_secret", "client_secret", "api_key", "private_key", "gotify_token", "token"} {
		assert.True(t, channel.IsSecretField(secret), secret)
	}
	for _, plain := range []string{"keyword", "multi_key_mode", "key_index", "is_multi_key", "masked_key", "token_name", "prompt_tokens", "name", "quota"} {
		assert.False(t, channel.IsSecretField(plain), plain)
	}

	assert.True(t, IsSecretOptionKey("SMTPToken"))
	assert.True(t, IsSecretOptionKey("GitHubClientSecret"))
	assert.True(t, IsSecretOptionKey("assistant_setting.admin_api_key"))
	assert.False(t, IsSecretOptionKey("QuotaPerUnit"))
	assert.False(t, IsSecretOptionKey("SMTPServer"))
}

func TestListPagesThroughVisibleOperations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) }
	for _, path := range []string{"/api/vendors/a", "/api/vendors/b", "/api/vendors/c", "/api/vendors/d", "/api/vendors/e"} {
		engine.GET(path, handler)
	}
	engine.PUT("/api/option/", handler)
	engine.POST("/api/user/manage", handler)
	Initialize(engine)
	t.Cleanup(ResetForTest)

	first, total := List("admin", common.RoleAdminUser, "vendors", "", 0, 2)
	require.Len(t, first, 2)
	assert.Equal(t, 5, total)
	second, total := List("admin", common.RoleAdminUser, "vendors", "", 2, 2)
	require.Len(t, second, 2)
	assert.Equal(t, 5, total)
	last, _ := List("admin", common.RoleAdminUser, "vendors", "", 4, 2)
	require.Len(t, last, 1)

	seen := map[string]bool{}
	for _, page := range [][]Operation{first, second, last} {
		for _, op := range page {
			assert.False(t, seen[op.ID], "operation repeated across pages: %s", op.ID)
			seen[op.ID] = true
		}
	}
	assert.Len(t, seen, 5)

	// Root-only operations are neither listed nor counted for an admin.
	_, adminTotal := List("admin", common.RoleAdminUser, "option", "", 0, 10)
	assert.Equal(t, 0, adminTotal)
	assert.Equal(t, map[string]int{"vendors": 5, "user": 1}, Domains("admin", common.RoleAdminUser))

	// The search also covers documented parameter names: "action" appears
	// only as a body field of the manage operation.
	matches, _ := List("admin", common.RoleAdminUser, "", "action", 0, 10)
	require.Len(t, matches, 1)
	assert.Equal(t, "POST /api/user/manage", matches[0].ID)
}

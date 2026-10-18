package assistant

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminDBToolsOnlyExposeSafeColumns(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	executor := NewRuntimeToolExecutor(Config{
		AdminScope: true, AdminDBTools: true, ActorID: 1, ActorRole: common.RoleRootUser, Profile: "admin",
	}, nil)

	schema := executor.Execute(context.Background(), 1, ToolCall{Name: "admin_db_schema", Arguments: "{}"}, nil)
	require.True(t, schema.OK, schema.Error)
	payload, err := common.Marshal(schema.Data)
	require.NoError(t, err)
	assert.Contains(t, string(payload), "users")
	assert.NotContains(t, string(payload), "password")
	assert.NotContains(t, string(payload), "access_token")

	query := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "admin_db_query",
		Arguments: "{\"table\":\"users\",\"columns\":[\"id\",\"username\",\"role\"],\"filters\":[{\"column\":\"role\",\"op\":\"=\",\"value\":1}]}",
	}, nil)
	require.True(t, query.OK, query.Error)
	data, ok := query.Data.(map[string]any)
	require.True(t, ok)
	rows, ok := data["rows"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, "alice", rows[0]["username"])

	denied := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "admin_db_query",
		Arguments: "{\"table\":\"tokens\",\"columns\":[\"key\"]}",
	}, nil)
	assert.False(t, denied.OK)

	denied = executor.Execute(context.Background(), 1, ToolCall{
		Name:      "admin_db_query",
		Arguments: "{\"table\":\"sqlite_master\"}",
	}, nil)
	assert.False(t, denied.OK)

	userExecutor := NewRuntimeToolExecutor(Config{AdminScope: false, ActorID: 1, ActorRole: common.RoleCommonUser, Profile: "user"}, nil)
	denied = userExecutor.Execute(context.Background(), 1, ToolCall{Name: "admin_db_schema", Arguments: "{}"}, nil)
	assert.False(t, denied.OK)
}

func TestGenericExecuteOperationRunsCatalogRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/api/token/probe", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"ok": true}})
	})
	assistantops.Initialize(engine)
	t.Cleanup(assistantops.ResetForTest)
	executor := NewRuntimeToolExecutor(Config{ActorID: 1, ActorRole: common.RoleCommonUser, Profile: "user"}, nil)
	result := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "execute_operation",
		Arguments: "{\"operation_id\":\"POST /api/token/probe\",\"body\":{}}",
	}, nil)
	require.True(t, result.OK, result.Error)

	list := executor.Execute(context.Background(), 1, ToolCall{Name: "list_operations", Arguments: "{\"query\":\"token/probe\"}"}, nil)
	require.True(t, list.OK, list.Error)
	listData, ok := list.Data.(map[string]any)
	require.True(t, ok)
	operations, ok := listData["operations"].([]map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, operations)
	for _, operation := range operations {
		assert.NotContains(t, operation, "batchable")
	}
}

func TestUserToolSetHidesAdministratorTools(t *testing.T) {
	userExecutor := NewRuntimeToolExecutor(Config{ActorID: 1, ActorRole: common.RoleCommonUser, Profile: "user"}, nil)
	userTools := map[string]struct{}{}
	for _, definition := range userExecutor.Definitions() {
		name, ok := definition["name"].(string)
		require.True(t, ok)
		userTools[name] = struct{}{}
	}
	for _, hidden := range []string{"list_users", "batch_operation", "admin_db_schema", "admin_db_query"} {
		assert.NotContains(t, userTools, hidden)
		assert.Equal(t, "unsupported_tool", userExecutor.DisplayToolName(hidden))
	}
	for _, visible := range []string{"list_operations", "get_operation", "execute_operation", "create_api_key", "list_api_keys"} {
		assert.Contains(t, userTools, visible)
		assert.Equal(t, visible, userExecutor.DisplayToolName(visible))
	}
	rejected := userExecutor.Execute(context.Background(), 1, ToolCall{Name: "batch_operation", Arguments: "{}"}, nil)
	require.False(t, rejected.OK)

	adminExecutor := NewRuntimeToolExecutor(Config{AdminScope: true, ActorID: 2, ActorRole: common.RoleRootUser, Profile: "admin", AdminDBTools: true}, nil)
	adminTools := map[string]struct{}{}
	for _, definition := range adminExecutor.Definitions() {
		name, ok := definition["name"].(string)
		require.True(t, ok)
		adminTools[name] = struct{}{}
	}
	for _, visible := range []string{"list_users", "batch_operation", "admin_db_schema", "admin_db_query"} {
		assert.Contains(t, adminTools, visible)
	}
}

func TestBatchOperationRunsPerTargetAndEnforcesLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/api/user/:id/probe", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"id": c.Param("id")}})
	})
	assistantops.Initialize(engine)
	t.Cleanup(assistantops.ResetForTest)
	executor := NewRuntimeToolExecutor(Config{AdminScope: true, ActorID: 1, ActorRole: common.RoleAdminUser, Profile: "admin"}, nil)
	result := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "batch_operation",
		Arguments: "{\"operation_id\":\"POST /api/user/:id/probe\",\"user_ids\":[1,2,3]}",
	}, nil)
	require.True(t, result.OK, result.Error)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 3, data["total"])
	assert.EqualValues(t, 3, data["succeeded"])

	tooMany := make([]int, 101)
	for i := range tooMany {
		tooMany[i] = i + 1
	}
	payload, err := common.Marshal(map[string]any{
		"operation_id": "POST /api/user/:id/probe",
		"user_ids":     tooMany,
	})
	require.NoError(t, err)
	result = executor.Execute(context.Background(), 1, ToolCall{Name: "batch_operation", Arguments: string(payload)}, nil)
	assert.False(t, result.OK)
	assert.Contains(t, result.Error, "limit is 100")
}

func TestGenericExecuteRejectsOperationsOutsideProfileScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.PUT("/api/option/probe", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	})
	assistantops.Initialize(engine)
	t.Cleanup(assistantops.ResetForTest)

	userExecutor := NewRuntimeToolExecutor(Config{ActorID: 1, ActorRole: common.RoleCommonUser, Profile: "user"}, nil)
	result := userExecutor.Execute(context.Background(), 1, ToolCall{
		Name:      "execute_operation",
		Arguments: "{\"operation_id\":\"PUT /api/option/probe\",\"body\":{}}",
	}, nil)
	require.False(t, result.OK)
	assert.Contains(t, result.Error, "not available")

	adminExecutor := NewRuntimeToolExecutor(Config{AdminScope: true, ActorID: 2, ActorRole: common.RoleAdminUser, Profile: "admin"}, nil)
	result = adminExecutor.Execute(context.Background(), 2, ToolCall{
		Name:      "execute_operation",
		Arguments: "{\"operation_id\":\"PUT /api/option/probe\",\"body\":{}}",
	}, nil)
	require.False(t, result.OK)
	assert.Contains(t, result.Error, "not available")
}

func TestUserOperationRejectsIdentityOverrides(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.PUT("/api/user/self", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	})
	assistantops.Initialize(engine)
	t.Cleanup(assistantops.ResetForTest)
	executor := NewRuntimeToolExecutor(Config{ActorID: 1, ActorRole: common.RoleCommonUser, Profile: "user"}, nil)
	result := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "execute_operation",
		Arguments: "{\"operation_id\":\"PUT /api/user/self\",\"body\":{\"password\":\"new\",\"original_password\":\"old\"}}",
	}, nil)
	require.False(t, result.OK)
	assert.Contains(t, result.Error, "not allowed")
}

func TestBatchOperationSupportsBodyTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/api/user/manage", func(c *gin.Context) {
		var body map[string]any
		_ = c.ShouldBindJSON(&body)
		c.JSON(200, gin.H{"success": true, "data": body})
	})
	assistantops.Initialize(engine)
	t.Cleanup(assistantops.ResetForTest)
	executor := NewRuntimeToolExecutor(Config{AdminScope: true, ActorID: 1, ActorRole: common.RoleAdminUser, Profile: "admin"}, nil)
	result := executor.Execute(context.Background(), 1, ToolCall{
		Name:      "batch_operation",
		Arguments: "{\"operation_id\":\"POST /api/user/manage\",\"user_ids\":[1,2],\"params\":{\"body\":{\"action\":\"enable\"}}}",
	}, nil)
	require.True(t, result.OK, result.Error)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 2, data["succeeded"])
}

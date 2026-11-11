package assistant

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rootExecutor() *DefaultToolExecutor {
	return NewRuntimeToolExecutor(Config{
		AdminScope: true, AdminDBTools: true, ActorID: 1, ActorRole: common.RoleRootUser, Profile: "admin",
	}, nil)
}

func runTool(t *testing.T, executor *DefaultToolExecutor, name, arguments string) ToolResult {
	t.Helper()
	return executor.Execute(context.Background(), 1, ToolCall{Name: name, Arguments: arguments}, nil)
}

func dbRows(t *testing.T, result ToolResult) []map[string]any {
	t.Helper()
	require.True(t, result.OK, result.Error)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	rows, ok := data["rows"].([]map[string]any)
	require.True(t, ok)
	return rows
}

// confirmingExecutor answers every confirmation with the given decision and
// records the confirmation events the approver would have seen.
func confirmingExecutor(cfg Config, approve bool) (*DefaultToolExecutor, *[]Event) {
	cfg.ConfirmWrites = true
	executor := NewRuntimeToolExecutor(cfg, nil)
	executor.confirmation = NewConfirmationBroker()
	seen := &[]Event{}
	executor.emit = func(event Event) error {
		if event.Type == "confirmation_required" {
			*seen = append(*seen, event)
			go func() {
				_ = executor.confirmation.Resolve(cfg.ActorID, cfg.Profile, event.ConfirmationID, approve)
			}()
		}
		return nil
	}
	return executor, seen
}

func TestAdminDBQueryHandlesReservedColumnsAndSoftDelete(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	seedAssistantUser(t, db, 3, "carol")
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 2).Update("group", "vip").Error)
	require.NoError(t, db.Delete(&model.User{}, 3).Error)
	executor := rootExecutor()

	// "group" is a reserved word; the default column list includes it.
	rows := dbRows(t, runTool(t, executor, "admin_db_query", `{"table":"users","order_by":[{"column":"id"}]}`))
	require.Len(t, rows, 2, "soft-deleted users are hidden by default")
	assert.Equal(t, "default", rows[0]["group"])
	assert.Equal(t, "vip", rows[1]["group"])

	rows = dbRows(t, runTool(t, executor, "admin_db_query",
		`{"table":"users","columns":["id","group"],"filters":[{"column":"group","op":"=","value":"vip"}]}`))
	require.Len(t, rows, 1)
	assert.EqualValues(t, 2, rows[0]["id"])

	rows = dbRows(t, runTool(t, executor, "admin_db_query",
		`{"table":"users","aggregate":{"metrics":[{"func":"count","column":"*"}]}}`))
	require.Len(t, rows, 1)
	assert.EqualValues(t, 2, rows[0]["count_all"])

	rows = dbRows(t, runTool(t, executor, "admin_db_query",
		`{"table":"users","include_deleted":true,"aggregate":{"metrics":[{"func":"count","column":"*"}]}}`))
	assert.EqualValues(t, 3, rows[0]["count_all"])

	rows = dbRows(t, runTool(t, executor, "admin_db_query",
		`{"table":"users","include_deleted":true,"columns":["id"],"filters":[{"column":"deleted_at","op":"not_null"}]}`))
	require.Len(t, rows, 1)
	assert.EqualValues(t, 3, rows[0]["id"])
}

// Every allow-listed column must exist in the real schema, otherwise the
// default "all columns" query of that table fails.
func TestAdminDBSchemaMatchesModels(t *testing.T) {
	db := setupToolTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{}, &model.Redemption{}, &model.TopUp{},
		&model.SubscriptionPlan{}, &model.UserSubscription{},
	))
	executor := rootExecutor()

	schema := runTool(t, executor, "admin_db_schema", "{}")
	require.True(t, schema.OK, schema.Error)
	tables := schema.Data.(map[string]any)["tables"].([]map[string]any)
	require.Len(t, tables, len(adminDBTables))
	for _, table := range tables {
		name := table["table"].(string)
		result := runTool(t, executor, "admin_db_query", fmt.Sprintf(`{"table":%q,"limit":1}`, name))
		assert.True(t, result.OK, "%s: %s", name, result.Error)
		for _, column := range table["columns"].([]dbColumn) {
			lower := strings.ToLower(column.Name)
			assert.False(t, lower == "key" || strings.Contains(lower, "password") || strings.Contains(lower, "access_token"),
				"%s.%s must not be queryable", name, column.Name)
		}
	}
}

func TestAdminDBQueryOrdersAggregatesAndPages(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	seedAssistantUser(t, db, 3, "carol")
	seedAssistantLog(t, db, 1, "model-a", "key", "c", 100)
	seedAssistantLog(t, db, 2, "model-a", "key", "c", 300)
	seedAssistantLog(t, db, 2, "model-b", "key", "c", 50)
	seedAssistantLog(t, db, 3, "model-b", "key", "c", 200)
	executor := rootExecutor()

	// Top spenders: order by the metric alias.
	result := runTool(t, executor, "admin_db_query",
		`{"table":"logs","aggregate":{"group_by":["user_id"],"metrics":[{"func":"sum","column":"quota"},{"func":"count","column":"*"}]},"order_by":[{"column":"sum_quota","direction":"desc"}],"limit":2}`)
	rows := dbRows(t, result)
	require.Len(t, rows, 2)
	assert.EqualValues(t, 2, rows[0]["user_id"])
	assert.EqualValues(t, 350, rows[0]["sum_quota"])
	assert.EqualValues(t, 2, rows[0]["count_all"])
	assert.EqualValues(t, 3, rows[1]["user_id"])
	data := result.Data.(map[string]any)
	assert.Equal(t, true, data["has_more"])
	assert.Equal(t, 2, data["next_offset"])

	result = runTool(t, executor, "admin_db_query",
		`{"table":"logs","aggregate":{"group_by":["user_id"],"metrics":[{"func":"sum","column":"quota"}]},"order_by":[{"column":"sum_quota","direction":"desc"}],"limit":2,"offset":2}`)
	rows = dbRows(t, result)
	require.Len(t, rows, 1)
	assert.EqualValues(t, 1, rows[0]["user_id"])
	assert.Equal(t, false, result.Data.(map[string]any)["has_more"])

	// Grouping by a reserved-word column.
	rows = dbRows(t, runTool(t, executor, "admin_db_query",
		`{"table":"logs","aggregate":{"group_by":["group","model_name"],"metrics":[{"func":"sum","column":"quota"}]},"order_by":[{"column":"model_name"}]}`))
	require.Len(t, rows, 2)
	assert.Equal(t, "default", rows[0]["group"])
	assert.EqualValues(t, 400, rows[0]["sum_quota"])
	assert.EqualValues(t, 250, rows[1]["sum_quota"])

	rejected := []string{
		// A raw column is not sortable once the query is aggregated.
		`{"table":"logs","aggregate":{"group_by":["user_id"],"metrics":[{"func":"sum","column":"quota"}]},"order_by":[{"column":"created_at"}]}`,
		`{"table":"logs","aggregate":{"metrics":[{"func":"sum","column":"model_name"}]}}`,
		`{"table":"logs","aggregate":{"metrics":[{"func":"sum","column":"*"}]}}`,
		`{"table":"logs","aggregate":{"metrics":[{"func":"count","column":"content"}]}}`,
		`{"table":"logs","aggregate":{"metrics":[]}}`,
		`{"table":"logs","filters":[{"column":"quota","op":"like","value":"1"}]}`,
		`{"table":"logs","filters":[{"column":"quota","op":"=","value":null}]}`,
		`{"table":"logs","filters":[{"column":"user_id","op":"in","value":[]}]}`,
		`{"table":"logs","filters":[{"column":"content","op":"=","value":"c"}]}`,
		`{"table":"logs","limit":101}`,
		`{"table":"logs","offset":-1}`,
	}
	for _, arguments := range rejected {
		assert.False(t, runTool(t, executor, "admin_db_query", arguments).OK, arguments)
	}
}

func TestListToolsPageWithOffset(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	for index := 0; index < 5; index++ {
		seedAssistantLog(t, db, 1, "model-a", "key", fmt.Sprintf("log-%d", index), index+1)
		require.NoError(t, db.Create(&model.Token{
			UserId: 1, Key: fmt.Sprintf("sk-page-%d", index), Name: fmt.Sprintf("key-%d", index),
			UnlimitedQuota: true, ExpiredTime: -1, Status: common.TokenStatusEnabled,
		}).Error)
	}
	executor := NewDefaultToolExecutor(true)

	for _, tool := range []string{"list_usage_logs", "list_api_keys"} {
		seen := map[string]bool{}
		offset := 0
		for page := 0; page < 3; page++ {
			result := runTool(t, executor, tool, fmt.Sprintf(`{"user_id":1,"limit":2,"offset":%d}`, offset))
			require.True(t, result.OK, result.Error)
			data := result.Data.(map[string]any)
			assert.EqualValues(t, 5, data["total"], tool)
			for _, item := range data["items"].([]map[string]any) {
				identity := fmt.Sprint(item["content"], item["name"])
				assert.False(t, seen[identity], "%s repeated %s across pages", tool, identity)
				seen[identity] = true
			}
			next, more := data["next_offset"]
			assert.Equal(t, more, data["truncated"], tool)
			if !more {
				break
			}
			offset = next.(int)
		}
		assert.Len(t, seen, 5, tool)
	}

	assert.False(t, runTool(t, executor, "list_usage_logs", `{"offset":-1}`).OK)
}

func TestListUsersMarksDeletedAndFilters(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	seedAssistantUser(t, db, 3, "carol")
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 2).Update("status", common.UserStatusDisabled).Error)
	require.NoError(t, db.Delete(&model.User{}, 3).Error)
	executor := NewDefaultToolExecutor(true)

	all := runTool(t, executor, "list_users", `{}`)
	require.True(t, all.OK, all.Error)
	deleted := map[string]any{}
	for _, item := range all.Data.(map[string]any)["items"].([]map[string]any) {
		deleted[item["username"].(string)] = item["deleted"]
	}
	assert.Equal(t, map[string]any{"alice": false, "bob": false, "carol": true}, deleted)

	disabled := runTool(t, executor, "list_users", `{"status":2}`)
	require.True(t, disabled.OK, disabled.Error)
	items := disabled.Data.(map[string]any)["items"].([]map[string]any)
	require.Len(t, items, 1)
	assert.Equal(t, "bob", items[0]["username"])

	// A deleted user cannot be targeted; the error says why.
	summary := runTool(t, executor, "get_account_summary", `{"user_id":3}`)
	require.False(t, summary.OK)
	assert.Contains(t, summary.Error, "deleted")
}

func TestUsageLogContentIsCutOnRuneBoundary(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantLog(t, db, 1, "model-a", "key", "a"+strings.Repeat("中", 600), 1)

	result := runTool(t, NewDefaultToolExecutor(true), "list_usage_logs", `{"user_id":1}`)
	require.True(t, result.OK, result.Error)
	content := result.Data.(map[string]any)["items"].([]map[string]any)[0]["content"].(string)
	assert.True(t, utf8.ValidString(content))
	assert.Equal(t, maxLogContentRunes+1, utf8.RuneCountInString(content))
	assert.True(t, strings.HasSuffix(content, "…"))
}

func TestAdminUsageWindowIsLongerThanUserWindow(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	assert.True(t, runTool(t, NewDefaultToolExecutor(true), "get_usage_summary", `{"days":90}`).OK)
	assert.False(t, runTool(t, NewDefaultToolExecutor(true), "get_usage_summary", `{"days":91}`).OK)
	assert.False(t, runTool(t, NewDefaultToolExecutor(false), "get_usage_summary", `{"days":32}`).OK)
}

func TestKeyToolsValidateBeforeConfirmingAndShowTheTarget(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	require.NoError(t, db.Create(&model.Token{UserId: 2, Key: "sk-bob-secret-value", Name: "bob-key", UnlimitedQuota: true, ExpiredTime: -1, Status: common.TokenStatusEnabled}).Error)
	bobToken, err := model.GetTokenByKey("sk-bob-secret-value", true)
	require.NoError(t, err)

	executor, seen := confirmingExecutor(Config{AdminScope: true, ActorID: 1, ActorRole: common.RoleRootUser, Profile: "admin"}, false)

	// Invalid requests fail without bothering the approver.
	for _, call := range []ToolCall{
		{Name: "set_api_key_status", Arguments: `{"id":-5,"enabled":false}`},
		{Name: "set_api_key_status", Arguments: `{"id":1}`},
		{Name: "set_api_key_status", Arguments: `{"id":99999,"enabled":false}`},
		{Name: "create_api_key", Arguments: `{"user_id":2,"name":""}`},
		{Name: "create_api_key", Arguments: `{"user_id":2,"name":"x","expires_in_days":0}`},
	} {
		result := executor.Execute(context.Background(), 1, call, nil)
		assert.False(t, result.OK, call.Arguments)
	}
	assert.Empty(t, *seen, "no confirmation may be requested for an invalid call")

	denied := executor.Execute(context.Background(), 1, ToolCall{
		Name: "set_api_key_status", Arguments: fmt.Sprintf(`{"id":%d,"enabled":false}`, bobToken.Id),
	}, nil)
	require.False(t, denied.OK)
	require.Len(t, *seen, 1)
	params, ok := (*seen)[0].Params.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bob-key", params["name"])
	assert.Equal(t, "bob", params["username"])
	assert.Equal(t, false, params["enabled"])
	assert.Equal(t, "Disable an API key", (*seen)[0].Summary)
	payload, err := common.Marshal((*seen)[0])
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "sk-bob-secret-value")

	unchanged, err := model.GetTokenById(bobToken.Id)
	require.NoError(t, err)
	assert.Equal(t, common.TokenStatusEnabled, unchanged.Status, "a denied confirmation must not change the key")

	denied = executor.Execute(context.Background(), 1, ToolCall{Name: "create_api_key", Arguments: `{"user_id":2,"name":"new-key"}`}, nil)
	require.False(t, denied.OK)
	require.Len(t, *seen, 2)
	params = (*seen)[1].Params.(map[string]any)
	assert.Equal(t, "new-key", params["name"])
	assert.Equal(t, "bob", params["username"])
	assert.Equal(t, "never", params["expires"])
	count, err := model.CountUserTokens(2)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count, "a denied confirmation must not create a key")
}

// operationProbe registers catalog routes that echo what they received.
func operationProbe(t *testing.T) *[]string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	hits := &[]string{}
	engine.POST("/api/user/:id/bonus", func(c *gin.Context) {
		var body map[string]any
		_ = c.ShouldBindJSON(&body)
		*hits = append(*hits, fmt.Sprintf("%s:%v", c.Param("id"), body["amount"]))
		c.JSON(200, gin.H{"success": true})
	})
	engine.GET("/api/user/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"id": c.Param("id"), "access_token": "tok-secret"}})
	})
	engine.GET("/api/option/", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "data": []gin.H{
			{"key": "QuotaPerUnit", "value": "500000"},
			{"key": "SMTPToken", "value": "smtp-secret"},
		}})
	})
	engine.PUT("/api/option/", func(c *gin.Context) {
		*hits = append(*hits, "option")
		c.JSON(200, gin.H{"success": true})
	})
	engine.GET("/api/channel/", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"items": []gin.H{{"id": 1, "name": "ch", "key": "sk-upstream", "channel_info": gin.H{"is_multi_key": true, "multi_key_mode": "random"}}}}})
	})
	engine.PUT("/api/channel/", func(c *gin.Context) { c.JSON(200, gin.H{"success": true}) })
	engine.POST("/api/redemption/", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "data": []string{"code-one", "code-two"}})
	})
	assistantops.Initialize(engine)
	t.Cleanup(assistantops.ResetForTest)
	return hits
}

func TestExecuteOperationRedactsSecretsButKeepsOptionNames(t *testing.T) {
	setupToolTestDB(t)
	operationProbe(t)
	executor := rootExecutor()

	options := runTool(t, executor, "execute_operation", `{"operation_id":"GET /api/option/"}`)
	require.True(t, options.OK, options.Error)
	payload, err := common.Marshal(options.Data)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"key":"QuotaPerUnit"`)
	assert.Contains(t, string(payload), `"value":"500000"`)
	assert.Contains(t, string(payload), `"key":"SMTPToken"`)
	assert.NotContains(t, string(payload), "smtp-secret")

	channels := runTool(t, executor, "execute_operation", `{"operation_id":"GET /api/channel/"}`)
	require.True(t, channels.OK, channels.Error)
	payload, err = common.Marshal(channels.Data)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "sk-upstream")
	assert.Contains(t, string(payload), `"multi_key_mode":"random"`)
	assert.Contains(t, string(payload), `"is_multi_key":true`)

	user := runTool(t, executor, "execute_operation", `{"operation_id":"GET /api/user/:id","path_params":{"id":7}}`)
	require.True(t, user.OK, user.Error)
	payload, err = common.Marshal(user.Data)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "tok-secret")

	codes := runTool(t, executor, "execute_operation", `{"operation_id":"POST /api/redemption/","body":{"name":"n","count":2,"quota":1}}`)
	require.True(t, codes.OK, codes.Error)
	payload, err = common.Marshal(codes.Data)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "code-one")
	assert.Contains(t, string(payload), "Redemption Codes page")
}

func TestExecuteOperationConfirmationShowsWhatChanges(t *testing.T) {
	setupToolTestDB(t)
	hits := operationProbe(t)
	executor, seen := confirmingExecutor(Config{AdminScope: true, ActorID: 1, ActorRole: common.RoleRootUser, Profile: "admin"}, false)
	ctx := context.Background()

	denied := executor.Execute(ctx, 1, ToolCall{Name: "execute_operation",
		Arguments: `{"operation_id":"PUT /api/option/","body":{"key":"QuotaPerUnit","value":"1"},"reason":"requested by ops"}`}, nil)
	require.False(t, denied.OK)
	require.Len(t, *seen, 1)
	event := (*seen)[0]
	assert.Equal(t, "destructive", event.Risk)
	params := event.Params.(map[string]any)
	assert.Equal(t, map[string]any{"key": "QuotaPerUnit", "value": "1"}, params["body"])
	assert.Equal(t, "requested by ops", params["reason"])

	// A credential option still names the option but hides the value.
	executor.Execute(ctx, 1, ToolCall{Name: "execute_operation",
		Arguments: `{"operation_id":"PUT /api/option/","body":{"key":"SMTPToken","value":"new-secret"}}`}, nil)
	require.Len(t, *seen, 2)
	assert.Equal(t, map[string]any{"key": "SMTPToken", "value": redactedPlaceholder}, (*seen)[1].Params.(map[string]any)["body"])

	executor.Execute(ctx, 1, ToolCall{Name: "execute_operation",
		Arguments: `{"operation_id":"PUT /api/channel/","body":{"id":3,"name":"renamed","key":"sk-new"}}`}, nil)
	require.Len(t, *seen, 3)
	assert.Equal(t, map[string]any{"id": float64(3), "name": "renamed", "key": redactedPlaceholder}, (*seen)[2].Params.(map[string]any)["body"])

	// Malformed calls fail before anyone is asked to approve them.
	missing := executor.Execute(ctx, 1, ToolCall{Name: "execute_operation",
		Arguments: `{"operation_id":"POST /api/user/:id/bonus","body":{"amount":1}}`}, nil)
	require.False(t, missing.OK)
	assert.Contains(t, missing.Error, "missing path parameter")
	assert.Len(t, *seen, 3)

	assert.Empty(t, *hits, "nothing may run while every confirmation is denied")
}

func TestTranscriptArgumentsUseOperationAwareRedaction(t *testing.T) {
	setupToolTestDB(t)
	operationProbe(t)
	executor := rootExecutor()

	option := executor.DisplayArguments(ToolCall{Name: "execute_operation",
		Arguments: `{"operation_id":"PUT /api/option/","body":{"key":"QuotaPerUnit","value":"1"}}`})
	assert.Equal(t, map[string]any{"key": "QuotaPerUnit", "value": "1"}, option.(map[string]any)["body"])

	channel := executor.DisplayArguments(ToolCall{Name: "execute_operation",
		Arguments: `{"operation_id":"PUT /api/channel/","body":{"id":1,"key":"sk-new"}}`})
	assert.Equal(t, redactedPlaceholder, channel.(map[string]any)["body"].(map[string]any)["key"])

	// Arguments that cannot be parsed are never echoed.
	broken := executor.DisplayArguments(ToolCall{Name: "execute_operation", Arguments: `{"body":{"key":"sk-leak"`})
	payload, err := common.Marshal(broken)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "sk-leak")
}

func TestBatchOperationTargetsEachUserOnce(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	seedAssistantUser(t, db, 3, "carol")
	require.NoError(t, db.Delete(&model.User{}, 3).Error)
	hits := operationProbe(t)
	executor := NewRuntimeToolExecutor(Config{AdminScope: true, ActorID: 1, ActorRole: common.RoleAdminUser, Profile: "admin"}, nil)

	// A repeated id must not grant twice.
	result := runTool(t, executor, "batch_operation",
		`{"operation_id":"POST /api/user/:id/bonus","user_ids":[2,2,1,2],"params":{"body":{"amount":100}}}`)
	require.True(t, result.OK, result.Error)
	assert.Equal(t, []string{"2:100", "1:100"}, *hits)
	assert.EqualValues(t, 2, result.Data.(map[string]any)["total"])

	// A filter never selects soft-deleted users.
	*hits = nil
	result = runTool(t, executor, "batch_operation",
		`{"operation_id":"POST /api/user/:id/bonus","filter":{"group":"default"},"params":{"body":{"amount":5}}}`)
	require.True(t, result.OK, result.Error)
	assert.Equal(t, []string{"1:5", "2:5"}, *hits)

	*hits = nil
	rejected := []string{
		// More than one way to choose targets is ambiguous.
		`{"operation_id":"POST /api/user/:id/bonus","user_ids":[1],"filter":{"group":"default"}}`,
		// An empty filter would mean "everyone".
		`{"operation_id":"POST /api/user/:id/bonus","filter":{"keyword":""}}`,
		`{"operation_id":"POST /api/user/:id/bonus","filter":{"unknown":"x"}}`,
		`{"operation_id":"POST /api/user/:id/bonus","filter":{"status":-1}}`,
		`{"operation_id":"POST /api/user/:id/bonus","items":[{"user_id":1},{"user_id":1}]}`,
		`{"operation_id":"POST /api/user/:id/bonus"}`,
		// Not a per-user operation.
		`{"operation_id":"PUT /api/option/","user_ids":[1]}`,
	}
	for _, arguments := range rejected {
		assert.False(t, runTool(t, executor, "batch_operation", arguments).OK, arguments)
	}
	assert.Empty(t, *hits)
}

func TestBatchOperationRefusesFiltersBeyondTheLimit(t *testing.T) {
	db := setupToolTestDB(t)
	for id := 1; id <= maxBatchTargets+1; id++ {
		seedAssistantUser(t, db, id, fmt.Sprintf("user-%d", id))
	}
	hits := operationProbe(t)
	executor := NewRuntimeToolExecutor(Config{AdminScope: true, ActorID: 1, ActorRole: common.RoleAdminUser, Profile: "admin"}, nil)

	result := runTool(t, executor, "batch_operation",
		`{"operation_id":"POST /api/user/:id/bonus","filter":{"group":"default"},"params":{"body":{"amount":5}}}`)
	require.False(t, result.OK)
	assert.Contains(t, result.Error, "101 users")
	assert.Empty(t, *hits, "a filter that matches too many users must not act on an arbitrary subset")
}

func TestBatchConfirmationListsUsersAndParameters(t *testing.T) {
	db := setupToolTestDB(t)
	seedAssistantUser(t, db, 1, "alice")
	seedAssistantUser(t, db, 2, "bob")
	hits := operationProbe(t)
	executor, seen := confirmingExecutor(Config{AdminScope: true, ActorID: 1, ActorRole: common.RoleAdminUser, Profile: "admin"}, false)

	denied := executor.Execute(context.Background(), 1, ToolCall{Name: "batch_operation",
		Arguments: `{"operation_id":"POST /api/user/:id/bonus","user_ids":[2,1],"params":{"body":{"amount":99999999,"remark":"promo"}}}`}, nil)
	require.False(t, denied.OK)
	require.Len(t, *seen, 1)
	params := (*seen)[0].Params.(map[string]any)
	assert.Equal(t, 2, params["user_count"])
	assert.Equal(t, []int{2, 1}, params["user_ids"])
	assert.Equal(t, map[string]any{"amount": float64(99999999), "remark": "promo"}, params["body"])
	assert.Empty(t, *hits)

	// Per-user parameters are listed per user.
	executor.Execute(context.Background(), 1, ToolCall{Name: "batch_operation",
		Arguments: `{"operation_id":"POST /api/user/:id/bonus","items":[{"user_id":1,"body":{"amount":10}},{"user_id":2,"body":{"amount":20}}]}`}, nil)
	require.Len(t, *seen, 2)
	items := (*seen)[1].Params.(map[string]any)["items"].([]any)
	require.Len(t, items, 2)
	assert.Equal(t, 1, items[0].(map[string]any)["user_id"])
	assert.Equal(t, map[string]any{"amount": float64(10)}, items[0].(map[string]any)["body"])
	assert.Equal(t, map[string]any{"amount": float64(20)}, items[1].(map[string]any)["body"])
	assert.Empty(t, *hits)
}

func TestListOperationsPagesAndGetOperationDocumentsParameters(t *testing.T) {
	setupToolTestDB(t)
	operationProbe(t)
	executor := rootExecutor()

	first := runTool(t, executor, "list_operations", `{"limit":3}`)
	require.True(t, first.OK, first.Error)
	firstData := first.Data.(map[string]any)
	total := firstData["total"].(int)
	require.Greater(t, total, 3)
	assert.Equal(t, 3, firstData["next_offset"])
	assert.NotEmpty(t, firstData["domains"])

	seen := map[string]bool{}
	for offset := 0; offset < total; offset += 3 {
		page := runTool(t, executor, "list_operations", fmt.Sprintf(`{"limit":3,"offset":%d}`, offset))
		require.True(t, page.OK, page.Error)
		for _, operation := range page.Data.(map[string]any)["operations"].([]map[string]any) {
			seen[operation["operation_id"].(string)] = true
		}
	}
	assert.Len(t, seen, total, "paging must reach every operation exactly once")
	assert.False(t, runTool(t, executor, "list_operations", `{"limit":101}`).OK)

	operation := runTool(t, executor, "get_operation", `{"operation_id":"PUT /api/option/"}`)
	require.True(t, operation.OK, operation.Error)
	data := operation.Data.(map[string]any)
	assert.EqualValues(t, "destructive", data["risk"])
	fields := data["body_fields"].([]assistantops.BodyField)
	require.Len(t, fields, 2)
	assert.Equal(t, "key", fields[0].Name)
	assert.Equal(t, false, data["batchable"])

	bonus := runTool(t, executor, "get_operation", `{"operation_id":"POST /api/user/:id/bonus"}`)
	require.True(t, bonus.OK, bonus.Error)
	assert.Equal(t, true, bonus.Data.(map[string]any)["batchable"])
	assert.Equal(t, "id", bonus.Data.(map[string]any)["batch_target"])
}

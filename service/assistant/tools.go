package assistant

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// DefaultToolExecutor is the production tool set. The user profile can only
// read and mutate the authenticated user's own data; the admin profile can
// target every user's usage, logs, and API keys.
type DefaultToolExecutor struct {
	adminScope     bool
	actorID        int
	actorUsername  string
	actorRole      int
	profile        string
	actorSessionID string
	authVersion    int64
	sessionVersion int64
	actorIP        string
	actorUserAgent string
	confirmWrites  bool
	adminDBTools   bool
	emit           EmitFunc
	confirmation   *ConfirmationBroker
}

func NewDefaultToolExecutor(adminScope bool) *DefaultToolExecutor {
	return &DefaultToolExecutor{adminScope: adminScope}
}

func (e *DefaultToolExecutor) Definitions() []map[string]any {
	groupByValues := []string{"model", "group", "token"}
	if e.adminScope {
		groupByValues = append(groupByValues, "user")
	}
	createKeyRequired := []string{"name"}
	if e.adminScope {
		createKeyRequired = append(createKeyRequired, "user_id")
	}

	accountSummaryProperties := map[string]any{}
	usageSummaryProperties := map[string]any{
		"days": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     31,
			"description": "Number of days to look back. Defaults to 7.",
		},
		"group_by": map[string]any{
			"type":        "string",
			"enum":        groupByValues,
			"description": "Dimension to group usage by. Defaults to model.",
		},
		"limit": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     20,
			"description": "Maximum number of rows. Defaults to 10.",
		},
	}
	usageLogProperties := map[string]any{
		"days": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     31,
			"description": "Number of days to look back. Defaults to 7.",
		},
		"model": map[string]any{
			"type":        "string",
			"maxLength":   128,
			"description": "Exact model name filter.",
		},
		"token_name": map[string]any{
			"type":        "string",
			"maxLength":   50,
			"description": "Exact API key name filter.",
		},
		"limit": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     20,
			"description": "Maximum number of logs. Defaults to 10.",
		},
	}
	listKeyProperties := map[string]any{
		"limit": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     50,
			"description": "Maximum number of keys. Defaults to 50.",
		},
	}
	createKeyProperties := map[string]any{
		"name": map[string]any{
			"type":        "string",
			"minLength":   1,
			"maxLength":   50,
			"description": "Human readable key name.",
		},
		"expires_in_days": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     3650,
			"description": "Days until expiry. Omit for never expires.",
		},
	}
	statusProperties := map[string]any{
		"id": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "API key id from list_api_keys or create_api_key.",
		},
		"enabled": map[string]any{
			"type":        "boolean",
			"description": "True to enable, false to disable.",
		},
	}

	accountDescription := "Get the current user's quota balance, lifetime usage, request count, account group, and today's usage."
	usageDescription := "Summarize consumption for a recent period, grouped by model, group, or API key name."
	logsDescription := "List recent consumption logs. Returns time, model, API key name, quota, token counts, and content."
	keysDescription := "List API keys with masked key values, status, quota, and today's usage."
	createDescription := "Create a new API key. The key inherits the owner's account group and has unlimited key quota. The full key is never returned; the UI can reveal it on explicit user action."
	statusDescription := "Enable or disable one API key. This is reversible and does not delete the key."

	if e.adminScope {
		accountSummaryProperties["user_id"] = map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Target user id. Omit for platform-wide totals.",
		}
		usageSummaryProperties["user_id"] = map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Target user id. Omit to aggregate all users.",
		}
		usageLogProperties["user_id"] = map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Target user id. Omit to list logs for all users.",
		}
		listKeyProperties["user_id"] = map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Target user id. Omit to list keys for all users.",
		}
		createKeyProperties["user_id"] = map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Owner user id for the new key.",
		}
		accountDescription = "Get an account summary. Pass user_id to inspect one user, or omit it for platform-wide totals (user count, quota, usage, requests)."
		usageDescription = "Summarize consumption for a recent period. Pass user_id to scope one user, or omit it for all users; group_by also supports user."
		logsDescription = "List consumption logs. Pass user_id for one user, or omit it for all users; each row includes the username."
		keysDescription = "List API keys with masked values. Pass user_id for one user, or omit it for all users; each row includes its owner."
		createDescription = "Create a new API key for user_id. The key inherits that user's group and has unlimited key quota. The full key is never returned."
	}

	definitions := []map[string]any{
		functionTool("get_account_summary", accountDescription, objectSchema(accountSummaryProperties, nil)),
		functionTool("get_usage_summary", usageDescription, objectSchema(usageSummaryProperties, nil)),
		functionTool("list_usage_logs", logsDescription, objectSchema(usageLogProperties, nil)),
		functionTool("list_api_keys", keysDescription, objectSchema(listKeyProperties, nil)),
		functionTool("create_api_key", createDescription, objectSchema(createKeyProperties, createKeyRequired)),
		functionTool("set_api_key_status", statusDescription, objectSchema(statusProperties, []string{"id", "enabled"})),
	}
	if e.adminScope {
		definitions = append(definitions, functionTool(
			"list_users",
			"Search users by username, display name, or email. Returns id, username, display name, role, status, group, quota, used quota, and request count.",
			objectSchema(map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"maxLength":   100,
					"description": "Optional search keyword.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     50,
					"description": "Maximum number of users. Defaults to 20.",
				},
			}, nil),
		))
	}
	definitions = append(definitions, e.genericDefinitions()...)
	return definitions
}

func (e *DefaultToolExecutor) Execute(ctx context.Context, userID int, call ToolCall, audit AuditFunc) ToolResult {
	switch call.Name {
	case "list_operations":
		return e.listOperations(call.Arguments)
	case "get_operation":
		return e.getOperation(call.Arguments)
	case "execute_operation":
		return e.executeOperation(ctx, call.Arguments, audit)
	case "batch_operation":
		return e.batchOperation(ctx, call.Arguments, audit)
	case "admin_db_schema":
		return e.adminDBSchema()
	case "admin_db_query":
		return e.adminDBQuery(ctx, call.Arguments)
	case "get_account_summary":
		targetUserID, err := e.resolveTargetUser(userID, call.Arguments, false)
		if err != nil {
			return toolError(err)
		}
		return e.getAccountSummary(targetUserID)
	case "get_usage_summary":
		targetUserID, err := e.resolveTargetUser(userID, call.Arguments, false)
		if err != nil {
			return toolError(err)
		}
		return e.getUsageSummary(targetUserID, call.Arguments)
	case "list_usage_logs":
		targetUserID, err := e.resolveTargetUser(userID, call.Arguments, false)
		if err != nil {
			return toolError(err)
		}
		return e.listUsageLogs(targetUserID, call.Arguments)
	case "list_api_keys":
		targetUserID, err := e.resolveTargetUser(userID, call.Arguments, false)
		if err != nil {
			return toolError(err)
		}
		return e.listAPIKeys(targetUserID, call.Arguments)
	case "create_api_key":
		targetUserID, err := e.resolveTargetUser(userID, call.Arguments, true)
		if err != nil {
			return toolError(err)
		}
		if err := e.askConfirmation(ctx, ConfirmationRequest{OperationID: "create_api_key", Risk: "write", Summary: "Create an API key"}); err != nil {
			return toolError(err)
		}
		return e.createAPIKey(targetUserID, call.Arguments, audit)
	case "set_api_key_status":
		if err := e.askConfirmation(ctx, ConfirmationRequest{OperationID: "set_api_key_status", Risk: "write", Summary: "Enable or disable an API key"}); err != nil {
			return toolError(err)
		}
		return e.setAPIKeyStatus(userID, call.Arguments, audit)
	case "list_users":
		if !e.adminScope {
			return toolErrorf("unknown tool: %s", call.Name)
		}
		return e.listUsers(call.Arguments)
	default:
		return toolErrorf("unknown tool: %s", call.Name)
	}
}

// resolveTargetUser returns actorID for user-scoped calls. For admins, a
// user_id argument selects a target user, while 0 means "all users" for tools
// that support aggregation. requireTarget is used by create_api_key.
func (e *DefaultToolExecutor) resolveTargetUser(actorID int, rawArguments string, requireTarget bool) (int, error) {
	if !e.adminScope {
		return actorID, nil
	}
	args := struct {
		UserID *int `json:"user_id"`
	}{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return 0, fmt.Errorf("invalid arguments: %v", err)
	}
	if args.UserID == nil || *args.UserID <= 0 {
		if requireTarget {
			return 0, fmt.Errorf("user_id is required")
		}
		return 0, nil
	}
	user, err := model.GetUserById(*args.UserID, false)
	if err != nil || user == nil {
		return 0, fmt.Errorf("user not found: %d", *args.UserID)
	}
	return user.Id, nil
}

func functionTool(name, description string, parameters map[string]any) map[string]any {
	return map[string]any{
		"type":        "function",
		"name":        name,
		"description": description,
		"parameters":  parameters,
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func toolErrorf(format string, args ...any) ToolResult {
	return ToolResult{OK: false, Error: strings.TrimSpace(fmt.Sprintf(format, args...))}
}

func toolError(err error) ToolResult {
	if err == nil {
		return ToolResult{OK: false, Error: "unknown tool error"}
	}
	return ToolResult{OK: false, Error: err.Error()}
}

func decodeArguments(input string, target any) error {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	return common.UnmarshalJsonStr(input, target)
}

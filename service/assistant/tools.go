package assistant

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// DefaultToolExecutor is the production tool set. It reads and mutates only
// data owned by the authenticated user.
type DefaultToolExecutor struct{}

func NewDefaultToolExecutor() *DefaultToolExecutor {
	return &DefaultToolExecutor{}
}

func (e *DefaultToolExecutor) Definitions() []map[string]any {
	definitions := []map[string]any{
		functionTool(
			"get_account_summary",
			"Get the current user's quota balance, lifetime usage, request count, account group, and today's usage.",
			objectSchema(map[string]any{}, nil),
		),
		functionTool(
			"get_usage_summary",
			"Summarize the current user's own consumption for a recent period, grouped by model, group, or API key name.",
			objectSchema(map[string]any{
				"days": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     31,
					"description": "Number of days to look back. Defaults to 7.",
				},
				"group_by": map[string]any{
					"type":        "string",
					"enum":        []string{"model", "group", "token"},
					"description": "Dimension to group usage by. Defaults to model.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     20,
					"description": "Maximum number of rows. Defaults to 10.",
				},
			}, nil),
		),
		functionTool(
			"list_usage_logs",
			"List the current user's own recent consumption logs. Returns time, model, API key name, quota, token counts, and content.",
			objectSchema(map[string]any{
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
			}, nil),
		),
		functionTool(
			"list_api_keys",
			"List the current user's own API keys with masked key values, status, quota, and today's usage.",
			objectSchema(map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     50,
					"description": "Maximum number of keys. Defaults to 50.",
				},
			}, nil),
		),
	}
	definitions = append(definitions,
		functionTool(
			"create_api_key",
			"Create a new API key for the current user. The key inherits the user's account group and has unlimited key quota. The full key is never returned; the UI can reveal it on explicit user action.",
			objectSchema(map[string]any{
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
			}, []string{"name"}),
		),
		functionTool(
			"set_api_key_status",
			"Enable or disable one of the current user's own API keys. This is reversible and does not delete the key.",
			objectSchema(map[string]any{
				"id": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "API key id from list_api_keys or create_api_key.",
				},
				"enabled": map[string]any{
					"type":        "boolean",
					"description": "True to enable, false to disable.",
				},
			}, []string{"id", "enabled"}),
		),
	)
	return definitions
}

func (e *DefaultToolExecutor) Execute(ctx context.Context, userID int, call ToolCall, audit AuditFunc) ToolResult {
	switch call.Name {
	case "get_account_summary":
		return e.getAccountSummary(userID)
	case "get_usage_summary":
		return e.getUsageSummary(userID, call.Arguments)
	case "list_usage_logs":
		return e.listUsageLogs(userID, call.Arguments)
	case "list_api_keys":
		return e.listAPIKeys(userID, call.Arguments)
	case "create_api_key":
		return e.createAPIKey(userID, call.Arguments, audit)
	case "set_api_key_status":
		return e.setAPIKeyStatus(userID, call.Arguments, audit)
	default:
		return toolErrorf("unknown tool: %s", call.Name)
	}
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

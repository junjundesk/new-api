package assistant

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"
)

func (e *DefaultToolExecutor) genericDefinitions() []map[string]any {
	defs := []map[string]any{
		functionTool(
			"list_operations",
			"List dashboard API operations available to the current assistant profile. Use this to discover the exact operation_id, method, path, risk, and path parameters before executing anything.",
			objectSchema(map[string]any{
				"scope":  map[string]any{"type": "string", "description": "Optional user/admin scope filter."},
				"domain": map[string]any{"type": "string", "description": "Optional domain such as user, token, channel, log."},
				"query":  map[string]any{"type": "string", "maxLength": 128, "description": "Optional text search."},
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "description": "Maximum operations to return. Defaults to 50."},
			}, nil),
		),
		functionTool(
			"get_operation",
			"Get the full description and parameter requirements of one operation_id returned by list_operations.",
			objectSchema(map[string]any{
				"operation_id": map[string]any{"type": "string", "description": "Operation id such as POST /api/token."},
			}, []string{"operation_id"}),
		),
		functionTool(
			"execute_operation",
			"Execute one dashboard API operation within the current user's or administrator's permissions. Write and destructive operations require user confirmation. Use path_params for :id style segments, query for query parameters, and body for the JSON request body.",
			objectSchema(map[string]any{
				"operation_id": map[string]any{"type": "string"},
				"path_params":  map[string]any{"type": "object", "additionalProperties": true},
				"query":        map[string]any{"type": "object", "additionalProperties": true},
				"body":         map[string]any{"type": "object", "additionalProperties": true},
				"reason":       map[string]any{"type": "string", "maxLength": 500},
			}, []string{"operation_id"}),
		),
		functionTool(
			"batch_operation",
			"Run one user-targeted operation for many users after one confirmation. Provide user_ids or a user filter, or explicit items. Maximum 100 targets. Per-target results are returned; there is no rollback.",
			objectSchema(map[string]any{
				"operation_id": map[string]any{"type": "string"},
				"user_ids":     map[string]any{"type": "array", "items": map[string]any{"type": "integer", "minimum": 1}},
				"filter": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{"type": "string", "maxLength": 100},
						"status":  map[string]any{"type": "integer"},
						"role":    map[string]any{"type": "integer"},
						"group":   map[string]any{"type": "string", "maxLength": 64},
					},
					"additionalProperties": false,
				},
				"items": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"user_id":     map[string]any{"type": "integer", "minimum": 1},
							"path_params": map[string]any{"type": "object", "additionalProperties": true},
							"query":       map[string]any{"type": "object", "additionalProperties": true},
							"body":        map[string]any{"type": "object", "additionalProperties": true},
						},
						"additionalProperties": false,
					},
				},
				"params": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path_params": map[string]any{"type": "object", "additionalProperties": true},
						"query":       map[string]any{"type": "object", "additionalProperties": true},
						"body":        map[string]any{"type": "object", "additionalProperties": true},
					},
					"additionalProperties": false,
				},
				"reason": map[string]any{"type": "string", "maxLength": 500},
			}, []string{"operation_id"}),
		),
	}
	if e.adminScope && e.actorRole >= 100 && e.adminDBTools {
		defs = append(defs, e.dbDefinitions()...)
	}
	return defs
}

func (e *DefaultToolExecutor) listOperations(rawArguments string) ToolResult {
	args := struct {
		Scope  string `json:"scope"`
		Domain string `json:"domain"`
		Query  string `json:"query"`
		Limit  *int   `json:"limit"`
	}{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	profile := e.actor().Profile
	if profile != "user" && profile != "admin" {
		return toolErrorf("unknown assistant profile")
	}
	limit := 50
	if args.Limit != nil {
		limit = *args.Limit
	}
	operations := assistantops.List(profile, e.actorRole, args.Domain, args.Query, limit)
	items := make([]map[string]any, 0, len(operations))
	for _, op := range operations {
		items = append(items, map[string]any{
			"operation_id": op.ID,
			"method":       op.Method,
			"path":         op.Path,
			"domain":       op.Domain,
			"scope":        op.Scope,
			"risk":         op.Risk,
			"summary":      op.Summary,
			"path_params":  op.PathParams,
			"batchable":    op.Batchable,
		})
	}
	return ToolResult{OK: true, Data: map[string]any{"operations": items, "returned": len(items)}}
}

func (e *DefaultToolExecutor) getOperation(rawArguments string) ToolResult {
	args := struct {
		OperationID string `json:"operation_id"`
	}{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	op, ok := assistantops.Find(strings.TrimSpace(args.OperationID))
	if !ok {
		return toolErrorf("unknown operation: %s", args.OperationID)
	}
	if !assistantops.Visible(op, e.actor().Profile, e.actorRole) {
		return toolErrorf("operation is not available to this assistant profile")
	}
	return ToolResult{OK: true, Data: map[string]any{
		"operation_id": op.ID,
		"method":       op.Method,
		"path":         op.Path,
		"domain":       op.Domain,
		"scope":        op.Scope,
		"risk":         op.Risk,
		"summary":      op.Summary,
		"description":  op.Description,
		"path_params":  op.PathParams,
		"batchable":    op.Batchable,
	}}
}

type executeOperationArguments struct {
	OperationID string         `json:"operation_id"`
	PathParams  map[string]any `json:"path_params"`
	Query       map[string]any `json:"query"`
	Body        any            `json:"body"`
	Reason      string         `json:"reason"`
}

func (e *DefaultToolExecutor) executeOperation(ctx context.Context, rawArguments string, audit AuditFunc) ToolResult {
	args := executeOperationArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	op, ok := assistantops.Find(strings.TrimSpace(args.OperationID))
	if !ok {
		return toolErrorf("unknown operation: %s", args.OperationID)
	}
	if !assistantops.Visible(op, e.actor().Profile, e.actorRole) {
		return toolErrorf("operation is not available to this assistant profile")
	}
	if e.actor().Profile == "user" {
		if err := rejectUserIdentityBody(args.Body); err != nil {
			return toolError(err)
		}
		if err := rejectUserIdentityBody(args.Query); err != nil {
			return toolError(err)
		}
	}
	if op.Risk == assistantops.RiskWrite || op.Risk == assistantops.RiskDestructive {
		if err := e.askConfirmation(ctx, ConfirmationRequest{
			OperationID: op.ID,
			Method:      op.Method,
			Path:        op.Path,
			Risk:        string(op.Risk),
			Summary:     confirmationSummary(op, args.Body),
			Params:      redactOperationParams(op, args.PathParams, args.Query, args.Body),
		}); err != nil {
			return toolErrorf("%v", err)
		}
	}
	result := assistantops.Invoke(ctx, e.actor(), op, args.PathParams, args.Query, args.Body)
	if result.Data != nil {
		result.Data = redactValue(result.Data, op.RedactFields)
	}
	if audit != nil {
		params := map[string]any{
			"operation": op.ID,
			"risk":      op.Risk,
			"success":   result.Success,
			"status":    result.StatusCode,
			"via":       "assistant",
		}
		if target := operationTarget(op, args.PathParams, args.Body); target != "" {
			params["target"] = target
		}
		audit("assistant.operation", params)
	}
	toolResult := ToolResult{OK: result.Success, Data: result}
	if !result.Success {
		toolResult.Error = result.Message
		if toolResult.Error == "" {
			toolResult.Error = fmt.Sprintf("operation failed with status %d", result.StatusCode)
		}
	}
	return toolResult
}

type batchOperationArguments struct {
	OperationID string                    `json:"operation_id"`
	UserIDs     []int                     `json:"user_ids"`
	Filter      map[string]any            `json:"filter"`
	Items       []map[string]any          `json:"items"`
	Params      map[string]map[string]any `json:"params"`
	Reason      string                    `json:"reason"`
}

func (e *DefaultToolExecutor) batchOperation(ctx context.Context, rawArguments string, audit AuditFunc) ToolResult {
	if !e.adminScope {
		return toolErrorf("batch operations are available only to administrators")
	}
	args := batchOperationArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	op, ok := assistantops.Find(strings.TrimSpace(args.OperationID))
	if !ok {
		return toolErrorf("unknown operation: %s", args.OperationID)
	}
	if !assistantops.Visible(op, e.actor().Profile, e.actorRole) || !op.Batchable || op.BatchTarget == "" {
		return toolErrorf("operation is not batchable or not available")
	}
	targets, err := e.resolveBatchTargets(args)
	if err != nil {
		return toolErrorf("%v", err)
	}
	if len(targets) == 0 {
		return toolErrorf("no target users matched")
	}
	if len(targets) > 100 {
		return toolErrorf("batch target limit is 100 users")
	}
	if err := e.askConfirmation(ctx, ConfirmationRequest{
		OperationID: op.ID,
		Method:      op.Method,
		Path:        op.Path,
		Risk:        string(op.Risk),
		Summary:     fmt.Sprintf("%s for %d users", op.Summary, len(targets)),
		Params:      map[string]any{"operation": op.ID, "user_count": len(targets)},
	}); err != nil {
		return toolErrorf("%v", err)
	}
	results := make([]map[string]any, 0, len(targets))
	succeeded := 0
	bodyTarget := strings.HasPrefix(op.BatchTarget, "body:")
	bodyField := strings.TrimPrefix(op.BatchTarget, "body:")
	for _, target := range targets {
		pathParams := map[string]any{}
		query := map[string]any{}
		body := map[string]any{}
		if item := target.Item; item != nil {
			if raw, ok := item["path_params"].(map[string]any); ok {
				for key, value := range raw {
					pathParams[key] = value
				}
			}
			if raw, ok := item["query"].(map[string]any); ok {
				query = raw
			}
			if raw, ok := item["body"].(map[string]any); ok {
				for key, value := range raw {
					body[key] = value
				}
			}
		} else if args.Params != nil {
			if raw, ok := args.Params["path_params"]; ok {
				for key, value := range raw {
					pathParams[key] = value
				}
			}
			if raw, ok := args.Params["query"]; ok {
				query = raw
			}
			if raw, ok := args.Params["body"]; ok {
				for key, value := range raw {
					body[key] = value
				}
			}
		}
		var requestBody any
		if bodyTarget {
			body[bodyField] = target.UserID
			requestBody = body
		} else {
			pathParams[op.BatchTarget] = target.UserID
			if len(body) > 0 {
				requestBody = body
			}
		}
		result := assistantops.Invoke(ctx, e.actor(), op, pathParams, query, requestBody)
		entry := map[string]any{
			"user_id": target.UserID,
			"success": result.Success,
			"status":  result.StatusCode,
		}
		if result.Message != "" {
			entry["message"] = result.Message
		}
		if result.Success {
			succeeded++
		}
		if audit != nil {
			audit("assistant.operation", map[string]any{
				"operation": op.ID,
				"target":    target.UserID,
				"success":   result.Success,
				"status":    result.StatusCode,
				"batch":     true,
				"via":       "assistant",
			})
		}
		results = append(results, entry)
	}
	return ToolResult{OK: succeeded == len(targets), Data: map[string]any{
		"operation": op.ID,
		"total":     len(targets),
		"succeeded": succeeded,
		"failed":    len(targets) - succeeded,
		"results":   results,
	}}
}

type batchTarget struct {
	UserID int
	Item   map[string]any
}

func (e *DefaultToolExecutor) resolveBatchTargets(args batchOperationArguments) ([]batchTarget, error) {
	targets := make([]batchTarget, 0, len(args.UserIDs))
	if len(args.Items) > 0 {
		for _, item := range args.Items {
			userID := 0
			if raw, ok := item["user_id"]; ok {
				if parsed, ok := raw.(float64); ok {
					userID = int(parsed)
				}
				if parsed, ok := raw.(int); ok {
					userID = parsed
				}
			}
			if userID <= 0 {
				return nil, fmt.Errorf("each item requires a positive user_id")
			}
			targets = append(targets, batchTarget{UserID: userID, Item: item})
		}
		return targets, nil
	}
	if len(args.UserIDs) > 0 {
		for _, userID := range args.UserIDs {
			if userID <= 0 {
				return nil, fmt.Errorf("user_ids must be positive")
			}
			targets = append(targets, batchTarget{UserID: userID})
		}
		return targets, nil
	}
	if len(args.Filter) == 0 {
		return nil, fmt.Errorf("provide user_ids, filter, or items")
	}
	keyword, _ := args.Filter["keyword"].(string)
	group, _ := args.Filter["group"].(string)
	var role *int
	if raw, ok := args.Filter["role"]; ok {
		if parsed, ok := raw.(float64); ok {
			value := int(parsed)
			role = &value
		}
	}
	var status *int
	if raw, ok := args.Filter["status"]; ok {
		if parsed, ok := raw.(float64); ok {
			value := int(parsed)
			status = &value
		}
	}
	users, _, err := model.SearchUsers(keyword, group, role, status, 0, 100)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		if user != nil {
			targets = append(targets, batchTarget{UserID: user.Id})
		}
	}
	return targets, nil
}

func rejectUserIdentityBody(body any) error {
	bodyMap, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	blocked := []string{"user_id", "role", "access_token", "refresh_token", "password", "original_password"}
	for _, field := range blocked {
		if _, exists := bodyMap[field]; exists {
			return fmt.Errorf("field %s is not allowed in user assistant operations", field)
		}
	}
	return nil
}

func confirmationSummary(op assistantops.Operation, body any) string {
	if body == nil {
		return op.Summary
	}
	return op.Summary + " (JSON body provided)"
}

func operationTarget(op assistantops.Operation, pathParams map[string]any, body any) string {
	if op.BatchTarget != "" {
		if value, ok := pathParams[op.BatchTarget]; ok {
			return fmt.Sprint(value)
		}
	}
	if bodyMap, ok := body.(map[string]any); ok {
		if value, ok := bodyMap["user_id"]; ok {
			return fmt.Sprint(value)
		}
	}
	return ""
}

func redactOperationParams(op assistantops.Operation, pathParams, query map[string]any, body any) map[string]any {
	params := map[string]any{
		"path_params": redactValue(pathParams, op.RedactFields),
		"query":       redactValue(query, op.RedactFields),
	}
	if body != nil {
		params["body"] = redactValue(body, op.RedactFields)
	}
	return params
}

func redactValue(value any, fields []string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if isRedactedField(key, fields) {
				result[key] = "[REDACTED]"
				continue
			}
			result[key] = redactValue(item, fields)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, redactValue(item, fields))
		}
		return result
	default:
		return value
	}
}

func isRedactedField(key string, fields []string) bool {
	lower := strings.ToLower(key)
	for _, field := range fields {
		if lower == strings.ToLower(field) || strings.Contains(lower, strings.ToLower(field)) {
			return true
		}
	}
	return false
}

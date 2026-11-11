package assistant

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/model"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"
)

const maxBatchTargets = 100

func (e *DefaultToolExecutor) genericDefinitions() []map[string]any {
	executeDescription := "Execute one dashboard API operation within your own account. Operations with risk write or destructive wait for the user's confirmation. Use path_params for :id style segments, query for query string parameters, and body for the JSON request body. Call get_operation first: it documents body_fields, query_fields, units, and pitfalls. List endpoints are paged with query p and page_size; keep page_size small."
	if e.adminScope {
		executeDescription = "Execute one dashboard API operation within the administrator's permissions. Operations with risk write or destructive wait for the administrator's confirmation. Use path_params for :id style segments, query for query string parameters, and body for the JSON request body. Call get_operation first: it documents body_fields, query_fields, units, and pitfalls. List endpoints are paged with query p and page_size; keep page_size small."
	}
	defs := []map[string]any{
		functionTool(
			"list_operations",
			"List dashboard API operations available to the current assistant profile, one page at a time. Returns operation_id, risk, summary, and path parameters, plus total and next_offset for paging and a per-domain count. Filter by domain or query instead of paging through everything.",
			objectSchema(map[string]any{
				"domain": map[string]any{"type": "string", "description": "Optional domain such as user, token, channel, log, option, redemption, subscription."},
				"query":  map[string]any{"type": "string", "maxLength": 128, "description": "Optional text search over ids, summaries, descriptions, and parameter names."},
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "description": "Maximum operations to return. Defaults to 50."},
				"offset": map[string]any{"type": "integer", "minimum": 0, "description": "Number of matching operations to skip. Use next_offset from the previous page."},
			}, nil),
		),
		functionTool(
			"get_operation",
			"Get the full description of one operation_id: path parameters, documented query_fields and body_fields with units, notes about pitfalls, and its risk. Read it before calling execute_operation so names and units are not guessed.",
			objectSchema(map[string]any{
				"operation_id": map[string]any{"type": "string", "description": "Operation id such as POST /api/token/."},
			}, []string{"operation_id"}),
		),
		functionTool(
			"execute_operation",
			executeDescription,
			objectSchema(map[string]any{
				"operation_id": map[string]any{"type": "string", "description": "Exact operation id from list_operations."},
				"path_params":  map[string]any{"type": "object", "additionalProperties": true, "description": "Values for the :name segments of the path."},
				"query":        map[string]any{"type": "object", "additionalProperties": true, "description": "Query string parameters."},
				"body":         map[string]any{"type": "object", "additionalProperties": true, "description": "JSON request body."},
				"reason":       map[string]any{"type": "string", "maxLength": 500, "description": "Short reason shown on the confirmation card."},
			}, []string{"operation_id"}),
		),
	}
	if e.adminScope {
		defs = append(defs, functionTool(
			"batch_operation",
			"Run one user-targeted operation (batchable=true in get_operation) for many users after a single confirmation. Choose targets with exactly one of user_ids, filter, or items. Shared parameters go in params; items carry per-user parameters. Maximum 100 distinct users. Per-target results are returned; there is no rollback.",
			objectSchema(map[string]any{
				"operation_id": map[string]any{"type": "string", "description": "Exact operation id of a batchable operation."},
				"user_ids":     map[string]any{"type": "array", "items": map[string]any{"type": "integer", "minimum": 1}, "description": "Target user ids. Duplicates are ignored."},
				"filter": map[string]any{
					"type":        "object",
					"description": "Select target users by search. Soft-deleted users are never selected. Fails when more than 100 users match.",
					"properties": map[string]any{
						"keyword": map[string]any{"type": "string", "maxLength": 100, "description": "Matches username, email, or display name."},
						"status":  map[string]any{"type": "integer", "description": "1 enabled, 2 disabled."},
						"role":    map[string]any{"type": "integer", "description": "1 common user, 10 admin, 100 root."},
						"group":   map[string]any{"type": "string", "maxLength": 64, "description": "Exact user group."},
					},
					"additionalProperties": false,
				},
				"items": map[string]any{
					"type":        "array",
					"description": "Per-user parameters when they differ between users.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"user_id":     map[string]any{"type": "integer", "minimum": 1},
							"path_params": map[string]any{"type": "object", "additionalProperties": true},
							"query":       map[string]any{"type": "object", "additionalProperties": true},
							"body":        map[string]any{"type": "object", "additionalProperties": true},
						},
						"required":             []string{"user_id"},
						"additionalProperties": false,
					},
				},
				"params": map[string]any{
					"type":        "object",
					"description": "Parameters applied to every target selected by user_ids or filter.",
					"properties": map[string]any{
						"path_params": map[string]any{"type": "object", "additionalProperties": true},
						"query":       map[string]any{"type": "object", "additionalProperties": true},
						"body":        map[string]any{"type": "object", "additionalProperties": true},
					},
					"additionalProperties": false,
				},
				"reason": map[string]any{"type": "string", "maxLength": 500, "description": "Short reason shown on the confirmation card."},
			}, []string{"operation_id"}),
		))
	}
	if e.canUseDBTools() {
		defs = append(defs, e.dbDefinitions()...)
	}
	return defs
}

func (e *DefaultToolExecutor) listOperations(rawArguments string) ToolResult {
	args := struct {
		Domain string `json:"domain"`
		Query  string `json:"query"`
		Limit  *int   `json:"limit"`
		Offset *int   `json:"offset"`
	}{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	profile := e.actor().Profile
	if profile != "user" && profile != "admin" {
		return toolErrorf("unknown assistant profile")
	}
	limit, err := normalizeLimit(args.Limit, 50, 100)
	if err != nil {
		return toolError(err)
	}
	offset, err := normalizeOffset(args.Offset)
	if err != nil {
		return toolError(err)
	}
	operations, total := assistantops.List(profile, e.actorRole, args.Domain, args.Query, offset, limit)
	items := make([]map[string]any, 0, len(operations))
	for _, op := range operations {
		item := map[string]any{
			"operation_id": op.ID,
			"domain":       op.Domain,
			"scope":        op.Scope,
			"risk":         op.Risk,
			"summary":      op.Summary,
		}
		if len(op.PathParams) > 0 {
			item["path_params"] = op.PathParams
		}
		if e.adminScope && op.Batchable {
			item["batchable"] = true
		}
		items = append(items, item)
	}
	data := map[string]any{
		"operations": items,
		"returned":   len(items),
		"total":      total,
		"offset":     offset,
		"domains":    assistantops.Domains(profile, e.actorRole),
	}
	if next := offset + len(items); next < total {
		data["next_offset"] = next
	}
	return ToolResult{OK: true, Data: data}
}

func (e *DefaultToolExecutor) getOperation(rawArguments string) ToolResult {
	args := struct {
		OperationID string `json:"operation_id"`
	}{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	op, errResult := e.findVisibleOperation(args.OperationID)
	if errResult != nil {
		return *errResult
	}
	data := map[string]any{
		"operation_id":          op.ID,
		"method":                op.Method,
		"path":                  op.Path,
		"domain":                op.Domain,
		"scope":                 op.Scope,
		"risk":                  op.Risk,
		"requires_confirmation": e.confirmWrites && op.Risk != assistantops.RiskRead,
		"summary":               op.Summary,
		"description":           op.Description,
		"path_params":           op.PathParams,
		"body_allowed":          op.BodyAllowed,
	}
	if len(op.QueryFields) > 0 {
		data["query_fields"] = op.QueryFields
	}
	if len(op.BodyFields) > 0 {
		data["body_fields"] = op.BodyFields
	}
	if op.BodyNotes != "" {
		data["body_notes"] = op.BodyNotes
	}
	if len(op.QueryFields) == 0 && len(op.BodyFields) == 0 && (op.BodyAllowed || op.Method == "GET") {
		data["parameters_documented"] = false
	}
	if e.adminScope {
		data["batchable"] = op.Batchable
		if op.Batchable {
			data["batch_target"] = op.BatchTarget
		}
	}
	return ToolResult{OK: true, Data: data}
}

// findVisibleOperation resolves an operation id for the current profile. The
// two failure modes share one wording per case so a hidden operation is not
// distinguishable from a missing one beyond what list_operations shows.
func (e *DefaultToolExecutor) findVisibleOperation(id string) (assistantops.Operation, *ToolResult) {
	op, ok := assistantops.Find(strings.TrimSpace(id))
	if !ok {
		result := toolErrorf("unknown operation: %s. Use list_operations to find the exact operation_id.", id)
		return assistantops.Operation{}, &result
	}
	if !assistantops.Visible(op, e.actor().Profile, e.actorRole) {
		result := toolErrorf("operation is not available to this assistant profile")
		return assistantops.Operation{}, &result
	}
	return op, nil
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
	op, errResult := e.findVisibleOperation(args.OperationID)
	if errResult != nil {
		return *errResult
	}
	if e.actor().Profile == "user" {
		if err := rejectUserIdentityBody(args.Body); err != nil {
			return toolError(err)
		}
		if err := rejectUserIdentityBody(args.Query); err != nil {
			return toolError(err)
		}
	}
	// Fail on malformed input before asking a human to approve it.
	if _, err := assistantops.RenderPath(op, args.PathParams); err != nil {
		return toolError(err)
	}
	if op.Risk != assistantops.RiskRead {
		params := operationParams(op, args.PathParams, args.Query, args.Body)
		if reason := strings.TrimSpace(args.Reason); reason != "" {
			params["reason"] = reason
		}
		if err := e.askConfirmation(ctx, ConfirmationRequest{
			OperationID: op.ID,
			Method:      op.Method,
			Path:        op.Path,
			Risk:        string(op.Risk),
			Summary:     op.Summary,
			Params:      params,
		}); err != nil {
			return toolError(err)
		}
	}
	result := assistantops.Invoke(ctx, e.actor(), op, args.PathParams, args.Query, args.Body)
	if result.Data != nil {
		result.Data = redactOperationValue(op, result.Data)
	}
	if note := op.SecretResultNote(); note != "" && result.Success {
		result.Data = map[string]any{"note": note}
		result.Raw = ""
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

// batchCall is one fully resolved per-user invocation.
type batchCall struct {
	UserID     int
	PathParams map[string]any
	Query      map[string]any
	Body       any
}

func (e *DefaultToolExecutor) batchOperation(ctx context.Context, rawArguments string, audit AuditFunc) ToolResult {
	if !e.adminScope {
		return toolErrorf("batch operations are available only to administrators")
	}
	args := batchOperationArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	op, errResult := e.findVisibleOperation(args.OperationID)
	if errResult != nil {
		return *errResult
	}
	if !op.Batchable || op.BatchTarget == "" {
		return toolErrorf("operation %s does not target one user per call and cannot be batched; use execute_operation", op.ID)
	}
	targets, err := resolveBatchTargets(args)
	if err != nil {
		return toolError(err)
	}
	if len(targets) == 0 {
		return toolErrorf("no target users matched")
	}
	if len(targets) > maxBatchTargets {
		return toolErrorf("batch target limit is %d users", maxBatchTargets)
	}

	calls := make([]batchCall, 0, len(targets))
	for _, target := range targets {
		call := buildBatchCall(op, target, args.Params)
		if _, err := assistantops.RenderPath(op, call.PathParams); err != nil {
			return toolErrorf("user %d: %v", target.UserID, err)
		}
		calls = append(calls, call)
	}

	if op.Risk != assistantops.RiskRead {
		if err := e.askConfirmation(ctx, ConfirmationRequest{
			OperationID: op.ID,
			Method:      op.Method,
			Path:        op.Path,
			Risk:        string(op.Risk),
			Summary:     fmt.Sprintf("%s for %d users", op.Summary, len(calls)),
			Params:      batchConfirmationParams(op, args, calls),
		}); err != nil {
			return toolError(err)
		}
	}

	results := make([]map[string]any, 0, len(calls))
	succeeded := 0
	for _, call := range calls {
		// Stop fanning out once the run is cancelled; report what was done.
		if ctx.Err() != nil {
			results = append(results, map[string]any{"user_id": call.UserID, "success": false, "message": "not attempted: request cancelled"})
			continue
		}
		result := assistantops.Invoke(ctx, e.actor(), op, call.PathParams, call.Query, call.Body)
		entry := map[string]any{
			"user_id": call.UserID,
			"success": result.Success,
			"status":  result.StatusCode,
		}
		if result.Message != "" {
			entry["message"] = result.Message
		}
		if op.Risk == assistantops.RiskRead && result.Data != nil {
			entry["data"] = redactOperationValue(op, result.Data)
		}
		if result.Success {
			succeeded++
		}
		if audit != nil {
			audit("assistant.operation", map[string]any{
				"operation": op.ID,
				"risk":      op.Risk,
				"target":    call.UserID,
				"success":   result.Success,
				"status":    result.StatusCode,
				"batch":     true,
				"via":       "assistant",
			})
		}
		results = append(results, entry)
	}
	toolResult := ToolResult{OK: succeeded == len(calls), Data: map[string]any{
		"operation": op.ID,
		"total":     len(calls),
		"succeeded": succeeded,
		"failed":    len(calls) - succeeded,
		"results":   results,
	}}
	if !toolResult.OK {
		toolResult.Error = fmt.Sprintf("%d of %d targets failed; see results", len(calls)-succeeded, len(calls))
	}
	return toolResult
}

// buildBatchCall merges the shared or per-item parameters for one target and
// writes the user id into the path parameter or body field the operation uses.
func buildBatchCall(op assistantops.Operation, target batchTarget, shared map[string]map[string]any) batchCall {
	source := map[string]any{}
	if target.Item != nil {
		source = target.Item
	} else {
		for key, value := range shared {
			source[key] = value
		}
	}
	call := batchCall{UserID: target.UserID, PathParams: map[string]any{}, Query: map[string]any{}}
	if raw, ok := source["path_params"].(map[string]any); ok {
		for key, value := range raw {
			call.PathParams[key] = value
		}
	}
	if raw, ok := source["query"].(map[string]any); ok {
		for key, value := range raw {
			call.Query[key] = value
		}
	}
	body := map[string]any{}
	if raw, ok := source["body"].(map[string]any); ok {
		for key, value := range raw {
			body[key] = value
		}
	}
	if field, ok := strings.CutPrefix(op.BatchTarget, "body:"); ok {
		body[field] = target.UserID
	} else {
		call.PathParams[op.BatchTarget] = target.UserID
	}
	if len(body) > 0 {
		call.Body = body
	}
	return call
}

// batchConfirmationParams shows the approver exactly who is affected and with
// which parameters, so a batch is never approved blind.
func batchConfirmationParams(op assistantops.Operation, args batchOperationArguments, calls []batchCall) map[string]any {
	userIDs := make([]int, 0, len(calls))
	for _, call := range calls {
		userIDs = append(userIDs, call.UserID)
	}
	params := map[string]any{
		"user_count": len(calls),
		"user_ids":   userIDs,
	}
	if len(args.Items) > 0 {
		items := make([]any, 0, len(calls))
		for _, call := range calls {
			item := operationParams(op, call.PathParams, call.Query, call.Body)
			item["user_id"] = call.UserID
			items = append(items, item)
		}
		params["items"] = items
	} else {
		// Every target shares the same parameters; show them once.
		shared := operationParams(op, nil, calls[0].Query, calls[0].Body)
		if body, ok := shared["body"].(map[string]any); ok {
			if field, isBody := strings.CutPrefix(op.BatchTarget, "body:"); isBody {
				delete(body, field)
			}
			if len(body) == 0 {
				delete(shared, "body")
			}
		}
		for key, value := range shared {
			params[key] = value
		}
		if len(args.Filter) > 0 {
			params["filter"] = args.Filter
		}
	}
	if reason := strings.TrimSpace(args.Reason); reason != "" {
		params["reason"] = reason
	}
	return params
}

type batchTarget struct {
	UserID int
	Item   map[string]any
}

// resolveBatchTargets turns user_ids, items, or a filter into a list of
// distinct users. A user listed twice is targeted once, so a repeated id can
// never apply a grant or quota change twice.
func resolveBatchTargets(args batchOperationArguments) ([]batchTarget, error) {
	sources := 0
	for _, present := range []bool{len(args.Items) > 0, len(args.UserIDs) > 0, len(args.Filter) > 0} {
		if present {
			sources++
		}
	}
	if sources == 0 {
		return nil, fmt.Errorf("provide user_ids, filter, or items")
	}
	if sources > 1 {
		return nil, fmt.Errorf("provide only one of user_ids, filter, or items")
	}

	seen := map[int]struct{}{}
	targets := make([]batchTarget, 0, len(args.UserIDs)+len(args.Items))
	if len(args.Items) > 0 {
		for _, item := range args.Items {
			userID, ok := jsonInt(item["user_id"])
			if !ok || userID <= 0 {
				return nil, fmt.Errorf("each item requires a positive user_id")
			}
			if _, duplicate := seen[userID]; duplicate {
				return nil, fmt.Errorf("user %d appears in more than one item", userID)
			}
			seen[userID] = struct{}{}
			targets = append(targets, batchTarget{UserID: userID, Item: item})
		}
		return targets, nil
	}
	if len(args.UserIDs) > 0 {
		for _, userID := range args.UserIDs {
			if userID <= 0 {
				return nil, fmt.Errorf("user_ids must be positive")
			}
			if _, duplicate := seen[userID]; duplicate {
				continue
			}
			seen[userID] = struct{}{}
			targets = append(targets, batchTarget{UserID: userID})
		}
		return targets, nil
	}

	for key := range args.Filter {
		switch key {
		case "keyword", "group", "role", "status":
		default:
			return nil, fmt.Errorf("unknown filter field: %s", key)
		}
	}
	keyword, _ := args.Filter["keyword"].(string)
	group, _ := args.Filter["group"].(string)
	var role *int
	if raw, present := args.Filter["role"]; present {
		value, ok := jsonInt(raw)
		if !ok {
			return nil, fmt.Errorf("filter.role must be an integer")
		}
		role = &value
	}
	var status *int
	if raw, present := args.Filter["status"]; present {
		value, ok := jsonInt(raw)
		if !ok || value <= 0 {
			return nil, fmt.Errorf("filter.status must be 1 (enabled) or 2 (disabled)")
		}
		status = &value
	}
	if strings.TrimSpace(keyword) == "" && group == "" && role == nil && status == nil {
		return nil, fmt.Errorf("filter must set at least one of keyword, group, role, status")
	}
	users, total, err := model.SearchUsers(strings.TrimSpace(keyword), group, role, status, 0, maxBatchTargets+1)
	if err != nil {
		return nil, err
	}
	// Refuse instead of silently acting on an arbitrary first page.
	if total > maxBatchTargets {
		return nil, fmt.Errorf("filter matches %d users, more than the batch limit of %d; narrow the filter or pass explicit user_ids", total, maxBatchTargets)
	}
	for _, user := range users {
		if user == nil || user.DeletedAt.Valid {
			continue
		}
		targets = append(targets, batchTarget{UserID: user.Id})
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].UserID < targets[j].UserID })
	return targets, nil
}

// jsonInt reads an integer from a decoded JSON value.
func jsonInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}
		return int(typed), true
	case int:
		return typed, true
	default:
		return 0, false
	}
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

func operationTarget(op assistantops.Operation, pathParams map[string]any, body any) string {
	bodyMap, _ := body.(map[string]any)
	if field, ok := strings.CutPrefix(op.BatchTarget, "body:"); ok {
		if value, exists := bodyMap[field]; exists {
			return fmt.Sprint(value)
		}
	} else if op.BatchTarget != "" {
		if value, exists := pathParams[op.BatchTarget]; exists {
			return fmt.Sprint(value)
		}
	}
	if value, exists := bodyMap["user_id"]; exists {
		return fmt.Sprint(value)
	}
	return ""
}

// operationParams is the redacted view of one invocation shown on the
// confirmation card. Empty sections are left out.
func operationParams(op assistantops.Operation, pathParams, query map[string]any, body any) map[string]any {
	params := map[string]any{}
	if len(pathParams) > 0 {
		params["path_params"] = redactOperationValue(op, pathParams)
	}
	if len(query) > 0 {
		params["query"] = redactOperationValue(op, query)
	}
	if body != nil {
		params["body"] = redactOperationValue(op, body)
	}
	return params
}

const redactedPlaceholder = "[REDACTED]"

// redactOperationValue hides credential fields of one operation's request or
// response. For site options, "key" is the option name and stays visible; the
// value is hidden instead when the option itself stores a credential.
func redactOperationValue(op assistantops.Operation, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if op.IsSecretField(key) {
				result[key] = redactedPlaceholder
				continue
			}
			result[key] = redactOperationValue(op, item)
		}
		if op.Domain == "option" {
			if name, ok := typed["key"].(string); ok && assistantops.IsSecretOptionKey(name) {
				if _, hasValue := result["value"]; hasValue {
					result["value"] = redactedPlaceholder
				}
			}
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, redactOperationValue(op, item))
		}
		return result
	default:
		return value
	}
}

// DisplayArguments returns the tool arguments as they may appear in the
// transcript. Generic operation tools are redacted with the rules of the
// operation they address; everything else uses the default credential rules.
func (e *DefaultToolExecutor) DisplayArguments(call ToolCall) any {
	if strings.TrimSpace(call.Arguments) == "" {
		return nil
	}
	var arguments any
	if err := decodeArguments(call.Arguments, &arguments); err != nil {
		return map[string]any{"raw": redactedPlaceholder}
	}
	op := assistantops.Operation{}
	if call.Name == "execute_operation" || call.Name == "batch_operation" {
		if fields, ok := arguments.(map[string]any); ok {
			if id, ok := fields["operation_id"].(string); ok {
				if found, exists := assistantops.Find(strings.TrimSpace(id)); exists {
					op = found
				}
			}
		}
	}
	return redactOperationValue(op, arguments)
}

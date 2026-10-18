package assistant

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type usageSummaryArguments struct {
	Days    *int   `json:"days"`
	GroupBy string `json:"group_by"`
	Limit   *int   `json:"limit"`
}

type usageLogsArguments struct {
	Days      *int   `json:"days"`
	Model     string `json:"model"`
	TokenName string `json:"token_name"`
	Limit     *int   `json:"limit"`
}

type listKeysArguments struct {
	Limit *int `json:"limit"`
}

func (e *DefaultToolExecutor) getAccountSummary(userID int) ToolResult {
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return toolError(err)
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	today := model.UserConsumeTotals{}
	todayUnavailable := ""
	if totals, err := model.GetUserConsumeTotals(userID, start, now.Unix()); err != nil {
		todayUnavailable = err.Error()
	} else {
		today = totals
	}
	data := map[string]any{
		"username":      user.Username,
		"display_name":  user.DisplayName,
		"group":         user.Group,
		"quota":         user.Quota,
		"used_quota":    user.UsedQuota,
		"request_count": user.RequestCount,
		"quota_display": DisplayQuota(int64(user.Quota)),
		"used_display":  DisplayQuota(int64(user.UsedQuota)),
		"today":         consumeTotalsMap(today),
	}
	if todayUnavailable != "" {
		data["today_unavailable"] = todayUnavailable
	}
	return ToolResult{OK: true, Data: data}
}

func (e *DefaultToolExecutor) getUsageSummary(userID int, rawArguments string) ToolResult {
	args := usageSummaryArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	days, err := normalizeDays(args.Days)
	if err != nil {
		return toolError(err)
	}
	groupBy := strings.TrimSpace(args.GroupBy)
	if groupBy == "" {
		groupBy = "model"
	}
	if groupBy != "model" && groupBy != "group" && groupBy != "token" {
		return toolErrorf("group_by must be one of model, group, token")
	}
	limit, err := normalizeLimit(args.Limit, 10, 20)
	if err != nil {
		return toolError(err)
	}

	end := time.Now().Unix()
	start := end - int64(days)*86400
	totals, err := model.GetUserConsumeTotals(userID, start, end)
	if err != nil {
		return toolError(err)
	}
	requestLimit := limit + 1
	if requestLimit > 50 {
		requestLimit = 50
	}
	rows, err := model.GetUserConsumeBreakdown(userID, start, end, groupBy, requestLimit)
	if err != nil {
		return toolError(err)
	}
	truncated := len(rows) > limit
	if truncated {
		rows = rows[:limit]
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"name":    row.Dimension,
			"quota":   row.Quota,
			"count":   row.Count,
			"tokens":  row.Tokens,
			"display": DisplayQuota(row.Quota),
		})
	}
	return ToolResult{OK: true, Data: map[string]any{
		"period_start":  start,
		"period_end":    end,
		"days":          days,
		"group_by":      groupBy,
		"totals":        consumeTotalsMap(totals),
		"total_display": DisplayQuota(totals.Quota),
		"rows":          items,
		"truncated":     truncated,
	}}
}

func (e *DefaultToolExecutor) listUsageLogs(userID int, rawArguments string) ToolResult {
	args := usageLogsArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	days, err := normalizeDays(args.Days)
	if err != nil {
		return toolError(err)
	}
	limit, err := normalizeLimit(args.Limit, 10, 20)
	if err != nil {
		return toolError(err)
	}
	modelName := strings.TrimSpace(args.Model)
	if len(modelName) > 128 {
		return toolErrorf("model filter is too long")
	}
	tokenName := strings.TrimSpace(args.TokenName)
	if len(tokenName) > 50 {
		return toolErrorf("token_name filter is too long")
	}
	if model.LOG_DB == nil {
		return toolErrorf("usage logs are temporarily unavailable")
	}
	end := time.Now().Unix()
	start := end - int64(days)*86400
	logs, total, err := model.GetUserLogs(userID, model.LogTypeConsume, "", start, end, modelName, tokenName, 0, limit, "", "", "", false)
	if err != nil {
		return toolError(err)
	}
	items := make([]map[string]any, 0, len(logs))
	for _, entry := range logs {
		content := entry.Content
		if len(content) > 500 {
			content = content[:500]
		}
		items = append(items, map[string]any{
			"created_at":        entry.CreatedAt,
			"created_at_iso":    time.Unix(entry.CreatedAt, 0).UTC().Format(time.RFC3339),
			"model":             entry.ModelName,
			"token_name":        entry.TokenName,
			"group":             entry.Group,
			"quota":             entry.Quota,
			"display":           DisplayQuota(int64(entry.Quota)),
			"prompt_tokens":     entry.PromptTokens,
			"completion_tokens": entry.CompletionTokens,
			"use_time":          entry.UseTime,
			"is_stream":         entry.IsStream,
			"content":           content,
		})
	}
	return ToolResult{OK: true, Data: map[string]any{
		"period_start": start,
		"period_end":   end,
		"days":         days,
		"total":        total,
		"returned":     len(items),
		"truncated":    total > int64(len(items)),
		"items":        items,
	}}
}

func (e *DefaultToolExecutor) listAPIKeys(userID int, rawArguments string) ToolResult {
	args := listKeysArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	limit, err := normalizeLimit(args.Limit, 50, 50)
	if err != nil {
		return toolError(err)
	}
	tokens, err := model.GetAllUserTokens(userID, 0, limit)
	if err != nil {
		return toolError(err)
	}
	total, err := model.CountUserTokens(userID)
	if err != nil {
		return toolError(err)
	}
	ids := make([]int, 0, len(tokens))
	for _, token := range tokens {
		if token != nil {
			ids = append(ids, token.Id)
		}
	}
	todayUsed, err := model.GetTokenTodayUsedQuota(userID, ids)
	if err != nil {
		todayUsed = map[int]int64{}
	}
	items := make([]map[string]any, 0, len(tokens))
	for _, token := range tokens {
		if token == nil {
			continue
		}
		items = append(items, map[string]any{
			"id":                   token.Id,
			"name":                 token.Name,
			"masked_key":           "sk-" + token.GetMaskedKey(),
			"status":               token.Status,
			"status_text":          tokenStatusText(token.Status),
			"unlimited_quota":      token.UnlimitedQuota,
			"remain_quota":         token.RemainQuota,
			"used_quota":           token.UsedQuota,
			"today_used_quota":     todayUsed[token.Id],
			"remain_display":       DisplayQuota(int64(token.RemainQuota)),
			"expired_time":         token.ExpiredTime,
			"never_expires":        token.ExpiredTime == -1,
			"group":                token.Group,
			"created_time":         token.CreatedTime,
			"model_limits_enabled": token.ModelLimitsEnabled,
		})
	}
	return ToolResult{OK: true, Data: map[string]any{
		"total":     total,
		"returned":  len(items),
		"truncated": total > int64(len(items)),
		"items":     items,
	}}
}

func consumeTotalsMap(totals model.UserConsumeTotals) map[string]any {
	return map[string]any{
		"quota":   totals.Quota,
		"count":   totals.Count,
		"tokens":  totals.Tokens,
		"display": DisplayQuota(totals.Quota),
	}
}

func normalizeDays(value *int) (int, error) {
	days := 7
	if value != nil {
		days = *value
	}
	if days < 1 || days > 31 {
		return 0, fmt.Errorf("days must be between 1 and 31")
	}
	return days, nil
}

func normalizeLimit(value *int, fallback, maxValue int) (int, error) {
	limit := fallback
	if value != nil {
		limit = *value
	}
	if limit < 1 || limit > maxValue {
		return 0, fmt.Errorf("limit must be between 1 and %d", maxValue)
	}
	return limit, nil
}

func tokenStatusText(status int) string {
	switch status {
	case common.TokenStatusEnabled:
		return "enabled"
	case common.TokenStatusDisabled:
		return "disabled"
	case common.TokenStatusExpired:
		return "expired"
	case common.TokenStatusExhausted:
		return "exhausted"
	default:
		return "unknown"
	}
}

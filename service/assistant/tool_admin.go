package assistant

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
)

type listUsersArguments struct {
	Keyword string `json:"keyword"`
	Limit   *int   `json:"limit"`
}

// listUsers is admin-only and provides user ids for the user-scoped admin
// tools. It returns identity and accounting fields, never credentials.
func (e *DefaultToolExecutor) listUsers(rawArguments string) ToolResult {
	args := listUsersArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	keyword := strings.TrimSpace(args.Keyword)
	if len(keyword) > 100 {
		return toolErrorf("keyword is too long")
	}
	limit, err := normalizeLimit(args.Limit, 20, 50)
	if err != nil {
		return toolError(err)
	}
	users, total, err := model.SearchUsers(keyword, "", nil, nil, 0, limit)
	if err != nil {
		return toolError(err)
	}
	items := make([]map[string]any, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		items = append(items, map[string]any{
			"id":            user.Id,
			"username":      user.Username,
			"display_name":  user.DisplayName,
			"role":          user.Role,
			"status":        user.Status,
			"group":         user.Group,
			"quota":         user.Quota,
			"used_quota":    user.UsedQuota,
			"request_count": user.RequestCount,
			"created_at":    user.CreatedAt,
		})
	}
	return ToolResult{OK: true, Data: map[string]any{
		"total":     total,
		"returned":  len(items),
		"truncated": total > int64(len(items)),
		"items":     items,
	}}
}

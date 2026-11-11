package assistant

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
)

type listUsersArguments struct {
	Keyword string `json:"keyword"`
	Group   string `json:"group"`
	Role    *int   `json:"role"`
	Status  *int   `json:"status"`
	Limit   *int   `json:"limit"`
	Offset  *int   `json:"offset"`
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
	offset, err := normalizeOffset(args.Offset)
	if err != nil {
		return toolError(err)
	}
	if args.Status != nil && *args.Status != 1 && *args.Status != 2 && *args.Status != -1 {
		return toolErrorf("status must be 1 (enabled), 2 (disabled), or -1 (soft-deleted)")
	}
	users, total, err := model.SearchUsers(keyword, strings.TrimSpace(args.Group), args.Role, args.Status, offset, limit)
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
			"quota_display": DisplayQuota(int64(user.Quota)),
			"created_at":    user.CreatedAt,
			"deleted":       user.DeletedAt.Valid,
		})
	}
	return ToolResult{OK: true, Data: pagedResult(map[string]any{"items": items}, total, offset, len(items))}
}

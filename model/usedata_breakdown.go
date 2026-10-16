package model

import "github.com/QuantumNous/new-api/common"

// userQuotaBreakdownMaxRows caps a single breakdown response. Rows are ordered
// by quota descending, so a truncated response keeps the highest spenders and
// the frontend can warn that the list is partial.
const userQuotaBreakdownMaxRows = 5000

// UserQuotaBreakdownRow aggregates one user's consumption of one model in one
// group over the requested time range.
type UserQuotaBreakdownRow struct {
	UserID    int    `json:"user_id" gorm:"column:user_id"`
	Username  string `json:"username" gorm:"column:username"`
	UseGroup  string `json:"use_group" gorm:"column:use_group"`
	ModelName string `json:"model_name" gorm:"column:model_name"`
	Count     int64  `json:"count" gorm:"column:count"`
	Quota     int64  `json:"quota" gorm:"column:quota"`
	TokenUsed int64  `json:"token_used" gorm:"column:token_used"`
}

type UserQuotaBreakdownResult struct {
	Rows      []*UserQuotaBreakdownRow `json:"rows"`
	Truncated bool                     `json:"truncated"`
}

// GetUserQuotaBreakdown groups quota_data by user, group and model.
// A start or end timestamp <= 0 leaves that side unbounded, which is how the
// dashboard serves its "all time" range.
// Admins (role >= RoleAdminUser) see every user and may narrow by username;
// everyone else is always scoped to their own user id.
func GetUserQuotaBreakdown(startTime int64, endTime int64, username string, userID int, role int) (*UserQuotaBreakdownResult, error) {
	rows := make([]*UserQuotaBreakdownRow, 0)
	query := DB.Table("quota_data").
		Select("user_id, username, use_group, model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used")
	if role >= common.RoleAdminUser {
		if username != "" {
			query = query.Where("username = ?", username)
		}
	} else {
		query = query.Where("user_id = ?", userID)
	}
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	// One extra row distinguishes "exactly at the cap" from "capped".
	err := query.
		Group("user_id, username, use_group, model_name").
		Order("quota DESC").
		Limit(userQuotaBreakdownMaxRows + 1).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	truncated := len(rows) > userQuotaBreakdownMaxRows
	if truncated {
		rows = rows[:userQuotaBreakdownMaxRows]
	}
	return &UserQuotaBreakdownResult{
		Rows:      rows,
		Truncated: truncated,
	}, nil
}

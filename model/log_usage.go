package model

import (
	"fmt"

	"gorm.io/gorm"
)

// UserConsumeTotals aggregates one user's consume records over a time range.
type UserConsumeTotals struct {
	Quota  int64 `gorm:"column:quota"`
	Count  int64 `gorm:"column:count"`
	Tokens int64 `gorm:"column:tokens"`
}

// UserConsumeBreakdownRow aggregates one user's consume records along a single
// dimension: model, group or token.
type UserConsumeBreakdownRow struct {
	Dimension string `gorm:"column:dimension"`
	Quota     int64  `gorm:"column:quota"`
	Count     int64  `gorm:"column:count"`
	Tokens    int64  `gorm:"column:tokens"`
}

const (
	userConsumeBreakdownDefaultLimit = 10
	userConsumeBreakdownMaxLimit     = 50
)

// userConsumeAggregateSelect keeps totals and breakdown on the same accounting
// semantics: consumed quota, request count and prompt+completion tokens.
const userConsumeAggregateSelect = "COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS count, COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS tokens"

// applyUserConsumeLogFilter scopes a logs query to one user's visible consume
// records. A start or end timestamp <= 0 leaves that side unbounded, which is
// how the dashboard serves its "all time" range.
func applyUserConsumeLogFilter(tx *gorm.DB, userID int, start, end int64) *gorm.DB {
	tx = applyUserVisibleLogFilter(tx.Where("user_id = ? AND type = ?", userID, LogTypeConsume))
	if start > 0 {
		tx = tx.Where("created_at >= ?", start)
	}
	if end > 0 {
		tx = tx.Where("created_at <= ?", end)
	}
	return tx
}

// GetUserConsumeTotals returns the consumed quota, request count and token
// total of one user over the requested time range.
func GetUserConsumeTotals(userID int, start, end int64) (UserConsumeTotals, error) {
	var totals UserConsumeTotals
	err := applyUserConsumeLogFilter(LOG_DB.Table("logs"), userID, start, end).
		Select(userConsumeAggregateSelect).
		Scan(&totals).Error
	if err != nil {
		return UserConsumeTotals{}, err
	}
	return totals, nil
}

// GetUserConsumeBreakdown groups one user's consume records along dimension,
// ordered by quota descending. dimension must be "model", "group" or "token";
// anything else is rejected so no caller-supplied text reaches the SQL text.
// limit is clamped to 1..50 with a default of 10, bounding the response size.
func GetUserConsumeBreakdown(userID int, start, end int64, dimension string, limit int) ([]*UserConsumeBreakdownRow, error) {
	var dimensionColumn string
	switch dimension {
	case "model":
		dimensionColumn = "model_name"
	case "group":
		dimensionColumn = logGroupCol
	case "token":
		dimensionColumn = "token_name"
	default:
		return nil, fmt.Errorf("unsupported consume breakdown dimension %q", dimension)
	}
	if limit <= 0 {
		limit = userConsumeBreakdownDefaultLimit
	}
	if limit > userConsumeBreakdownMaxLimit {
		limit = userConsumeBreakdownMaxLimit
	}

	rows := make([]*UserConsumeBreakdownRow, 0)
	// Group by the selected alias: it is a plain identifier, so GORM's quoting
	// stays valid for the reserved "group" column on every supported database.
	err := applyUserConsumeLogFilter(LOG_DB.Table("logs"), userID, start, end).
		Select(dimensionColumn + " AS dimension, " + userConsumeAggregateSelect).
		Group("dimension").
		Order("quota DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AssistantUsageDaily stores one server-local day of assistant token usage per
// user. The row is upserted so concurrent runs only ever add to the counter.
type AssistantUsageDaily struct {
	Id               int   `json:"id"`
	UserId           int   `json:"user_id" gorm:"uniqueIndex:idx_assistant_usage_user_day,priority:1"`
	Day              int   `json:"day" gorm:"uniqueIndex:idx_assistant_usage_user_day,priority:2"`
	PromptTokens     int64 `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int64 `json:"completion_tokens" gorm:"default:0"`
	TotalTokens      int64 `json:"total_tokens" gorm:"default:0"`
	CreatedAt        int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt        int64 `json:"updated_at" gorm:"bigint"`
}

func (AssistantUsageDaily) TableName() string {
	return "assistant_usage_daily"
}

func (u *AssistantUsageDaily) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	u.CreatedAt = now
	u.UpdatedAt = now
	return nil
}

// GetAssistantDailyUsage returns the total assistant tokens one user consumed
// on the given day key.
func GetAssistantDailyUsage(userID, day int) (int64, error) {
	var total int64
	err := DB.Model(&AssistantUsageDaily{}).
		Where("user_id = ? AND day = ?", userID, day).
		Select("COALESCE(SUM(total_tokens), 0) AS total").
		Scan(&total).Error
	return total, err
}

// AddAssistantDailyUsage atomically adds one run's token usage to the user's
// daily counter.
func AddAssistantDailyUsage(userID, day int, promptTokens, completionTokens, totalTokens int64) error {
	if userID <= 0 || day <= 0 || totalTokens <= 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "day"}},
		DoUpdates: clause.Assignments(map[string]any{
			"prompt_tokens":     gorm.Expr("assistant_usage_daily.prompt_tokens + ?", promptTokens),
			"completion_tokens": gorm.Expr("assistant_usage_daily.completion_tokens + ?", completionTokens),
			"total_tokens":      gorm.Expr("assistant_usage_daily.total_tokens + ?", totalTokens),
			"updated_at":        common.GetTimestamp(),
		}),
	}).Create(&AssistantUsageDaily{
		UserId:           userID,
		Day:              day,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
	}).Error
}

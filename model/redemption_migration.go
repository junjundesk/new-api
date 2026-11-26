package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type RedemptionMigrationMode string

const (
	RedemptionMigrationExact RedemptionMigrationMode = "exact"
	RedemptionMigrationFuzzy RedemptionMigrationMode = "fuzzy"
)

var ErrRedemptionMigrationMode = errors.New("invalid redemption migration mode")

type RedemptionMigrationResult struct {
	Migrated  int64 `json:"migrated"`
	Unmatched int64 `json:"unmatched"`
}

// MigrateUncategorizedRedemptions assigns unused codes to existing denominations.
// Fuzzy matching only rounds down; an ambiguous denomination is never guessed.
func MigrateUncategorizedRedemptions(mode RedemptionMigrationMode) (RedemptionMigrationResult, error) {
	var result RedemptionMigrationResult
	if mode != RedemptionMigrationExact && mode != RedemptionMigrationFuzzy {
		return result, ErrRedemptionMigrationMode
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var categories []RedemptionCategory
		if err := lockForUpdate(tx).Select("id", "quota").Where("quota > 0 AND quota < ?", common.MaxQuota).
			Order("quota ASC, id ASC").Find(&categories).Error; err != nil {
			return err
		}
		// Keep eligibility in the UPDATE itself (a redeem may win the row lock first).
		// Legacy codes can have NULL timestamps, or a restored status despite a recorded redemption.
		eligible := unusedRedemptions(tx.Model(&Redemption{})).
			Where("category_id = 0 AND COALESCE(used_user_id, 0) = 0 AND COALESCE(redeemed_time, 0) = 0").
			Session(&gorm.Session{})
		for i, category := range categories {
			nextQuota := common.MaxQuota
			if i+1 < len(categories) {
				nextQuota = categories[i+1].Quota
			}
			if category.Quota == nextQuota || (i > 0 && category.Quota == categories[i-1].Quota) {
				continue // Same face value can have different bonus rules.
			}
			var match *gorm.DB
			if mode == RedemptionMigrationExact {
				match = eligible.Where("quota = ?", category.Quota)
			} else {
				match = eligible.Where("quota >= ? AND quota < ?", category.Quota, nextQuota)
			}
			updated := match.Updates(map[string]any{"category_id": category.Id, "quota": category.Quota})
			if updated.Error != nil {
				return updated.Error
			}
			result.Migrated += updated.RowsAffected
		}
		return eligible.Count(&result.Unmatched).Error
	})
	if err != nil {
		return RedemptionMigrationResult{}, err
	}
	return result, nil
}

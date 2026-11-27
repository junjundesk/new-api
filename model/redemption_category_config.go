package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

const redemptionCategoryConfigVersion = 1

var ErrRedemptionCategoryConfig = errors.New("invalid redemption category configuration")

type RedemptionCategoryConfigConflict struct{ Name string }

func (e *RedemptionCategoryConfigConflict) Error() string {
	return "existing redemption category has different settings: " + e.Name
}

// This portable format contains only category settings, never database IDs or redemption keys.
// Wallet visibility is site-local: importing a new category never publishes it.
// A missing bonus rule and an explicit 0% placeholder are distinct configurations.
type RedemptionCategoryConfig struct {
	Version    int                             `json:"version"`
	Categories []RedemptionCategoryConfigEntry `json:"categories"`
}

type RedemptionCategoryConfigEntry struct {
	Name         string `json:"name"`
	Quota        int    `json:"quota"`
	BonusRateBps *int   `json:"bonus_rate_bps,omitempty"`
}

type RedemptionCategoryImportResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
}

func ExportRedemptionCategoryConfig(ctx context.Context) (RedemptionCategoryConfig, error) {
	config := RedemptionCategoryConfig{Version: redemptionCategoryConfigVersion, Categories: []RedemptionCategoryConfigEntry{}}
	// A single query keeps the rule and its category in the same snapshot.
	err := DB.WithContext(ctx).Model(&RedemptionCategory{}).
		Select("redemption_categories.name, redemption_categories.quota, redemption_bonus_rules.bonus_rate_bps").
		Joins("LEFT JOIN redemption_bonus_rules ON redemption_bonus_rules.category_id = redemption_categories.id").
		Order("redemption_categories.quota ASC, redemption_categories.name ASC").Scan(&config.Categories).Error
	return config, err
}

// ImportRedemptionCategoryConfig only adds new categories. A name collision must
// not change the face value or live bonus of codes already issued at this site.
func ImportRedemptionCategoryConfig(ctx context.Context, config RedemptionCategoryConfig) (RedemptionCategoryImportResult, error) {
	var result RedemptionCategoryImportResult
	if config.Version != redemptionCategoryConfigVersion || config.Categories == nil {
		return result, ErrRedemptionCategoryConfig
	}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seen := make(map[string]bool, len(config.Categories))
		for _, entry := range config.Categories {
			category := RedemptionCategory{Name: entry.Name, Quota: entry.Quota}
			if err := category.validateDenomination(); err != nil {
				return err
			}
			if seen[category.Name] {
				return ErrRedemptionCategoryName
			}
			seen[category.Name] = true
			var rule RedemptionBonusRule
			if entry.BonusRateBps != nil {
				rule.BonusRateBps = *entry.BonusRateBps
				if _, err := rule.bonusQuota(category.Quota); err != nil {
					return err
				}
			}
			var existing RedemptionCategory
			lookup := lockForUpdate(tx).Where("name = ?", category.Name).Limit(1).Find(&existing)
			if lookup.Error != nil {
				return lookup.Error
			}
			if lookup.RowsAffected != 0 {
				if existing.Name != category.Name || existing.Quota != category.Quota {
					return &RedemptionCategoryConfigConflict{Name: category.Name}
				}
				var existingRule RedemptionBonusRule
				stored := lockForUpdate(tx).Where("category_id = ?", existing.Id).Limit(1).Find(&existingRule)
				if stored.Error != nil {
					return stored.Error
				}
				if (entry.BonusRateBps != nil) != (stored.RowsAffected != 0) || existingRule.BonusRateBps != rule.BonusRateBps {
					return &RedemptionCategoryConfigConflict{Name: category.Name}
				}
				result.Skipped++
				continue
			}
			if err := tx.Create(&category).Error; err != nil {
				return err
			}
			if entry.BonusRateBps != nil {
				rule.CategoryId = category.Id
				if err := tx.Create(&rule).Error; err != nil {
					return err
				}
			}
			result.Created++
		}
		return nil
	})
	if err != nil {
		return RedemptionCategoryImportResult{}, err
	}
	return result, nil
}

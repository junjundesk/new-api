package model

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type RedemptionCategory struct {
	Id    int    `json:"id"`
	Name  string `json:"name" gorm:"type:varchar(20);uniqueIndex;not null"`
	Quota int    `json:"quota" gorm:"not null"`
}

// RedemptionBonusRule grants a percentage of the face value as bonus when a code of the category is redeemed.
type RedemptionBonusRule struct {
	Id           int `json:"id"`
	CategoryId   int `json:"category_id" gorm:"uniqueIndex;not null"`
	BonusRateBps int `json:"bonus_rate_bps" gorm:"not null"` // 100 basis points = 1%.
}

var (
	ErrRedemptionName              = errors.New("invalid redemption name")
	ErrRedemptionQuota             = errors.New("invalid redemption face value")
	ErrRedemptionBonusRate         = errors.New("invalid redemption bonus rate")
	ErrRedemptionBonusRuleConflict = errors.New("redemption category already has a bonus rule")
	ErrRedemptionCategory          = errors.New("redemption category not found")
	ErrRedemptionCategoryName      = errors.New("redemption category name already exists")
	ErrRedemptionCategoryInUse     = errors.New("redemption category contains codes")
	ErrRedemptionAlreadyUsed       = errors.New("redemption code has already been used")
)

func GetRedemptionCategories() ([]RedemptionCategory, error) {
	var categories []RedemptionCategory
	err := DB.Order("quota ASC, id ASC").Find(&categories).Error
	return categories, err
}

// bonusQuota is evaluated against the issued face value inside the redemption transaction.
func (rule *RedemptionBonusRule) bonusQuota(quota int) (int, error) {
	if quota <= 0 || quota >= common.MaxQuota {
		return 0, ErrRedemptionQuota
	}
	if rule.BonusRateBps <= 0 || rule.BonusRateBps > common.MaxQuota {
		return 0, ErrRedemptionBonusRate
	}
	bonus, err := common.QuotaRoundStrict(float64(quota) * float64(rule.BonusRateBps) / 10000)
	if err != nil || bonus >= common.MaxQuota {
		return 0, ErrRedemptionBonusRate
	}
	return bonus, nil
}

func GetRedemptionBonusRules() ([]RedemptionBonusRule, error) {
	var rules []RedemptionBonusRule
	err := DB.Order("id ASC").Find(&rules).Error
	return rules, err
}

// ReplaceRedemptionBonusRules atomically swaps the whole rule set edited in the admin panel.
func ReplaceRedemptionBonusRules(rules []RedemptionBonusRule) ([]RedemptionBonusRule, error) {
	saved := make([]RedemptionBonusRule, 0, len(rules))
	err := DB.Transaction(func(tx *gorm.DB) error {
		seen := make(map[int]bool, len(rules))
		for _, rule := range rules {
			if seen[rule.CategoryId] {
				return ErrRedemptionBonusRuleConflict
			}
			seen[rule.CategoryId] = true
			category, err := getRedemptionCategoryForUpdate(tx, rule.CategoryId)
			if err != nil {
				return err
			}
			if _, err = rule.bonusQuota(category.Quota); err != nil {
				return err
			}
			saved = append(saved, RedemptionBonusRule{CategoryId: rule.CategoryId, BonusRateBps: rule.BonusRateBps})
		}
		if err := tx.Where("id > ?", 0).Delete(&RedemptionBonusRule{}).Error; err != nil {
			return err
		}
		if len(saved) == 0 {
			return nil
		}
		return tx.Create(&saved).Error
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// migrateRedemptionCategoryBonusRates moves rates that older releases stored on categories into bonus rules.
func migrateRedemptionCategoryBonusRates() error {
	if !DB.Migrator().HasColumn("redemption_categories", "bonus_rate_bps") {
		return nil
	}
	var legacy []RedemptionBonusRule
	err := DB.Table("redemption_categories").Select("id AS category_id, bonus_rate_bps").
		Where("bonus_rate_bps > ?", 0).Scan(&legacy).Error
	if err != nil || len(legacy) == 0 {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		categoryIds := make([]int, 0, len(legacy))
		for _, rule := range legacy {
			categoryIds = append(categoryIds, rule.CategoryId)
			var existing int64
			if err := tx.Model(&RedemptionBonusRule{}).Where("category_id = ?", rule.CategoryId).Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				continue
			}
			if err := tx.Create(&RedemptionBonusRule{CategoryId: rule.CategoryId, BonusRateBps: rule.BonusRateBps}).Error; err != nil {
				return err
			}
		}
		// Clear the legacy column so rules deleted later are not restored on the next start.
		return tx.Table("redemption_categories").Where("id IN ?", categoryIds).Update("bonus_rate_bps", 0).Error
	})
}

func getRedemptionCategoryForUpdate(tx *gorm.DB, id int) (*RedemptionCategory, error) {
	var category RedemptionCategory
	err := lockForUpdate(tx).Where("id = ?", id).First(&category).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRedemptionCategory
	}
	return &category, err
}

func (category *RedemptionCategory) SaveSettings() error {
	category.Name = strings.TrimSpace(category.Name)
	if category.Name == "" || utf8.RuneCountInString(category.Name) > 20 {
		return ErrRedemptionName
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if category.Id != 0 {
			stored, err := getRedemptionCategoryForUpdate(tx, category.Id)
			if err != nil {
				return err
			}
			category.Quota = stored.Quota
		}
		if category.Quota <= 0 || category.Quota >= common.MaxQuota {
			return ErrRedemptionQuota
		}
		var duplicates int64
		if err := tx.Model(&RedemptionCategory{}).Where("name = ? AND id <> ?", category.Name, category.Id).Count(&duplicates).Error; err != nil {
			return err
		}
		if duplicates > 0 {
			return ErrRedemptionCategoryName
		}
		if category.Id == 0 {
			return tx.Create(category).Error
		}
		return tx.Model(category).Select("name").Updates(category).Error
	})
}

func DeleteRedemptionCategory(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		category, err := getRedemptionCategoryForUpdate(tx, id)
		if err != nil {
			return err
		}
		var count int64
		if err = tx.Model(&Redemption{}).Where("category_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrRedemptionCategoryInUse
		}
		if err = tx.Where("category_id = ?", id).Delete(&RedemptionBonusRule{}).Error; err != nil {
			return err
		}
		return tx.Delete(category).Error
	})
}

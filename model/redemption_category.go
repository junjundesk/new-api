package model

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type RedemptionCategory struct {
	Id           int    `json:"id"`
	Name         string `json:"name" gorm:"type:varchar(20);uniqueIndex;not null"`
	Quota        int    `json:"quota" gorm:"not null"`
	BonusRateBps int    `json:"bonus_rate_bps" gorm:"not null;default:0"` // 100 basis points = 1%.
}

var (
	ErrRedemptionName          = errors.New("invalid redemption name")
	ErrRedemptionQuota         = errors.New("invalid redemption face value")
	ErrRedemptionBonusRate     = errors.New("invalid redemption bonus rate")
	ErrRedemptionCategory      = errors.New("redemption category not found")
	ErrRedemptionCategoryName  = errors.New("redemption category name already exists")
	ErrRedemptionCategoryInUse = errors.New("redemption category contains codes")
	ErrRedemptionAlreadyUsed   = errors.New("redemption code has already been used")
)

func GetRedemptionCategories() ([]RedemptionCategory, error) {
	var categories []RedemptionCategory
	err := DB.Order("quota ASC, id ASC").Find(&categories).Error
	return categories, err
}

// bonusQuota is evaluated against the issued face value inside the redemption transaction.
func (category *RedemptionCategory) bonusQuota(quota int) (int, error) {
	if quota <= 0 || quota >= common.MaxQuota {
		return 0, ErrRedemptionQuota
	}
	if category.BonusRateBps < 0 || category.BonusRateBps > common.MaxQuota {
		return 0, ErrRedemptionBonusRate
	}
	bonus, err := common.QuotaRoundStrict(float64(quota) * float64(category.BonusRateBps) / 10000)
	if err != nil {
		return 0, ErrRedemptionBonusRate
	}
	return bonus, nil
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
		if _, err := category.bonusQuota(category.Quota); err != nil {
			return err
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
		return tx.Model(category).Select("name", "bonus_rate_bps").Updates(category).Error
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
		return tx.Delete(category).Error
	})
}

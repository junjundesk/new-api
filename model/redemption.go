package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
)

type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	CategoryId   int            `json:"category_id" gorm:"index;not null;default:0"` // 0 includes codes issued before categories existed.
	BonusQuota   int            `json:"bonus_quota" gorm:"not null;default:0"`       // Actual grant at redemption time.
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

func SearchRedemptions(keyword, status string, categoryId *int, startIdx, num int) (redemptions []*Redemption, total int64, err error) {
	err = DB.Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&Redemption{})
		if categoryId != nil {
			query = query.Where("category_id = ?", *categoryId)
		}
		if keyword != "" {
			if id, err := strconv.Atoi(keyword); err == nil {
				query = query.Where("(id = ? OR name LIKE ?)", id, keyword+"%")
			} else {
				query = query.Where("name LIKE ?", keyword+"%")
			}
		}
		switch status {
		case "expired":
			query = query.Where("status = ? AND expired_time != 0 AND expired_time < ?", common.RedemptionCodeStatusEnabled, common.GetTimestamp())
		case strconv.Itoa(common.RedemptionCodeStatusEnabled):
			query = query.Where("status = ? AND (expired_time = 0 OR expired_time >= ?)", common.RedemptionCodeStatusEnabled, common.GetTimestamp())
		case strconv.Itoa(common.RedemptionCodeStatusDisabled), strconv.Itoa(common.RedemptionCodeStatusUsed):
			query = query.Where("status = ?", status)
		}
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		return query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	})
	return
}

func GetRedemptionById(id int) (*Redemption, error) {
	var redemption Redemption
	err := DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

// applyDenomination resolves the tier on the server so every code issued for it has the same face value.
func (redemption *Redemption) applyDenomination(tx *gorm.DB) error {
	redemption.Name = strings.TrimSpace(redemption.Name)
	if redemption.CategoryId != 0 {
		category, err := getRedemptionCategoryForUpdate(tx, redemption.CategoryId)
		if err != nil {
			return err
		}
		redemption.Quota = category.Quota
		if redemption.Name == "" {
			redemption.Name = category.Name
		}
	}
	if redemption.Name == "" || utf8.RuneCountInString(redemption.Name) > 20 {
		return ErrRedemptionName
	}
	if redemption.Quota <= 0 || redemption.Quota >= common.MaxQuota {
		return ErrRedemptionQuota
	}
	return nil
}

func (redemption *Redemption) IssueBatch() ([]string, error) {
	if redemption.Count < 1 || redemption.Count > 100 {
		return nil, errors.New("invalid redemption batch size")
	}
	keys := make([]string, redemption.Count)
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := redemption.applyDenomination(tx); err != nil {
			return err
		}
		now := common.GetTimestamp()
		codes := make([]Redemption, redemption.Count)
		for i := range codes {
			keys[i] = common.GetUUID()
			codes[i] = Redemption{
				UserId:      redemption.UserId,
				Key:         keys[i],
				Name:        redemption.Name,
				Quota:       redemption.Quota,
				CategoryId:  redemption.CategoryId,
				CreatedTime: now,
				ExpiredTime: redemption.ExpiredTime,
			}
		}
		// Keep each statement below SQLite's parameter limit, including on older installations.
		return tx.CreateInBatches(&codes, 50).Error
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func UpdateRedemptionDetails(id int, categoryId *int, name string, quota int, expiredTime int64, status *int) (*Redemption, error) {
	var redemption Redemption
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", id).First(&redemption).Error; err != nil {
			return err
		}
		if redemption.Status == common.RedemptionCodeStatusUsed {
			return ErrRedemptionAlreadyUsed
		}
		if status != nil {
			if *status != common.RedemptionCodeStatusEnabled && *status != common.RedemptionCodeStatusDisabled {
				return errors.New("invalid redemption status")
			}
			return tx.Model(&redemption).Update("status", *status).Error
		}
		if categoryId != nil {
			redemption.CategoryId = *categoryId
		}
		redemption.Name, redemption.Quota, redemption.ExpiredTime = name, quota, expiredTime
		if err := redemption.applyDenomination(tx); err != nil {
			return err
		}
		return tx.Model(&redemption).Select("name", "quota", "category_id", "expired_time").Updates(&redemption).Error
	})
	return &redemption, err
}

func Redeem(key string, userId int) (*Redemption, error) {
	if key == "" || userId <= 0 {
		return nil, ErrRedeemFailed
	}
	redemption := &Redemption{}
	common.RandomSleep()
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where(commonKeyCol+" = ?", key).First(redemption).Error; err != nil {
			return err
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return ErrRedemptionAlreadyUsed
		}
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			return errors.New("该兑换码已过期")
		}
		bonus := 0
		if redemption.CategoryId != 0 {
			var rules []RedemptionBonusRule
			if err := tx.Where("category_id = ?", redemption.CategoryId).Limit(1).Find(&rules).Error; err != nil {
				return err
			}
			if len(rules) == 1 {
				var err error
				if bonus, err = rules[0].bonusQuota(redemption.Quota); err != nil {
					return err
				}
			}
		}
		// Only the transaction that claims the code may credit either balance.
		result := tx.Model(redemption).Where("status = ?", common.RedemptionCodeStatusEnabled).
			Updates(map[string]interface{}{
				"redeemed_time": common.GetTimestamp(),
				"status":        common.RedemptionCodeStatusUsed,
				"used_user_id":  userId,
				"bonus_quota":   bonus,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRedemptionAlreadyUsed
		}
		if err := creditTopUpQuota(tx, userId, redemption.Quota, nil); err != nil {
			return err
		}
		if bonus > 0 {
			if _, err := grantBonusTx(tx, userId, int64(bonus), 0, BonusSourceRedemption, fmt.Sprintf("兑换码 #%d", redemption.Id)); err != nil {
				return err
			}
		}
		return RecordReferralRedemption(tx, userId, redemption.Id, int64(redemption.Quota))
	})
	if err != nil {
		common.SysError("redemption failed: " + err.Error())
		return nil, ErrRedeemFailed
	}
	syncCreditUserQuotaCache(userId, redemption.Quota, "redemption")
	message := fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id)
	if redemption.BonusQuota > 0 {
		message += fmt.Sprintf("，额外赠金 %s", logger.LogQuota(redemption.BonusQuota))
	}
	RecordLog(userId, LogTypeTopup, message)
	return redemption, nil
}

func DeleteRedemptionById(id int) error {
	redemption, err := GetRedemptionById(id)
	if err != nil {
		return err
	}
	return DB.Delete(redemption).Error
}

func DeleteInvalidRedemptions(categoryId *int) (int64, error) {
	query := DB.Where("(status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?))", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, common.GetTimestamp())
	if categoryId != nil {
		query = query.Where("category_id = ?", *categoryId)
	}
	result := query.Delete(&Redemption{})
	return result.RowsAffected, result.Error
}

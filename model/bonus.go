package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// Bonus grant statuses.
const (
	BonusStatusActive  = "active"
	BonusStatusUsed    = "used"
	BonusStatusExpired = "expired"
	BonusStatusRevoked = "revoked"
)

// Bonus grant sources.
const (
	BonusSourceAdmin  = "admin"
	BonusSourceSignup = "signup"
	BonusSourceAll    = "admin_all"
)

// UserBonusGrant is a single expiring bonus credit for a user. Grants are
// consumed oldest-expiring-first so no credit is wasted. AmountTotal == 0
// means the grant is exhausted; expiry is enforced on read.
type UserBonusGrant struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"index;index:idx_bonus_user_status,priority:1"`
	AmountTotal int64  `json:"amount_total" gorm:"type:bigint;not null;default:0"`
	AmountUsed  int64  `json:"amount_used" gorm:"type:bigint;not null;default:0"`
	ExpireTime  int64  `json:"expire_time" gorm:"bigint;index"` // 0 = never expires
	Status      string `json:"status" gorm:"type:varchar(32);index;index:idx_bonus_user_status,priority:2"`
	Source      string `json:"source" gorm:"type:varchar(32);default:'admin'"`
	Remark      string `json:"remark" gorm:"type:varchar(255);default:''"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

func (b *UserBonusGrant) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	b.CreatedAt = now
	b.UpdatedAt = now
	return nil
}

func (b *UserBonusGrant) BeforeUpdate(tx *gorm.DB) error {
	b.UpdatedAt = common.GetTimestamp()
	return nil
}

// Remaining returns the unconsumed amount of this grant.
func (b *UserBonusGrant) Remaining() int64 {
	remaining := b.AmountTotal - b.AmountUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ExpireTimeOrZero normalizes an expiry timestamp; non-positive means never.
func normalizeBonusExpireTime(expireTime int64) int64 {
	if expireTime <= 0 {
		return 0
	}
	return expireTime
}

// ---------------------------------------------------------------------------
// Grant lifecycle
// ---------------------------------------------------------------------------

// GrantBonus creates a bonus grant for a user. amount must be > 0.
// expireTime <= 0 means the grant never expires.
func GrantBonus(userId int, amount int64, expireTime int64, source string, remark string) (*UserBonusGrant, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if amount <= 0 {
		return nil, errors.New("bonus amount must be > 0")
	}
	if source == "" {
		source = BonusSourceAdmin
	}
	grant := &UserBonusGrant{
		UserId:      userId,
		AmountTotal: amount,
		AmountUsed:  0,
		ExpireTime:  normalizeBonusExpireTime(expireTime),
		Status:      BonusStatusActive,
		Source:      source,
		Remark:      remark,
	}
	if err := DB.Create(grant).Error; err != nil {
		return nil, err
	}
	return grant, nil
}

// GetActiveBonusRemaining returns the sum of unexpired remaining bonus quota.
func GetActiveBonusRemaining(userId int) (int64, error) {
	if userId <= 0 {
		return 0, nil
	}
	now := GetDBTimestamp()
	var total int64
	err := DB.Model(&UserBonusGrant{}).
		Where("user_id = ? AND status = ? AND (expire_time = 0 OR expire_time > ?)",
			userId, BonusStatusActive, now).
		Select("COALESCE(SUM(amount_total - amount_used), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if total < 0 {
		return 0, nil
	}
	return total, nil
}

// ListActiveBonusGrants returns the user's unexpired grants, soonest expiry first.
func ListActiveBonusGrants(userId int) ([]UserBonusGrant, error) {
	now := GetDBTimestamp()
	var grants []UserBonusGrant
	err := DB.Where("user_id = ? AND status = ? AND (expire_time = 0 OR expire_time > ?) AND amount_used < amount_total",
		userId, BonusStatusActive, now).
		Order("CASE WHEN expire_time = 0 THEN 1 ELSE 0 END, expire_time asc, id asc").
		Find(&grants).Error
	return grants, err
}

// ListUserBonusGrants returns all grants for admin display.
func ListUserBonusGrants(userId int) ([]UserBonusGrant, error) {
	var grants []UserBonusGrant
	err := DB.Where("user_id = ?", userId).Order("id desc").Find(&grants).Error
	return grants, err
}

// ExpireDueBonusGrants marks elapsed grants as expired. Returns affected rows.
func ExpireDueBonusGrants() (int64, error) {
	now := GetDBTimestamp()
	res := DB.Model(&UserBonusGrant{}).
		Where("status = ? AND expire_time != 0 AND expire_time <= ?", BonusStatusActive, now).
		Update("status", BonusStatusExpired)
	return res.RowsAffected, res.Error
}

// ---------------------------------------------------------------------------
// Consumption (oldest-expiring-first, idempotent per request)
// ---------------------------------------------------------------------------

// BonusPreConsumeResult reports the outcome of a bonus pre-consume.
type BonusPreConsumeResult struct {
	AmountTotal     int64
	AmountUsedAfter int64
	GrantId         int
	PreConsumed     int64
}

// PreConsumeUserBonus pre-consumes amount from the user's active grants,
// consuming the soonest-to-expire grant first. The whole operation is atomic
// and idempotent on requestId.
func PreConsumeUserBonus(requestId string, userId int, amount int64) (*BonusPreConsumeResult, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	now := GetDBTimestamp()
	result := &BonusPreConsumeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing BonusPreConsumeRecord
		q := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected > 0 {
			if existing.Status == "refunded" {
				return errors.New("bonus pre-consume already refunded")
			}
			result.PreConsumed = existing.PreConsumed
			result.GrantId = existing.UserBonusGrantId
			if total, err := bonusGrantTotalTx(tx, existing.UserBonusGrantId); err == nil {
				result.AmountTotal = total
			}
			return nil
		}

		remaining, allocations, amountTotal, usedAfter, err := consumeBonusGrantsTx(tx, userId, amount, now)
		if err != nil {
			return err
		}
		if remaining > 0 {
			return fmt.Errorf("bonus quota insufficient, need=%d short=%d", amount, remaining)
		}
		result.AmountTotal = amountTotal
		result.AmountUsedAfter = usedAfter

		record := &BonusPreConsumeRecord{
			RequestId:        requestId,
			UserId:           userId,
			UserBonusGrantId: allocations[0].GrantId,
			PreConsumed:      amount,
			Status:           "consumed",
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		if err := createBonusAllocationsTx(tx, requestId, allocations); err != nil {
			return err
		}
		result.GrantId = allocations[0].GrantId
		result.PreConsumed = amount
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// bonusAllocation records how much of a request's pre-consume came from a
// specific grant, so refunds restore every grant exactly.
type bonusAllocation struct {
	GrantId int
	Amount  int64
}

// consumeBonusGrantsTx consumes amount across the user's active grants
// (soonest expiry first). It returns the still-unconsumed remainder (0 on
// success), the per-grant allocation, and the first grant's totals for
// reporting.
func consumeBonusGrantsTx(tx *gorm.DB, userId int, amount int64, now int64) (int64, []bonusAllocation, int64, int64, error) {
	var grants []UserBonusGrant
	if err := lockForUpdate(tx).
		Where("user_id = ? AND status = ? AND (expire_time = 0 OR expire_time > ?)", userId, BonusStatusActive, now).
		Order("CASE WHEN expire_time = 0 THEN 1 ELSE 0 END, expire_time asc, id asc").
		Find(&grants).Error; err != nil {
		return amount, nil, 0, 0, err
	}

	remaining := amount
	var allocations []bonusAllocation
	var firstTotal, firstUsedAfter int64
	for _, candidate := range grants {
		if remaining <= 0 {
			break
		}
		grant := candidate
		available := grant.Remaining()
		if available <= 0 {
			continue
		}
		take := available
		if take > remaining {
			take = remaining
		}
		grant.AmountUsed += take
		if grant.AmountUsed >= grant.AmountTotal {
			grant.Status = BonusStatusUsed
		}
		if err := tx.Save(&grant).Error; err != nil {
			return amount, nil, 0, 0, err
		}
		allocations = append(allocations, bonusAllocation{GrantId: grant.Id, Amount: take})
		if len(allocations) == 1 {
			firstTotal = grant.AmountTotal
			firstUsedAfter = grant.AmountUsed
		}
		remaining -= take
	}
	return remaining, allocations, firstTotal, firstUsedAfter, nil
}

func createBonusAllocationsTx(tx *gorm.DB, requestId string, allocations []bonusAllocation) error {
	for _, a := range allocations {
		row := &BonusPreConsumeAllocation{
			RequestId: requestId,
			GrantId:   a.GrantId,
			Amount:    a.Amount,
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
	}
	return nil
}

// ApplyUserBonusDelta settles a bonus pre-consume identified by requestId.
// A positive delta consumes additional bonus (soonest-expiring-first); a
// negative delta returns quota across the grants that funded the request.
func ApplyUserBonusDelta(requestId string, delta int64) error {
	if delta == 0 {
		return nil
	}
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	now := GetDBTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		var record BonusPreConsumeRecord
		if err := tx.Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if delta > 0 {
			remaining, allocations, _, _, err := consumeBonusGrantsTx(tx, record.UserId, delta, now)
			if err != nil {
				return err
			}
			if remaining > 0 {
				return fmt.Errorf("bonus quota insufficient, need=%d short=%d", delta, remaining)
			}
			return createBonusAllocationsTx(tx, requestId, allocations)
		}
		return refundBonusAllocationsTx(tx, requestId, -delta, true)
	})
}

// refundBonusAllocationsTx returns amount to the grants that funded requestId.
// desc=true walks the allocation list newest-first (used to reverse a top-up
// reserve); desc=false walks it oldest-first (used to reverse the original
// pre-consume). The two orderings are disjoint because a request's allocations
// partition exactly into its original pre-consume followed by later reserves.
func refundBonusAllocationsTx(tx *gorm.DB, requestId string, amount int64, desc bool) error {
	order := "id asc"
	if desc {
		order = "id desc"
	}
	var allocations []BonusPreConsumeAllocation
	if err := tx.Where("request_id = ?", requestId).Order(order).Find(&allocations).Error; err != nil {
		return err
	}
	remaining := amount
	for _, alloc := range allocations {
		if remaining <= 0 {
			break
		}
		restore := alloc.Amount
		if restore > remaining {
			restore = remaining
		}
		var grant UserBonusGrant
		if err := lockForUpdate(tx).Where("id = ?", alloc.GrantId).First(&grant).Error; err != nil {
			return err
		}
		grant.AmountUsed -= restore
		if grant.AmountUsed < 0 {
			grant.AmountUsed = 0
		}
		if grant.Status == BonusStatusUsed && grant.AmountUsed < grant.AmountTotal {
			grant.Status = BonusStatusActive
		}
		if err := tx.Save(&grant).Error; err != nil {
			return err
		}
		remaining -= restore
	}
	if remaining > 0 {
		return fmt.Errorf("bonus refund exceeds allocated amount for request %s", requestId)
	}
	return nil
}

// RefundBonusPreConsume is idempotent and returns a pre-consume by requestId,
// restoring every grant that funded the request.
func RefundBonusPreConsume(requestId string) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record BonusPreConsumeRecord
		if err := lockForUpdate(tx).Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return nil
		}
		if record.PreConsumed > 0 {
			if err := refundBonusAllocationsTx(tx, requestId, record.PreConsumed, false); err != nil {
				return err
			}
		}
		record.Status = "refunded"
		return tx.Save(&record).Error
	})
}

func bonusGrantTotalTx(tx *gorm.DB, grantId int) (int64, error) {
	var grant UserBonusGrant
	if err := tx.Where("id = ?", grantId).First(&grant).Error; err != nil {
		return 0, err
	}
	return grant.AmountTotal, nil
}

// BonusPreConsumeRecord stores idempotent bonus pre-consume operations per request.
type BonusPreConsumeRecord struct {
	Id               int    `json:"id"`
	RequestId        string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId           int    `json:"user_id" gorm:"index"`
	UserBonusGrantId int    `json:"user_bonus_grant_id" gorm:"index"`
	PreConsumed      int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	Status           string `json:"status" gorm:"type:varchar(32);index"` // consumed/refunded
	CreatedAt        int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt        int64  `json:"updated_at" gorm:"bigint;index"`
}

// BonusPreConsumeAllocation records how much of one request's bonus pre-consume
// came from a specific grant, so refunds restore each grant exactly even when a
// single request spans multiple grants.
type BonusPreConsumeAllocation struct {
	Id        int    `json:"id"`
	RequestId string `json:"request_id" gorm:"type:varchar(64);index"`
	GrantId   int    `json:"grant_id" gorm:"index"`
	Amount    int64  `json:"amount" gorm:"type:bigint;not null;default:0"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
}

func (a *BonusPreConsumeAllocation) BeforeCreate(tx *gorm.DB) error {
	a.CreatedAt = common.GetTimestamp()
	return nil
}

func (r *BonusPreConsumeRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *BonusPreConsumeRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

// CleanupBonusPreConsumeRecords removes old idempotency records.
func CleanupBonusPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	cutoff := common.GetTimestamp() - olderThanSeconds
	res := DB.Where("updated_at < ?", cutoff).Delete(&BonusPreConsumeRecord{})
	return res.RowsAffected, res.Error
}

// GetAllActiveUsersForBonus returns every non-deleted user (id/username/email/
// display name) so an admin can grant bonus to all of them at once.
func GetAllActiveUsersForBonus() ([]User, error) {
	var users []User
	err := DB.Model(&User{}).
		Select("id", "username", "display_name", "email").
		Find(&users).Error
	return users, err
}

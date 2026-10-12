package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

// Referral charge kinds.
const (
	ReferralKindInviter    = 1 // inviter commission on an invitee top-up
	ReferralKindInvitee    = 2 // invitee's own first-top-up reward
	ReferralKindSelfPayout = 3 // user's own conversion of pending reward to balance
)

// Referral charge statuses.
const (
	ReferralStatusPending  = 0 // recorded, not yet paid out
	ReferralStatusSettled  = 1 // bonus granted (or moved to balance for self-payout)
	ReferralStatusCanceled = 2 // superseded or failed permanently
)

// RefKeyKind prefixes distinguish the source of a referral charge so the same
// numeric id from two tables can never collide inside RefKey.
const (
	ReferralRefTopUp      = "topup"
	ReferralRefRedemption = "redemption"
)

// ReferralCharge records one referral reward owed to a user. Rows are written
// inside the payment-bound transaction with RefKey as the idempotency key, and
// are paid out later by the settlement task, so a slow bonus grant never blocks
// a payment callback or redemption.
type ReferralCharge struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"index:idx_referral_user_status,priority:1"` // recipient
	InviterId   int    `json:"inviter_id" gorm:"index"`                                  // 0 for invitee/self rows
	InviteeId   int    `json:"invitee_id" gorm:"index"`                                  // paying user
	Kind        int    `json:"kind" gorm:"index"`
	RefKey      string `json:"ref_key" gorm:"type:varchar(64);uniqueIndex"`
	RefKind     string `json:"ref_kind" gorm:"type:varchar(16)"`
	IsFirst     bool   `json:"is_first"`                      // invitee's first successful top-up
	CountsValid bool   `json:"counts_valid"`                  // this event promoted the inviter's tier
	RateBps     int    `json:"rate_bps"`                      // frozen at settlement time
	BaseQuota   int64  `json:"base_quota" gorm:"type:bigint"` // quota credited to the invitee
	Amount      int64  `json:"amount" gorm:"type:bigint"`     // reward owed, quota units
	Status      int    `json:"status" gorm:"index:idx_referral_user_status,priority:2"`
	GrantId     int    `json:"grant_id"` // user_bonus_grants.id once paid
	CreateTime  int64  `json:"create_time" gorm:"bigint;index"`
	SettleTime  int64  `json:"settle_time" gorm:"bigint"`
	Remark      string `json:"remark" gorm:"type:varchar(255);default:''"`
}

// BuildReferralRefKey is the single place that formats the idempotency key, so
// every caller produces an identical key for the same source event.
func BuildReferralRefKey(refKind string, refId int) string {
	return fmt.Sprintf("%s:%d", refKind, refId)
}

// RecordReferralTopUp records the referral rewards owed for one successfully
// credited top-up. It runs inside the caller's transaction so the charge and
// the wallet credit commit together.
//
// baseQuota is the quota actually credited to the invitee. It is the reward
// basis because every payment channel converts its own Amount/Money into quota
// differently (see getUserTotalRechargeQuotaUnits), so the credited quota is
// the only figure comparable across channels.
//
// countAsValid is false for sources that must not promote the inviter's tier.
// It is ignored for the invitee's own first-top-up reward, which depends only
// on whether this is genuinely the invitee's first successful top-up.
//
// Callers already hold a row lock on the payment or redemption record they are
// crediting, so the idempotency pre-check below cannot race with a concurrent
// callback for the same source event.
func RecordReferralTopUp(tx *gorm.DB, inviteeId int, refKind string, refId int, baseQuota int64, countAsValid bool) error {
	setting := operation_setting.GetReferralSetting()
	if !setting.Enabled || inviteeId <= 0 || baseQuota <= 0 {
		return nil
	}

	var invitee User
	if err := tx.Select("id", "inviter_id").Where("id = ?", inviteeId).First(&invitee).Error; err != nil {
		return err
	}
	inviterId := invitee.InviterId
	if inviterId <= 0 || inviterId == inviteeId {
		return nil // no inviter, or self-invite: nothing to pay
	}

	// The invitee reward is granted only once, on the first recorded top-up.
	var existing int64
	if err := tx.Model(&ReferralCharge{}).
		Where("invitee_id = ? AND kind = ?", inviteeId, ReferralKindInvitee).
		Count(&existing).Error; err != nil {
		return err
	}
	isFirst := existing == 0

	now := common.GetTimestamp()
	refKey := BuildReferralRefKey(refKind, refId)

	// Idempotency is enforced by a pre-check rather than by reacting to a
	// unique-index violation, because duplicate-key error text differs across
	// SQLite, MySQL and PostgreSQL.
	already, err := referralChargeExistsTx(tx, refKey)
	if err != nil {
		return err
	}
	if already {
		return nil
	}
	// The invitee row may be absent (reward disabled), so the inviter row
	// carries its own namespaced key instead of reusing refKey.
	inviterKey := fmt.Sprintf("%s|inviter:%d", refKey, inviterId)
	inviterAlready, err := referralChargeExistsTx(tx, inviterKey)
	if err != nil {
		return err
	}
	if inviterAlready {
		return nil
	}

	if isFirst && setting.InviteeFirstTopUpEnabled && setting.InviteeFirstTopUpRateBps > 0 {
		amount, err := referralShare(baseQuota, setting.InviteeFirstTopUpRateBps, setting.InviteeFirstTopUpMaxQuota)
		if err != nil {
			return err
		}
		if amount > 0 {
			row := &ReferralCharge{
				UserId:     inviteeId,
				InviteeId:  inviteeId,
				Kind:       ReferralKindInvitee,
				RefKey:     refKey,
				RefKind:    refKind,
				IsFirst:    true,
				RateBps:    setting.InviteeFirstTopUpRateBps,
				BaseQuota:  baseQuota,
				Amount:     amount,
				Status:     ReferralStatusPending,
				CreateTime: now,
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
	}

	if !setting.InviterEnabled {
		return nil
	}

	// The rate uses the valid-invitee count including this one, so an invitee
	// that reaches a tier floor pays out at the new rate immediately.
	countsValid := isFirst && countAsValid
	validCount, err := countValidInviteesTx(tx, inviterId)
	if err != nil {
		return err
	}
	if countsValid {
		validCount++
	}
	rateBps := setting.RateBpsForInviteeCount(validCount)
	if rateBps <= 0 {
		return nil
	}
	amount, err := referralShare(baseQuota, rateBps, setting.InviterMaxQuota)
	if err != nil {
		return err
	}
	if amount <= 0 {
		return nil
	}

	inviterRow := &ReferralCharge{
		UserId:      inviterId,
		InviterId:   inviterId,
		InviteeId:   inviteeId,
		Kind:        ReferralKindInviter,
		RefKey:      inviterKey,
		RefKind:     refKind,
		IsFirst:     isFirst,
		CountsValid: countsValid,
		RateBps:     rateBps,
		BaseQuota:   baseQuota,
		Amount:      amount,
		Status:      ReferralStatusPending,
		CreateTime:  now,
	}
	return tx.Create(inviterRow).Error
}

// referralSourceForTopUp maps a completed top-up order to its referral credit
// source. Every channel credits through creditTopUpQuotaFrom, so this is the
// one place that decides which payment providers count as a "valid" invitee
// when the admin excludes redemption codes from tier progress.
func referralSourceForTopUp(topUp *TopUp) *topUpCreditSource {
	if topUp == nil || topUp.Id <= 0 {
		return nil
	}
	// Balance-funded orders are not new external revenue, so they never
	// promote the inviter's tier.
	countAsValid := topUp.PaymentProvider != PaymentProviderBalance &&
		topUp.PaymentMethod != PaymentMethodBalance
	return &topUpCreditSource{
		refKind:      ReferralRefTopUp,
		refId:        topUp.Id,
		countAsValid: countAsValid,
	}
}

// RecordReferralRedemption records the referral rewards owed for one redeemed
// redemption code. baseQuota is the quota the code credited. Codes are never
// treated as new external revenue for tier progress unless the admin opts in.
func RecordReferralRedemption(tx *gorm.DB, userId int, redemptionId int, baseQuota int64) error {
	setting := operation_setting.GetReferralSetting()
	if !setting.Enabled {
		return nil
	}
	return RecordReferralTopUp(tx, userId, ReferralRefRedemption, redemptionId, baseQuota, setting.CountRedeemAsValid)
}

func referralChargeExistsTx(tx *gorm.DB, refKey string) (bool, error) {
	var count int64
	if err := tx.Model(&ReferralCharge{}).Where("ref_key = ?", refKey).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// referralShare converts a basis-point share of a base amount into quota,
// applying the optional per-payout cap. All rounding and saturation go through
// common.QuotaRoundChecked, so a referral reward can never invent quota through
// a bare int cast and any clamp is logged like every other quota conversion.
func referralShare(baseQuota int64, rateBps int, maxQuota int64) (int64, error) {
	if baseQuota <= 0 || rateBps <= 0 {
		return 0, nil
	}
	amount, clamp := common.QuotaRoundChecked(float64(baseQuota) * float64(rateBps) / 10000)
	if clamp != nil {
		common.SysError(fmt.Sprintf("referral reward saturated: base=%d rate_bps=%d kind=%s", baseQuota, rateBps, clamp.Kind))
	}
	if amount <= 0 {
		return 0, nil
	}
	if maxQuota > 0 && int64(amount) > maxQuota {
		return maxQuota, nil
	}
	return int64(amount), nil
}

// countValidInviteesTx counts the inviter's valid invitees: invitees whose
// first top-up was recorded as tier-promoting. An invitee whose first top-up
// came from a non-counting source (a redemption code, when the admin excluded
// those) never promotes the inviter's tier.
func countValidInviteesTx(tx *gorm.DB, inviterId int) (int, error) {
	var ids []int
	if err := tx.Model(&ReferralCharge{}).
		Distinct("invitee_id").
		Where("inviter_id = ? AND kind = ? AND counts_valid = ?", inviterId, ReferralKindInviter, true).
		Pluck("invitee_id", &ids).Error; err != nil {
		return 0, err
	}
	return len(ids), nil
}

// CountValidInvitees is the read-only variant used by the invite page.
func CountValidInvitees(inviterId int) (int, error) {
	return countValidInviteesTx(DB, inviterId)
}

// referralBatchSize bounds how many charges one settlement pass handles.
const referralBatchSize = 200

// SettlePendingReferralCharges pays out a batch of pending charges. Returns
// how many charges were settled.
//
// The two reward tracks pay out differently. Inviter commissions move into the
// claimable referral balance (users.aff_quota), which the invite page can move
// to the main balance. Invitee first-top-up rewards are spendable bonus only
// and never touch aff_quota. Both are backed by an expiring bonus grant, so a
// reward that expires unspent also stops being claimable.
//
// Each charge is claimed with a compare-and-swap on status before anything is
// paid, so two nodes can never pay the same charge twice.
func SettlePendingReferralCharges() (int, error) {
	setting := operation_setting.GetReferralSetting()
	if !setting.Enabled {
		return 0, nil
	}

	var rows []ReferralCharge
	if err := DB.Where("status = ?", ReferralStatusPending).
		Order("id").
		Limit(referralBatchSize).
		Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	expireTime, err := referralBonusExpireTime(setting.BonusDuration)
	if err != nil {
		return 0, err
	}

	settled := 0
	for i := range rows {
		row := rows[i]
		if row.Amount <= 0 || row.UserId <= 0 {
			if err := markReferralCharge(DB, row.Id, ReferralStatusCanceled, 0, "empty amount"); err != nil {
				return settled, err
			}
			continue
		}
		// Self-invite rows are rejected at write time; this is the second gate
		// in case an inviter_id changed after the charge was recorded.
		if row.InviterId > 0 && row.InviterId == row.InviteeId {
			if err := markReferralCharge(DB, row.Id, ReferralStatusCanceled, 0, "self invite"); err != nil {
				return settled, err
			}
			continue
		}

		claimed, err := claimReferralCharge(row.Id)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to claim referral charge %d: %s", row.Id, err.Error()))
			return settled, err
		}
		if !claimed {
			continue // another node took it
		}

		// The grant is what makes the amount expire on schedule. It is created
		// before the claimable balance is credited, so a crash between the two
		// steps leaves an unspent grant rather than a claimable balance with no
		// expiry behind it.
		grant, err := GrantBonus(row.UserId, row.Amount, expireTime, BonusSourceReferral, referralRemark(&row))
		if err != nil {
			// The charge is already claimed, so another pass will not retry it.
			// Park it as canceled with the reason instead of leaving it pending
			// forever or risking a double payout.
			common.SysError(fmt.Sprintf("failed to grant referral reward for charge %d: %s", row.Id, err.Error()))
			_ = markReferralCharge(DB, row.Id, ReferralStatusCanceled, 0, "grant failed: "+err.Error())
			continue
		}

		// Inviter commissions are the only track that becomes claimable.
		// aff_history tracks the lifetime total and is never reduced.
		if row.Kind == ReferralKindInviter {
			if err := DB.Model(&User{}).Where("id = ?", row.UserId).Updates(map[string]interface{}{
				"aff_quota":   gorm.Expr("aff_quota + ?", row.Amount),
				"aff_history": gorm.Expr("aff_history + ?", row.Amount),
			}).Error; err != nil {
				common.SysError(fmt.Sprintf("failed to credit aff_quota for charge %d: %s", row.Id, err.Error()))
				return settled, err
			}
		}

		if err := markReferralCharge(DB, row.Id, ReferralStatusSettled, grant.Id, ""); err != nil {
			common.SysError(fmt.Sprintf("failed to mark referral charge %d settled: %s", row.Id, err.Error()))
			return settled, err
		}
		settled++
	}
	if settled > 0 {
		common.SysLog(fmt.Sprintf("referral settlement: %d charges paid out", settled))
	}
	return settled, nil
}

// claimReferralCharge atomically flips one pending charge to settled, reporting
// whether this caller won the race. Status settled here means "in flight"; the
// grant id is written once the grant exists.
func claimReferralCharge(id int) (bool, error) {
	result := DB.Model(&ReferralCharge{}).
		Where("id = ? AND status = ?", id, ReferralStatusPending).
		Update("status", ReferralStatusSettled)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func markReferralCharge(tx *gorm.DB, id int, status int, grantId int, remark string) error {
	updates := map[string]interface{}{
		"status":      status,
		"settle_time": common.GetTimestamp(),
	}
	if grantId > 0 {
		updates["grant_id"] = grantId
	}
	if remark != "" {
		updates["remark"] = remark
	}
	return tx.Model(&ReferralCharge{}).Where("id = ?", id).Updates(updates).Error
}

func referralRemark(row *ReferralCharge) string {
	if row.Kind == ReferralKindInviter {
		return "邀请返利"
	}
	return "首充返利"
}

// referralBonusExpireTime resolves the configured validity window into an
// absolute expiry. A blank duration or "0" means the reward never expires,
// which is the shipped default. Any other unparsable duration is an error
// rather than a silent permanent grant, because a misconfiguration would
// otherwise hand out never-expiring quota.
func referralBonusExpireTime(duration string) (int64, error) {
	trimmed := strings.TrimSpace(duration)
	if trimmed == "" || trimmed == "0" {
		return 0, nil
	}
	d, err := ParseBonusDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid referral bonus duration %q: %w", duration, err)
	}
	return common.GetTimestamp() + int64(d.Seconds()), nil
}

// ReferralSummary is the invite page payload for one user.
type ReferralSummary struct {
	PendingQuota     int64 `json:"pending_quota"`      // recorded, not yet granted
	TotalQuota       int64 `json:"total_quota"`        // granted to date
	ValidInvitees    int   `json:"valid_invitees"`     // invitees that produced a charge
	RegisteredCount  int   `json:"registered_count"`   // users.inviter_id = me
	CurrentRateBps   int   `json:"current_rate_bps"`   // rate applying right now
	NextTierInvitees int   `json:"next_tier_invitees"` // 0 when at the top tier
	NextTierRateBps  int   `json:"next_tier_rate_bps"`
}

// GetReferralSummary aggregates the invite page numbers for one user.
func GetReferralSummary(userId int) (*ReferralSummary, error) {
	summary := &ReferralSummary{}

	var pending, total int64
	if err := DB.Model(&ReferralCharge{}).
		Where("user_id = ? AND status = ?", userId, ReferralStatusPending).
		Select("COALESCE(SUM(amount), 0)").Scan(&pending).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&ReferralCharge{}).
		Where("user_id = ? AND status = ?", userId, ReferralStatusSettled).
		Select("COALESCE(SUM(amount), 0)").Scan(&total).Error; err != nil {
		return nil, err
	}
	summary.PendingQuota = pending
	summary.TotalQuota = total

	valid, err := CountValidInvitees(userId)
	if err != nil {
		return nil, err
	}
	summary.ValidInvitees = valid

	var registered int64
	if err := DB.Model(&User{}).Where("inviter_id = ?", userId).Count(&registered).Error; err != nil {
		return nil, err
	}
	summary.RegisteredCount = int(registered)

	setting := operation_setting.GetReferralSetting()
	summary.CurrentRateBps = setting.RateBpsForInviteeCount(valid)
	for _, tier := range setting.SortedInviterTiers() {
		if tier.RateBps <= 0 || tier.MinInvitees <= valid {
			continue
		}
		summary.NextTierInvitees = tier.MinInvitees
		summary.NextTierRateBps = tier.RateBps
		break
	}
	return summary, nil
}

// ListReferralCharges returns one page of a user's referral history.
func ListReferralCharges(userId int, offset int, limit int) ([]ReferralCharge, int64, error) {
	var rows []ReferralCharge
	var total int64
	query := DB.Model(&ReferralCharge{}).Where("user_id = ?", userId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

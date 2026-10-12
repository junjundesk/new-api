package model

import (
	"errors"
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

	// The invitee reward is paid only once. "First" here means no invitee
	// reward row has been recorded yet; it also fills the informational IsFirst
	// flag. Tier promotion is deliberately not derived from it, because this
	// row is only written while the invitee reward is enabled.
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

	// A counting event promotes the invitee once, on the invitee's first such
	// event. The decision must come from the recorded promotion rather than
	// from isFirst: the invitee-reward row is only written while the invitee
	// reward is enabled, so deriving it from that row would re-promote the same
	// invitee on every later top-up and inflate the count the rate is looked up
	// with.
	countsValid := countAsValid
	if countsValid {
		counted, err := inviteeAlreadyCountedTx(tx, inviterId, inviteeId)
		if err != nil {
			return err
		}
		countsValid = !counted
	}
	// The rate uses the valid-invitee count including this one, so an invitee
	// that reaches a tier floor pays out at the new rate immediately.
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

// countValidInviteesTx counts the inviter's valid invitees: invitees with a
// recorded tier-promoting event. An invitee becomes valid on their first
// counting top-up, so an invitee whose first event came from a non-counting
// source (a redemption code, when the admin excluded those) still becomes
// valid on a later online payment.
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

// inviteeAlreadyCountedTx reports whether this invitee already promoted the
// inviter's tier. The recorded promotion is the durable fact behind "once per
// invitee": it survives the invitee reward being toggled off, unlike a
// freshly computed first-event judgement.
func inviteeAlreadyCountedTx(tx *gorm.DB, inviterId int, inviteeId int) (bool, error) {
	var count int64
	if err := tx.Model(&ReferralCharge{}).
		Where("inviter_id = ? AND invitee_id = ? AND kind = ? AND counts_valid = ?",
			inviterId, inviteeId, ReferralKindInviter, true).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// referralBatchSize bounds how many charges one settlement pass handles.
const referralBatchSize = 200

// SettlePendingReferralCharges pays out a batch of pending charges. Returns
// how many charges were settled.
//
// The two reward tracks pay out through different channels. An inviter
// commission is money-like: it is credited straight to the claimable referral
// balance (users.aff_quota) and deliberately does not also become spendable
// bonus. Bonus is consumed automatically ahead of the wallet while nothing
// drains aff_quota in step, so mirroring one reward into both channels would
// let the user redeem it twice. The invitee's first-top-up reward is spendable
// bonus only, backed by an expiring grant, and never touches aff_quota.
//
// Each charge is settled inside one transaction that claims it with a
// compare-and-swap on status, pays it, and writes the terminal status, so two
// nodes can never pay the same charge twice and a crash mid-payout leaves the
// charge pending for the next tick instead of a claimed row with no reward.
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

	// Resolved once per batch. Only the bonus track depends on the duration, so
	// a broken setting defers bonus rewards instead of erroring out and leaving
	// the commissions queued behind them unpaid every tick.
	expireTime, expireErr := referralBonusExpireTime(setting.BonusDuration)
	if expireErr != nil {
		common.SysError(fmt.Sprintf("referral settlement: bonus rewards deferred: %s", expireErr.Error()))
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

		outcome, err := settleReferralCharge(&row, expireTime, expireErr)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to settle referral charge %d: %s", row.Id, err.Error()))
			return settled, err
		}
		if outcome == referralSettlePaid {
			settled++
		}
	}
	if settled > 0 {
		common.SysLog(fmt.Sprintf("referral settlement: %d charges paid out", settled))
	}
	return settled, nil
}

// referralSettleOutcome is what happened to one charge during a settlement pass.
type referralSettleOutcome int

const (
	// referralSettleTaken means another node claimed the charge first.
	referralSettleTaken referralSettleOutcome = iota
	// referralSettlePaid means the reward was paid and the charge closed.
	referralSettlePaid
	// referralSettleDeferred means the charge was left pending because it
	// cannot be paid yet, either under the current configuration or because
	// paying it now would exceed a column ceiling.
	referralSettleDeferred
	// referralSettleCanceled means the charge was parked as unpayable.
	referralSettleCanceled
)

// errReferralPayoutDeferred aborts the settlement transaction so a charge that
// cannot be paid yet keeps its pending status instead of staying claimed.
var errReferralPayoutDeferred = errors.New("referral payout deferred")

// settleReferralCharge settles one charge as a single transaction: it claims
// the charge with a compare-and-swap, pays it, and writes the terminal status.
// Two nodes therefore can never pay the same charge twice, and a crash mid
// payout rolls the claim back, leaving the charge pending for the next tick
// instead of a claimed row with no reward.
func settleReferralCharge(row *ReferralCharge, expireTime int64, expireErr error) (referralSettleOutcome, error) {
	outcome := referralSettleTaken
	err := DB.Transaction(func(tx *gorm.DB) error {
		claimed, err := claimReferralCharge(tx, row.Id)
		if err != nil {
			return err
		}
		if !claimed {
			return nil // another node took it
		}
		outcome, err = payReferralChargeTx(tx, row, expireTime, expireErr)
		return err
	})
	if errors.Is(err, errReferralPayoutDeferred) {
		// The claim rolled back with the transaction, so the charge is still
		// pending and a later tick retries it.
		return referralSettleDeferred, nil
	}
	if err != nil {
		return outcome, err
	}
	return outcome, nil
}

// payReferralChargeTx pays one claimed charge and writes its terminal status
// inside the caller's transaction.
func payReferralChargeTx(tx *gorm.DB, row *ReferralCharge, expireTime int64, expireErr error) (referralSettleOutcome, error) {
	if row.Kind == ReferralKindInviter {
		// Inviter commissions are the only track that becomes claimable.
		// aff_history tracks the lifetime total and is never reduced.
		var user User
		if err := lockForUpdate(tx).Select("id", "aff_quota", "aff_history").
			Where("id = ?", row.UserId).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return referralSettleCanceled, markReferralCharge(tx, row.Id, ReferralStatusCanceled, 0, "user not found")
			}
			return referralSettleCanceled, err
		}
		// aff_quota and aff_history are 32-bit columns. A payout that does not
		// fit yet is deferred rather than canceled: the ceiling is a floor on
		// what the user holds, so moving quota to the main balance frees room
		// and the commission pays on a later tick.
		if row.Amount > int64(common.MaxQuota) ||
			user.AffQuota > common.MaxQuota-int(row.Amount) ||
			user.AffHistoryQuota > common.MaxQuota-int(row.Amount) {
			common.SysError(fmt.Sprintf("referral commission for charge %d exceeds the aff quota ceiling, deferring", row.Id))
			return referralSettleDeferred, errReferralPayoutDeferred
		}
		if err := tx.Model(&User{}).Where("id = ?", row.UserId).Updates(map[string]interface{}{
			"aff_quota":   gorm.Expr("aff_quota + ?", row.Amount),
			"aff_history": gorm.Expr("aff_history + ?", row.Amount),
		}).Error; err != nil {
			return referralSettleCanceled, err
		}
		return referralSettlePaid, markReferralCharge(tx, row.Id, ReferralStatusSettled, 0, "")
	}

	// The invitee's first-top-up reward is spendable bonus only, and the grant
	// is what carries its expiry. A duration that cannot be parsed defers the
	// reward — it must never quietly become a permanent grant, and it must not
	// be canceled either, because the administrator can repair the setting.
	if expireErr != nil {
		return referralSettleDeferred, errReferralPayoutDeferred
	}
	grant, err := grantBonusTx(tx, row.UserId, row.Amount, expireTime, BonusSourceReferral, referralRemark(row))
	if err != nil {
		return referralSettleCanceled, err
	}
	return referralSettlePaid, markReferralCharge(tx, row.Id, ReferralStatusSettled, grant.Id, "")
}

// claimReferralCharge atomically flips one pending charge to settled, reporting
// whether this caller won the race. Status settled here means "in flight"; the
// payout and the terminal status are written in the same transaction, so it
// cannot survive as a claimed row without its reward.
func claimReferralCharge(tx *gorm.DB, id int) (bool, error) {
	result := tx.Model(&ReferralCharge{}).
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

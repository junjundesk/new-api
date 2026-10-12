package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// referralSettleTestSetting enables both reward tracks with a single 1% inviter
// tier, so a test's expected rate never depends on another test's state.
func referralSettleTestSetting(t *testing.T) {
	t.Helper()
	setting := operation_setting.GetReferralSetting()
	previous := *setting
	t.Cleanup(func() { *setting = previous })

	setting.Enabled = true
	setting.InviteeFirstTopUpEnabled = true
	setting.InviteeFirstTopUpRateBps = 300
	setting.InviteeFirstTopUpMaxQuota = 0
	setting.InviterEnabled = true
	setting.InviterTiers = []operation_setting.ReferralTier{{MinInvitees: 1, RateBps: 100}}
	setting.InviterMaxQuota = 0
	setting.BonusDuration = "0"
	setting.CountRedeemAsValid = true
}

// referralSettleTestReset empties every table this feature reads or writes, so
// each test computes against its own users and charges.
func referralSettleTestReset(t *testing.T) {
	t.Helper()
	clear := func() {
		for _, table := range []interface{}{
			&ReferralCharge{}, &UserBonusGrant{}, &BonusPreConsumeRecord{}, &User{}, &TopUp{},
		} {
			require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(table).Error)
		}
	}
	clear()
	t.Cleanup(clear)
}

func seedReferralPair(t *testing.T, inviterAffQuota int) (inviterId int, inviteeId int) {
	t.Helper()
	inviter := &User{
		Username: "settle-inviter",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "settle-inviter-aff",
		AffQuota: inviterAffQuota,
	}
	require.NoError(t, DB.Create(inviter).Error)

	invitee := &User{
		Username:  "settle-invitee",
		Password:  "password123",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   "settle-invitee-aff",
		InviterId: inviter.Id,
	}
	require.NoError(t, DB.Create(invitee).Error)
	return inviter.Id, invitee.Id
}

func seedDirectInvitee(t *testing.T, inviterId int, username string) int {
	t.Helper()
	user := &User{
		Username:  username,
		Password:  "password123",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		InviterId: inviterId,
	}
	require.NoError(t, DB.Create(user).Error)
	return user.Id
}

func recordTopUp(t *testing.T, inviteeId int, refId int, baseQuota int64, countAsValid bool) {
	t.Helper()
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return RecordReferralTopUp(tx, inviteeId, ReferralRefTopUp, refId, baseQuota, countAsValid)
	}))
}

func loadUser(t *testing.T, userId int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.Where("id = ?", userId).First(&user).Error)
	return user
}

func loadCharge(t *testing.T, userId int, kind int) ReferralCharge {
	t.Helper()
	var charge ReferralCharge
	require.NoError(t, DB.Where("user_id = ? AND kind = ?", userId, kind).First(&charge).Error)
	return charge
}

// A commission is money: settling it must credit the claimable referral balance
// and must not also create a spendable bonus grant. Bonus is consumed
// automatically while nothing drains aff_quota in step, so a reward issued
// through both channels could be redeemed twice.
func TestSettleInviterCommissionCreditsAffQuotaOnly(t *testing.T) {
	referralSettleTestSetting(t)
	referralSettleTestReset(t)

	const baseQuota = 10000
	inviterId, inviteeId := seedReferralPair(t, 0)
	recordTopUp(t, inviteeId, 1, baseQuota, true)

	settled, err := SettlePendingReferralCharges()
	require.NoError(t, err)
	require.Equal(t, 2, settled, "commission and invitee reward both settle")

	inviter := loadUser(t, inviterId)
	assert.Equal(t, baseQuota/100, inviter.AffQuota, "1% of the credited quota becomes claimable")
	assert.Equal(t, baseQuota/100, inviter.AffHistoryQuota)

	inviterBonus, err := GetActiveBonusRemaining(inviterId)
	require.NoError(t, err)
	assert.Equal(t, int64(0), inviterBonus, "a commission must never also become spendable bonus")

	inviteeBonus, err := GetActiveBonusRemaining(inviteeId)
	require.NoError(t, err)
	assert.Equal(t, int64(baseQuota*3/100), inviteeBonus, "the invitee reward is spendable bonus only")

	inviteeCharge := loadCharge(t, inviteeId, ReferralKindInvitee)
	assert.Equal(t, ReferralStatusSettled, inviteeCharge.Status)
	assert.NotZero(t, inviteeCharge.GrantId, "the invitee reward records the grant backing it")

	inviterCharge := loadCharge(t, inviterId, ReferralKindInviter)
	assert.Equal(t, ReferralStatusSettled, inviterCharge.Status)
	assert.Zero(t, inviterCharge.GrantId, "a commission is not backed by a bonus grant")

	// Settling again must not pay anything twice.
	settled, err = SettlePendingReferralCharges()
	require.NoError(t, err)
	assert.Zero(t, settled)
	assert.Equal(t, baseQuota/100, loadUser(t, inviterId).AffQuota)
}

// Tier promotion is once per invitee, even when the invitee reward is disabled
// and therefore records no invitee row to derive "first" from. Without that,
// every later top-up would promote the same invitee again and inflate the count
// the commission rate is looked up with.
func TestInviteePromotesInviterTierOnce(t *testing.T) {
	referralSettleTestSetting(t)
	referralSettleTestReset(t)
	operation_setting.GetReferralSetting().InviteeFirstTopUpEnabled = false

	const baseQuota = 10000
	inviterId, inviteeId := seedReferralPair(t, 0)
	recordTopUp(t, inviteeId, 1, baseQuota, true)
	recordTopUp(t, inviteeId, 2, baseQuota, true)

	var charges []ReferralCharge
	require.NoError(t, DB.Where("kind = ?", ReferralKindInviter).Order("id asc").Find(&charges).Error)
	require.Len(t, charges, 2)
	assert.True(t, charges[0].CountsValid, "the invitee's first top-up promotes them")
	assert.False(t, charges[1].CountsValid, "a later top-up must not promote the same invitee again")

	valid, err := CountValidInvitees(inviterId)
	require.NoError(t, err)
	assert.Equal(t, 1, valid, "one invitee counts once")

	summary, err := GetReferralSummary(inviterId)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.ValidInvitees)
}

// A non-counting first event (a redemption code with the admin exclusion) must
// not consume the invitee's single promotion, so a later online payment still
// promotes them.
func TestNonCountingFirstEventStillAllowsLaterPromotion(t *testing.T) {
	referralSettleTestSetting(t)
	referralSettleTestReset(t)
	operation_setting.GetReferralSetting().CountRedeemAsValid = false

	const baseQuota = 10000
	inviterId, inviteeId := seedReferralPair(t, 0)
	// An existing valid invitee keeps the commission rate positive, so the
	// non-counting event still records a (non-promoting) charge.
	otherId := seedDirectInvitee(t, inviterId, "settle-other")
	recordTopUp(t, otherId, 1, baseQuota, true)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return RecordReferralRedemption(tx, inviteeId, 1, baseQuota)
	}))
	// A distinct source id: ref keys are built from the source row id, which is
	// unique per payment record rather than per user.
	recordTopUp(t, inviteeId, 2, baseQuota, true)

	var charges []ReferralCharge
	require.NoError(t, DB.Where("invitee_id = ? AND kind = ?", inviteeId, ReferralKindInviter).
		Order("id asc").Find(&charges).Error)
	require.Len(t, charges, 2)
	assert.False(t, charges[0].CountsValid, "a redemption code is not tier-promoting by configuration")
	assert.True(t, charges[1].CountsValid, "the later online payment is the invitee's first counting event")

	valid, err := CountValidInvitees(inviterId)
	require.NoError(t, err)
	assert.Equal(t, 2, valid, "the other invitee plus this one")
}

// A payout that no longer fits the 32-bit aff_quota column is deferred, not
// canceled: the user can free room by moving quota to the main balance, and the
// commission then settles on a later tick. A canceled charge would silently
// discard money the platform owes.
func TestSettleCommissionBeyondAffQuotaCeilingIsDeferred(t *testing.T) {
	referralSettleTestSetting(t)
	referralSettleTestReset(t)

	const baseQuota = 10000
	inviterId, inviteeId := seedReferralPair(t, common.MaxQuota)
	recordTopUp(t, inviteeId, 1, baseQuota, true)

	settled, err := SettlePendingReferralCharges()
	require.NoError(t, err)
	assert.Equal(t, 1, settled, "the invitee reward is unaffected and still settles")

	charge := loadCharge(t, inviterId, ReferralKindInviter)
	assert.Equal(t, ReferralStatusPending, charge.Status, "the commission stays pending for a later tick")
	assert.Equal(t, common.MaxQuota, loadUser(t, inviterId).AffQuota, "the deferred payout must not change the balance")

	// Moving quota out of aff_quota frees room and the deferred payout settles.
	require.NoError(t, DB.Model(&User{}).Where("id = ?", inviterId).Update("aff_quota", 0).Error)

	settled, err = SettlePendingReferralCharges()
	require.NoError(t, err)
	assert.Equal(t, 1, settled)
	inviter := loadUser(t, inviterId)
	assert.Equal(t, baseQuota/100, inviter.AffQuota)
	assert.Equal(t, baseQuota/100, inviter.AffHistoryQuota)
}

// A duration that cannot be parsed defers bonus rewards instead of erroring the
// whole batch out: the administrator can repair the setting, and a permanently
// broken setting must never silently become a permanent grant. Commissions in
// the same batch do not depend on the duration and still settle.
func TestSettleInvalidBonusDurationDefersInviteeReward(t *testing.T) {
	referralSettleTestSetting(t)
	referralSettleTestReset(t)
	operation_setting.GetReferralSetting().BonusDuration = "abc"

	const baseQuota = 10000
	inviterId, inviteeId := seedReferralPair(t, 0)
	recordTopUp(t, inviteeId, 1, baseQuota, true)

	settled, err := SettlePendingReferralCharges()
	require.NoError(t, err)
	assert.Equal(t, 1, settled, "the commission does not depend on the bonus duration")
	assert.Equal(t, baseQuota/100, loadUser(t, inviterId).AffQuota)

	charge := loadCharge(t, inviteeId, ReferralKindInvitee)
	assert.Equal(t, ReferralStatusPending, charge.Status, "the reward waits for a valid setting, it is not dropped")
	assert.Zero(t, charge.GrantId)

	// Repairing the setting drains the backlog without double-paying.
	operation_setting.GetReferralSetting().BonusDuration = "0"
	settled, err = SettlePendingReferralCharges()
	require.NoError(t, err)
	assert.Equal(t, 1, settled)

	inviteeBonus, err := GetActiveBonusRemaining(inviteeId)
	require.NoError(t, err)
	assert.Equal(t, int64(baseQuota*3/100), inviteeBonus)
	assert.Equal(t, baseQuota/100, loadUser(t, inviterId).AffQuota, "the commission was not paid twice")
}

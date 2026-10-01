package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostConsumeQuotaChargesBonusBucket pins the funding contract for
// post-settlement charges (for example a violation fee) on a request that was
// billed from bonus: the charge must come out of the bonus bucket that funded
// the request, not the wallet, which the request never touched.
func TestPostConsumeQuotaChargesBonusBucket(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.Exec("DELETE FROM user_bonus_grants").Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM user_bonus_grants")
		model.DB.Exec("DELETE FROM bonus_pre_consume_records")
		model.DB.Exec("DELETE FROM bonus_pre_consume_allocations")
	})

	const userID = 8811
	const walletQuota = 0
	seedUser(t, userID, walletQuota)

	grant, err := model.GrantBonus(userID, 10_000, 0, model.BonusSourceAdmin, "test")
	require.NoError(t, err)

	relayInfo := &relaycommon.RelayInfo{
		UserId:        userID,
		RequestId:     "req-fee-from-bonus",
		IsPlayground:  true, // skips token quota mutation
		BillingSource: BillingSourceBonus,
	}

	const fee = 1_500
	require.NoError(t, PostConsumeQuota(relayInfo, fee, 0, false))

	// The fee must land on the bonus bucket.
	var refreshed model.UserBonusGrant
	require.NoError(t, model.DB.Where("id = ?", grant.Id).First(&refreshed).Error)
	assert.Equal(t, int64(fee), refreshed.AmountUsed, "fee must be charged against the bonus grant")

	// The wallet must stay untouched.
	remaining, err := model.GetActiveBonusRemaining(userID)
	require.NoError(t, err)
	assert.Equal(t, int64(10_000-fee), remaining)

	user, err := model.GetUserById(userID, false)
	require.NoError(t, err)
	assert.Equal(t, walletQuota, user.Quota, "a bonus-billed request must not debit the wallet")
}

// TestPostConsumeQuotaFallsBackToWalletWhenBonusExhausted keeps the "fee is
// always collected" guarantee: when the bonus cannot cover the charge, the
// difference is recorded as wallet debt instead of being dropped.
func TestPostConsumeQuotaFallsBackToWalletWhenBonusExhausted(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.Exec("DELETE FROM user_bonus_grants").Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM user_bonus_grants")
		model.DB.Exec("DELETE FROM bonus_pre_consume_records")
		model.DB.Exec("DELETE FROM bonus_pre_consume_allocations")
	})

	const userID = 8812
	seedUser(t, userID, 5_000)

	// Bonus covers far less than the fee.
	_, err := model.GrantBonus(userID, 100, 0, model.BonusSourceAdmin, "small")
	require.NoError(t, err)

	relayInfo := &relaycommon.RelayInfo{
		UserId:        userID,
		RequestId:     "req-fee-bonus-short",
		IsPlayground:  true,
		BillingSource: BillingSourceBonus,
	}

	const fee = 900
	require.NoError(t, PostConsumeQuota(relayInfo, fee, 0, false))

	user, err := model.GetUserById(userID, false)
	require.NoError(t, err)
	assert.Equal(t, 5_000-fee, user.Quota, "uncovered fee must be recorded as wallet debt")
	assert.Less(t, user.Quota, 5_000, "the fee must still be collected")
}

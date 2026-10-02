package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBillingTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func cleanupBonusTables(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Exec("DELETE FROM user_bonus_grants").Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM user_bonus_grants")
		model.DB.Exec("DELETE FROM bonus_pre_consume_records")
		model.DB.Exec("DELETE FROM bonus_pre_consume_allocations")
	})
}

// TestNewBillingSessionMixedBonusWallet pins the deduction-priority contract:
// when active bonus alone cannot cover the pre-consume but bonus + wallet can,
// the session must drain the bonus first and let the wallet cover only the
// shortfall. On settlement refund (actual < pre-consumed) the wallet is
// refunded first so the expiring bonus stays consumed.
func TestNewBillingSessionMixedBonusWallet(t *testing.T) {
	truncate(t)
	cleanupBonusTables(t)

	const userID = 8901
	seedUser(t, userID, 70_000)

	// Bonus (30k) alone cannot cover the 100k pre-consume; bonus + wallet exactly can.
	_, err := model.GrantBonus(userID, 30_000, 0, model.BonusSourceAdmin, "test")
	require.NoError(t, err)

	relayInfo := &relaycommon.RelayInfo{
		UserId:       userID,
		RequestId:    "req-mixed-bonus-wallet",
		IsPlayground: true, // skips token quota mutation
	}
	session, apiErr := NewBillingSession(newBillingTestContext(), relayInfo, 100_000)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	// Bonus drained first, wallet covered exactly the 70k shortfall.
	remaining, err := model.GetActiveBonusRemaining(userID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), remaining)

	user, err := model.GetUserById(userID, false)
	require.NoError(t, err)
	assert.Equal(t, 0, user.Quota)

	assert.Equal(t, 100_000, session.GetPreConsumedQuota())
	assert.Equal(t, BillingSourceBonus, relayInfo.BillingSource)
	assert.Equal(t, int64(30_000), relayInfo.BonusPreConsumed)
	assert.Equal(t, int64(70_000), relayInfo.BonusWalletDeducted)

	// Actual 60k: the 40k over-reserve is refunded to the wallet first and the
	// expiring bonus stays consumed.
	require.NoError(t, session.Settle(60_000))
	user, err = model.GetUserById(userID, false)
	require.NoError(t, err)
	assert.Equal(t, 40_000, user.Quota)

	remaining, err = model.GetActiveBonusRemaining(userID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), remaining)
	assert.Equal(t, int64(30_000), relayInfo.BonusWalletDeducted)
}

// TestNewBillingSessionMixedFallsBackWhenBonusPlusWalletInsufficient keeps the
// rejection contract: when neither the bonus nor bonus + wallet can cover the
// pre-consume, billing fails as insufficient and both buckets stay untouched.
func TestNewBillingSessionMixedFallsBackWhenBonusPlusWalletInsufficient(t *testing.T) {
	truncate(t)
	cleanupBonusTables(t)

	const userID = 8902
	seedUser(t, userID, 10_000)

	_, err := model.GrantBonus(userID, 30_000, 0, model.BonusSourceAdmin, "test")
	require.NoError(t, err)

	relayInfo := &relaycommon.RelayInfo{
		UserId:       userID,
		RequestId:    "req-mixed-insufficient",
		IsPlayground: true,
		UserSetting:  dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(newBillingTestContext(), relayInfo, 100_000)
	require.Nil(t, session)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())

	// Both buckets untouched by the failed mixed attempt.
	user, err := model.GetUserById(userID, false)
	require.NoError(t, err)
	assert.Equal(t, 10_000, user.Quota)

	remaining, err := model.GetActiveBonusRemaining(userID)
	require.NoError(t, err)
	assert.Equal(t, int64(30_000), remaining)
}

// TestBonusWalletFundingRefundRestoresBothBuckets: a failed request must
// return the whole pre-consume — the wallet shortfall and the drained bonus
// alike — leaving both buckets exactly as before.
func TestBonusWalletFundingRefundRestoresBothBuckets(t *testing.T) {
	truncate(t)
	cleanupBonusTables(t)

	const userID = 8903
	seedUser(t, userID, 100_000)

	_, err := model.GrantBonus(userID, 30_000, 0, model.BonusSourceAdmin, "test")
	require.NoError(t, err)

	mw := &BonusWalletFunding{
		bonus:      &BonusFunding{requestId: "req-mixed-refund", userId: userID, amount: 30_000},
		wallet:     &WalletFunding{userId: userID},
		bonusShare: 30_000,
	}
	require.NoError(t, mw.PreConsume(100_000))
	require.NoError(t, mw.Refund())

	user, err := model.GetUserById(userID, false)
	require.NoError(t, err)
	assert.Equal(t, 100_000, user.Quota)

	remaining, err := model.GetActiveBonusRemaining(userID)
	require.NoError(t, err)
	assert.Equal(t, int64(30_000), remaining)
}

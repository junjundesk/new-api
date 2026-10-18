package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestPreConsumeUserBonusConsumesSoonestExpiryFirst verifies the core bonus
// billing contract: a request draws down the grant that expires first, then
// spills into the next grant, and expired grants are never touched.
func TestPreConsumeUserBonusConsumesSoonestExpiryFirst(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&UserBonusGrant{}, &BonusPreConsumeRecord{}))
	reset := func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&UserBonusGrant{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&BonusPreConsumeRecord{}).Error)
	}
	reset()
	t.Cleanup(reset)

	now := common.GetTimestamp()
	const userId = 4242
	grants := []UserBonusGrant{
		// Expired: must never be consumed.
		{UserId: userId, AmountTotal: 1000, ExpireTime: now - 100, Status: BonusStatusActive},
		// Expires in 1 hour: consumed first.
		{UserId: userId, AmountTotal: 300, ExpireTime: now + 3600, Status: BonusStatusActive},
		// Expires in 2 hours: consumed second.
		{UserId: userId, AmountTotal: 500, ExpireTime: now + 7200, Status: BonusStatusActive},
		// Never expires: consumed last.
		{UserId: userId, AmountTotal: 400, ExpireTime: 0, Status: BonusStatusActive},
	}
	require.NoError(t, DB.Create(&grants).Error)

	// Remaining excludes the expired grant: 300 + 500 + 400 = 1200.
	remaining, err := GetActiveBonusRemaining(userId)
	require.NoError(t, err)
	assert.Equal(t, int64(1200), remaining)

	// Consume 600: 300 from the 1h grant, 300 from the 2h grant.
	res, err := PreConsumeUserBonus("req-bonus-1", userId, 600)
	require.NoError(t, err)
	assert.Equal(t, int64(600), res.PreConsumed)
	assert.Equal(t, grants[1].Id, res.GrantId, "first touched grant must be the soonest-expiring one")

	var first, second, never UserBonusGrant
	require.NoError(t, DB.Where("id = ?", grants[1].Id).First(&first).Error)
	require.NoError(t, DB.Where("id = ?", grants[2].Id).First(&second).Error)
	require.NoError(t, DB.Where("id = ?", grants[3].Id).First(&never).Error)
	assert.Equal(t, int64(300), first.AmountUsed, "1h grant fully consumed")
	assert.Equal(t, BonusStatusUsed, first.Status)
	assert.Equal(t, int64(300), second.AmountUsed, "2h grant partially consumed")
	assert.Equal(t, BonusStatusActive, second.Status)
	assert.Equal(t, int64(0), never.AmountUsed, "never-expiring grant untouched")

	// Idempotency: replaying the same requestId must not double-consume.
	res2, err := PreConsumeUserBonus("req-bonus-1", userId, 600)
	require.NoError(t, err)
	assert.Equal(t, int64(600), res2.PreConsumed)
	after, err := GetActiveBonusRemaining(userId)
	require.NoError(t, err)
	assert.Equal(t, int64(600), after, "replayed request must not consume twice")

	// Insufficient bonus must fail atomically.
	_, err = PreConsumeUserBonus("req-bonus-2", userId, 10_000)
	require.Error(t, err)
	afterFail, err := GetActiveBonusRemaining(userId)
	require.NoError(t, err)
	assert.Equal(t, int64(600), afterFail, "failed pre-consume must not partially consume")

	// Refund restores the pre-consumed amount.
	require.NoError(t, RefundBonusPreConsume("req-bonus-1"))
	refunded, err := GetActiveBonusRemaining(userId)
	require.NoError(t, err)
	assert.Equal(t, int64(1200), refunded)
}

func TestRevokeUserBonusGrantRemovesRemainingCredit(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&UserBonusGrant{}, &BonusPreConsumeRecord{}, &BonusPreConsumeAllocation{}))
	reset := func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&UserBonusGrant{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&BonusPreConsumeRecord{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&BonusPreConsumeAllocation{}).Error)
	}
	reset()
	t.Cleanup(reset)

	const userId = 5252
	grant := &UserBonusGrant{UserId: userId, AmountTotal: 1000, ExpireTime: 0, Status: BonusStatusActive, Remark: "wrong expiry"}
	require.NoError(t, DB.Create(grant).Error)
	other := &UserBonusGrant{UserId: userId, AmountTotal: 500, ExpireTime: 0, Status: BonusStatusActive}
	require.NoError(t, DB.Create(other).Error)

	_, err := PreConsumeUserBonus("req-revoke-1", userId, 200)
	require.NoError(t, err)

	revoked, amount, err := RevokeUserBonusGrant(userId, grant.Id, "expiry entered incorrectly")
	require.NoError(t, err)
	assert.Equal(t, BonusStatusRevoked, revoked.Status)
	assert.Equal(t, int64(800), amount, "only the unconsumed amount is removed")
	assert.Equal(t, "expiry entered incorrectly", revoked.RevokeRemark)
	assert.NotZero(t, revoked.RevokedAt)

	remaining, err := GetActiveBonusRemaining(userId)
	require.NoError(t, err)
	assert.Equal(t, int64(500), remaining, "revoked grant no longer counts as active credit")

	_, _, err = RevokeUserBonusGrant(userId, grant.Id, "")
	require.ErrorIs(t, err, ErrBonusGrantRevoked)

	_, _, err = RevokeUserBonusGrant(userId+1, other.Id, "")
	require.ErrorIs(t, err, ErrBonusGrantNotFound)

	used := &UserBonusGrant{UserId: userId, AmountTotal: 100, AmountUsed: 100, ExpireTime: 0, Status: BonusStatusUsed}
	require.NoError(t, DB.Create(used).Error)
	_, _, err = RevokeUserBonusGrant(userId, used.Id, "")
	require.ErrorIs(t, err, ErrBonusGrantNotActive)
}

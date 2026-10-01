package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestActiveBonusSplitKeepsPermanentAndExpiringSeparate covers the admin UI
// contract: a user holding both an expiring and a never-expiring grant must be
// able to tell them apart, and expiry must never hide the permanent part.
func TestActiveBonusSplitKeepsPermanentAndExpiringSeparate(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&UserBonusGrant{}, &BonusPreConsumeRecord{}, &BonusPreConsumeAllocation{}))
	reset := func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&UserBonusGrant{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&BonusPreConsumeRecord{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&BonusPreConsumeAllocation{}).Error)
	}
	reset()
	t.Cleanup(reset)

	now := common.GetTimestamp()
	const userId = 5150
	grants := []UserBonusGrant{
		{UserId: userId, AmountTotal: 300, ExpireTime: now + 3600, Status: BonusStatusActive},
		{UserId: userId, AmountTotal: 700, ExpireTime: 0, Status: BonusStatusActive},
	}
	require.NoError(t, DB.Create(&grants).Error)

	active, err := ListActiveBonusGrants(userId)
	require.NoError(t, err)
	require.Len(t, active, 2)

	total, expiring, permanent := int64(0), int64(0), int64(0)
	var nearest int64
	for _, g := range active {
		remaining := g.Remaining()
		total += remaining
		if g.ExpireTime == 0 {
			permanent += remaining
			continue
		}
		expiring += remaining
		if nearest == 0 || g.ExpireTime < nearest {
			nearest = g.ExpireTime
		}
	}

	assert.Equal(t, int64(1000), total)
	assert.Equal(t, int64(300), expiring, "expiring portion stays separate from permanent")
	assert.Equal(t, int64(700), permanent, "permanent portion must not be folded into the expiring figure")
	assert.Equal(t, grants[0].ExpireTime, nearest, "nearest expiry comes from the expiring grant only")

	// Draining the expiring grant must leave the permanent one untouched.
	_, err = PreConsumeUserBonus("req-split-1", userId, 300)
	require.NoError(t, err)
	afterActive, err := ListActiveBonusGrants(userId)
	require.NoError(t, err)
	afterPermanent := int64(0)
	for _, g := range afterActive {
		if g.ExpireTime == 0 {
			afterPermanent += g.Remaining()
		}
	}
	assert.Equal(t, int64(700), afterPermanent, "exhausting the expiring grant must not consume the permanent one")
}

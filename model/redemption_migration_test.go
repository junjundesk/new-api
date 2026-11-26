package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateUncategorizedRedemptionsMatchesFaceValues(t *testing.T) {
	userId, legacyKey := setupRedeemFixture(t, 1030)
	require.NoError(t, DB.AutoMigrate(&RedemptionCategory{}, &RedemptionBonusRule{}))
	categories := []RedemptionCategory{
		{Name: "Premium", Quota: 2000}, {Name: "Standard", Quota: 1000},
		{Name: "Campaign A", Quota: 3000}, {Name: "Campaign B", Quota: 3000},
	}
	require.NoError(t, DB.Create(&categories).Error)
	t.Cleanup(func() { require.NoError(t, DB.Delete(&categories).Error) })
	high, low := categories[0], categories[1]
	bonus := RedemptionBonusRule{CategoryId: low.Id, BonusRateBps: 500}
	require.NoError(t, DB.Create(&bonus).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Delete(&bonus).Error)
		require.NoError(t, DB.Where("user_id = ?", userId).Delete(&UserBonusGrant{}).Error)
	})
	codes := []Redemption{
		{Key: "ten", Quota: 1000}, {Key: "almost-twenty", Quota: 1999},
		{Key: "twenty", Quota: 2000}, {Key: "twenty-one", Quota: 2100},
		{Key: "below-minimum", Quota: 999}, {Key: "ambiguous", Quota: 3000},
		{Key: "above-ambiguous", Quota: 3100},
	}
	require.NoError(t, DB.Create(&codes).Error)

	var expected []Redemption
	require.NoError(t, DB.Order("id ASC").Find(&expected).Error)
	for _, step := range []struct {
		mode    RedemptionMigrationMode
		movedTo map[string]RedemptionCategory
		result  RedemptionMigrationResult
	}{
		{RedemptionMigrationExact, map[string]RedemptionCategory{"ten": low, "twenty": high}, RedemptionMigrationResult{Migrated: 2, Unmatched: 6}},
		{RedemptionMigrationFuzzy, map[string]RedemptionCategory{legacyKey: low, "almost-twenty": low, "twenty-one": high}, RedemptionMigrationResult{Migrated: 3, Unmatched: 3}},
		{RedemptionMigrationFuzzy, nil, RedemptionMigrationResult{Unmatched: 3}}, // A repeat cannot round a code down again.
	} {
		result, err := MigrateUncategorizedRedemptions(step.mode)
		require.NoError(t, err)
		assert.Equal(t, step.result, result)
		for i, code := range expected {
			if target, ok := step.movedTo[code.Key]; ok {
				expected[i].CategoryId, expected[i].Quota = target.Id, target.Quota
			}
		}
		var stored []Redemption
		require.NoError(t, DB.Order("id ASC").Find(&stored).Error)
		assert.Equal(t, expected, stored, "only the category and face value of matched codes may change")
	}

	receipt, err := Redeem(legacyKey, userId)
	require.NoError(t, err)
	assert.Equal(t, 1000, receipt.Quota)
	assert.Equal(t, 50, receipt.BonusQuota, "redeeming uses the destination's live bonus rule")
}

func TestMigrateUncategorizedRedemptionsLeavesIneligibleCodesUntouched(t *testing.T) {
	userId, legacyKey := setupRedeemFixture(t, 1030)
	require.NoError(t, DB.AutoMigrate(&RedemptionCategory{}))
	result, err := MigrateUncategorizedRedemptions(RedemptionMigrationFuzzy)
	require.NoError(t, err)
	assert.Equal(t, RedemptionMigrationResult{Unmatched: 1}, result, "without categories nothing moves")

	category := RedemptionCategory{Name: "Ten", Quota: 1000}
	require.NoError(t, category.SaveSettings())
	t.Cleanup(func() { require.NoError(t, DB.Delete(&category).Error) })
	now := common.GetTimestamp()
	protected := []Redemption{
		{Key: "used", Status: common.RedemptionCodeStatusUsed, Quota: 1030, UsedUserId: userId, RedeemedTime: now, BonusQuota: 50},
		{Key: "disabled", Status: common.RedemptionCodeStatusDisabled, Quota: 1030},
		{Key: "expired", Quota: 1030, ExpiredTime: now - 1},
		{Key: "already-grouped", Quota: 1000, CategoryId: category.Id},
		{Key: "previously-redeemed", Quota: 1030, RedeemedTime: now},
		{Key: "deleted", Quota: 1030},
	}
	require.NoError(t, DB.Create(&protected).Error)
	require.NoError(t, DB.Delete(&protected[len(protected)-1]).Error)
	// Old unused rows can predate expiry and used-user tracking.
	require.NoError(t, DB.Model(&Redemption{}).Where(commonKeyCol+" = ?", legacyKey).
		Updates(map[string]any{"expired_time": nil, "used_user_id": nil, "redeemed_time": nil}).Error)
	var before []Redemption
	require.NoError(t, DB.Unscoped().Where(commonKeyCol+" <> ?", legacyKey).Order("id ASC").Find(&before).Error)

	result, err = MigrateUncategorizedRedemptions(RedemptionMigrationFuzzy)
	require.NoError(t, err)
	assert.Equal(t, RedemptionMigrationResult{Migrated: 1}, result)
	var after []Redemption
	require.NoError(t, DB.Unscoped().Where(commonKeyCol+" <> ?", legacyKey).Order("id ASC").Find(&after).Error)
	assert.Equal(t, before, after)
}

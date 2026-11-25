package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCategoryBatchUsesLiveBonusAndKeepsPreviousGrants(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 100)
	require.NoError(t, DB.AutoMigrate(&RedemptionCategory{}, &RedemptionBonusRule{}))
	category := RedemptionCategory{Name: "100 元档", Quota: 10000}
	require.NoError(t, category.SaveSettings())
	categoryId := category.Id
	t.Cleanup(func() {
		require.NoError(t, DB.Delete(&RedemptionCategory{}, categoryId).Error)
		require.NoError(t, DB.Where("category_id = ?", categoryId).Delete(&RedemptionBonusRule{}).Error)
		require.NoError(t, DB.Where("user_id = ?", userId).Delete(&UserBonusGrant{}).Error)
	})
	issued := Redemption{UserId: userId, CategoryId: category.Id, Count: 3, Quota: 1}
	keys, err := issued.IssueBatch()
	require.NoError(t, err)
	require.Len(t, keys, 3)
	uncategorized := 0
	_, total, err := SearchRedemptions("", "", &uncategorized, 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	rows, total, err := SearchRedemptions("100", "1", &category.Id, 0, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, rows, 1)
	assert.Equal(t, 10000, rows[0].Quota)
	assert.Equal(t, category.Name, rows[0].Name)
	assert.ErrorIs(t, DeleteRedemptionCategory(category.Id), ErrRedemptionCategoryInUse)

	for i, scenario := range []struct{ rate, wantBonus int }{{200, 200}, {325, 325}, {0, 0}} {
		// A 0% rule is kept as a placeholder and must not create an empty bonus grant.
		_, err := ReplaceRedemptionBonusRules([]RedemptionBonusRule{{CategoryId: category.Id, BonusRateBps: scenario.rate}})
		require.NoError(t, err)
		receipt, err := Redeem(keys[i], userId)
		require.NoError(t, err)
		assert.Equal(t, scenario.wantBonus, receipt.BonusQuota)
		assert.Equal(t, 10000, receipt.Quota)
		_, err = Redeem(keys[i], userId)
		assert.ErrorIs(t, err, ErrRedeemFailed)
		enabled := common.RedemptionCodeStatusEnabled
		_, err = UpdateRedemptionDetails(receipt.Id, nil, "", 0, 0, &enabled)
		assert.ErrorIs(t, err, ErrRedemptionAlreadyUsed)
	}
	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	assert.Equal(t, 30000, user.Quota)
	grants, err := ListActiveBonusGrants(userId)
	require.NoError(t, err)
	require.Len(t, grants, 2)
	assert.ElementsMatch(t, []int64{200, 325}, []int64{grants[0].AmountTotal, grants[1].AmountTotal})
	for _, grant := range grants {
		assert.Equal(t, BonusSourceRedemption, grant.Source)
		assert.Zero(t, grant.ExpireTime)
	}
	rows, _, err = SearchRedemptions("", "3", &category.Id, 0, 10)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, 200, rows[2].BonusQuota)
	_, err = DeleteInvalidRedemptions(&uncategorized)
	require.NoError(t, err)
	deleted, err := DeleteInvalidRedemptions(&category.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 3, deleted)
	_, err = ReplaceRedemptionBonusRules([]RedemptionBonusRule{{CategoryId: category.Id, BonusRateBps: 100}})
	require.NoError(t, err)
	require.NoError(t, DeleteRedemptionCategory(category.Id))
	rules, err := GetRedemptionBonusRules()
	require.NoError(t, err)
	assert.Empty(t, rules)
	_, total, err = SearchRedemptions("", "", &uncategorized, 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
}

func TestRedemptionCategoryRejectsInvalidIssuanceAndRollsBackFailedCredit(t *testing.T) {
	userId, legacyKey := setupRedeemFixture(t, 500)
	require.NoError(t, DB.AutoMigrate(&RedemptionCategory{}, &RedemptionBonusRule{}))
	category := RedemptionCategory{Name: "Trial", Quota: 10000}
	require.NoError(t, category.SaveSettings())
	t.Cleanup(func() { require.NoError(t, DB.Delete(&category).Error) })
	_, err := ReplaceRedemptionBonusRules([]RedemptionBonusRule{{CategoryId: category.Id, BonusRateBps: 100}})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := ReplaceRedemptionBonusRules(nil)
		require.NoError(t, err)
	})
	for _, scenario := range []struct {
		rules []RedemptionBonusRule
		want  error
	}{
		{[]RedemptionBonusRule{{CategoryId: category.Id, BonusRateBps: -1}}, ErrRedemptionBonusRate},
		{[]RedemptionBonusRule{{CategoryId: category.Id, BonusRateBps: MaxRedemptionBonusRateBps + 1}}, ErrRedemptionBonusRate},
		{[]RedemptionBonusRule{{CategoryId: category.Id, BonusRateBps: 100}, {CategoryId: category.Id, BonusRateBps: 200}}, ErrRedemptionBonusRuleConflict},
		{[]RedemptionBonusRule{{CategoryId: category.Id + 1, BonusRateBps: 100}}, ErrRedemptionCategory},
	} {
		_, err = ReplaceRedemptionBonusRules(scenario.rules)
		assert.ErrorIs(t, err, scenario.want)
	}
	rules, err := GetRedemptionBonusRules()
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, 100, rules[0].BonusRateBps)
	batch := Redemption{Count: 100, CategoryId: category.Id + 1}
	keys, err := batch.IssueBatch()
	assert.ErrorIs(t, err, ErrRedemptionCategory)
	assert.Empty(t, keys)

	var code Redemption
	require.NoError(t, DB.Where(commonKeyCol+" = ?", legacyKey).First(&code).Error)
	updated, err := UpdateRedemptionDetails(code.Id, &category.Id, code.Name, 500, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, category.Quota, updated.Quota)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", userId).Update("quota", common.MaxQuota-1).Error)
	_, err = Redeem(legacyKey, userId)
	require.ErrorIs(t, err, ErrRedeemFailed)
	require.NoError(t, DB.First(&code, code.Id).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, code.Status)
	assert.Zero(t, code.UsedUserId)
	bonus, err := GetActiveBonusRemaining(userId)
	require.NoError(t, err)
	assert.Zero(t, bonus)
}

func TestLegacyCategoryBonusRatesMigrateToRulesOnce(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&RedemptionCategory{}, &RedemptionBonusRule{}))
	require.NoError(t, DB.Exec("ALTER TABLE redemption_categories ADD COLUMN bonus_rate_bps integer NOT NULL DEFAULT 0").Error)
	legacy := RedemptionCategory{Name: "Legacy", Quota: 10000}
	plain := RedemptionCategory{Name: "Plain", Quota: 20000}
	require.NoError(t, legacy.SaveSettings())
	require.NoError(t, plain.SaveSettings())
	t.Cleanup(func() {
		require.NoError(t, DeleteRedemptionCategory(legacy.Id))
		require.NoError(t, DeleteRedemptionCategory(plain.Id))
		require.NoError(t, DB.Exec("ALTER TABLE redemption_categories DROP COLUMN bonus_rate_bps").Error)
	})
	require.NoError(t, DB.Table("redemption_categories").Where("id = ?", legacy.Id).Update("bonus_rate_bps", 250).Error)

	require.NoError(t, migrateRedemptionCategoryBonusRates())
	rules, err := GetRedemptionBonusRules()
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, legacy.Id, rules[0].CategoryId)
	assert.Equal(t, 250, rules[0].BonusRateBps)

	_, err = ReplaceRedemptionBonusRules(nil)
	require.NoError(t, err)
	require.NoError(t, migrateRedemptionCategoryBonusRates())
	rules, err = GetRedemptionBonusRules()
	require.NoError(t, err)
	assert.Empty(t, rules, "a rule deleted by an admin must not be restored on restart")
}

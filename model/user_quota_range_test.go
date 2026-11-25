package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// A wallet must be able to hold and redeem amounts beyond the old int32 ceiling
// (about $4,294.97 at 500,000 quota per USD).
func TestRedeemCreditsFaceValueAboveInt32Range(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 100)
	require.NoError(t, DB.AutoMigrate(&RedemptionCategory{}, &RedemptionBonusRule{}))
	const faceValue = 2_500_000_000 // $5,000
	category := RedemptionCategory{Name: "5000 档", Quota: faceValue}
	require.NoError(t, category.SaveSettings())
	t.Cleanup(func() {
		require.NoError(t, DB.Where("category_id = ?", category.Id).Delete(&RedemptionBonusRule{}).Error)
		require.NoError(t, DB.Delete(&category).Error)
		require.NoError(t, DB.Where("user_id = ?", userId).Delete(&UserBonusGrant{}).Error)
	})
	_, err := ReplaceRedemptionBonusRules([]RedemptionBonusRule{{CategoryId: category.Id, BonusRateBps: 1000}})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := ReplaceRedemptionBonusRules(nil)
		require.NoError(t, err)
	})
	batch := Redemption{UserId: userId, CategoryId: category.Id, Count: 2}
	keys, err := batch.IssueBatch()
	require.NoError(t, err)

	for _, key := range keys {
		receipt, err := Redeem(key, userId)
		require.NoError(t, err)
		assert.Equal(t, faceValue, receipt.Quota)
		assert.Equal(t, 250_000_000, receipt.BonusQuota)
	}
	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	assert.Equal(t, 2*faceValue, user.Quota)
	bonus, err := GetActiveBonusRemaining(userId)
	require.NoError(t, err)
	assert.EqualValues(t, 500_000_000, bonus)
}

func TestTopUpCreditRejectsBalanceBeyondMaxQuota(t *testing.T) {
	userId, _ := setupRedeemFixture(t, 100)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", userId).Update("quota", common.MaxQuota-10).Error)
	err := DB.Transaction(func(tx *gorm.DB) error { return creditTopUpQuota(tx, userId, 10, nil) })
	require.ErrorIs(t, err, ErrTopUpQuotaLimitExceeded)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error { return creditTopUpQuota(tx, userId, 9, nil) }))
	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	assert.Equal(t, common.MaxQuota-1, user.Quota)
}

// Older SQLite databases declared wallet columns as "int". SQLite stores
// INTEGER values as 64-bit regardless, so AutoMigrate must leave those columns
// alone (no table rebuild) and large balances must still round-trip.
func TestLegacySQLiteIntWalletColumnsHoldLargeBalances(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	type legacyUser struct {
		Id              int
		Quota           int `gorm:"type:int;default:0"`
		UsedQuota       int `gorm:"type:int;default:0;column:used_quota"`
		AffQuota        int `gorm:"type:int;default:0;column:aff_quota"`
		AffHistoryQuota int `gorm:"type:int;default:0;column:aff_history"`
	}
	legacy := db.Table("users")
	require.NoError(t, legacy.AutoMigrate(&legacyUser{}))

	var ddlBefore string
	require.NoError(t, db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'users'").Scan(&ddlBefore).Error)
	require.Contains(t, ddlBefore, "`quota` int")

	stmt := &gorm.Statement{DB: db}
	require.NoError(t, stmt.Parse(&User{}))
	columnTypes, err := legacy.Migrator().ColumnTypes(&legacyUser{})
	require.NoError(t, err)
	for _, column := range columnTypes {
		field := stmt.Schema.LookUpField(column.Name())
		if field == nil || column.Name() == "id" {
			continue
		}
		require.NoError(t, db.Migrator().MigrateColumn(&User{}, field, column))
	}
	var ddlAfter string
	require.NoError(t, db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'users'").Scan(&ddlAfter).Error)
	assert.Equal(t, ddlBefore, ddlAfter, "SQLite must not rebuild the users table for the wider Go type")

	user := legacyUser{Quota: 9_000_000_000_000, UsedQuota: 3_000_000_000, AffQuota: 2_200_000_000, AffHistoryQuota: 2_300_000_000}
	require.NoError(t, legacy.Create(&user).Error)
	var stored User
	require.NoError(t, db.Unscoped().Select("id", "quota", "used_quota", "aff_quota", "aff_history").First(&stored, user.Id).Error)
	assert.Equal(t, user.Quota, stored.Quota)
	assert.Equal(t, user.UsedQuota, stored.UsedQuota)
	assert.Equal(t, user.AffQuota, stored.AffQuota)
	assert.Equal(t, user.AffHistoryQuota, stored.AffHistoryQuota)
}

/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// resetRechargeFixtures clears every table that feeds the enterprise recharge
// aggregation so each case starts from a known zero.
func resetRechargeFixtures(t *testing.T) {
	t.Helper()
	cleanup := func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&TopUp{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Redemption{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&UserRechargeRecord{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Log{}).Error)
	}
	cleanup()
	t.Cleanup(cleanup)
}

// pinRechargeCurrency freezes the display currency so expected conversions are
// deterministic, restoring the previous settings when the test ends.
func pinRechargeCurrency(t *testing.T, displayType string, rate float64) {
	t.Helper()
	prevDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	prevRate := operation_setting.USDExchangeRate
	operation_setting.GetGeneralSetting().QuotaDisplayType = displayType
	operation_setting.USDExchangeRate = rate
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = prevDisplay
		operation_setting.USDExchangeRate = prevRate
	})
}

// The enterprise total must follow the balance: each channel contributes the
// quota it actually credited (not the paid money), converted with the site's
// current display currency.
func TestUserTotalRechargeFollowsCreditedQuotaAndCurrency(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&TopUp{}, &Redemption{}, &UserRechargeRecord{}))
	resetRechargeFixtures(t)
	pinRechargeCurrency(t, operation_setting.QuotaDisplayTypeCNY, 1)

	const userId = 7101
	// epay: 5 USD credited as quota; the paid money (35) is deliberately larger
	// and must not inflate the enterprise total.
	require.NoError(t, DB.Create(&TopUp{
		UserId: userId, Amount: 5, Money: 35, TradeNo: "t-epay-success",
		PaymentProvider: PaymentProviderEpay,
		Status:          common.TopUpStatusSuccess, CreateTime: common.GetTimestamp(),
	}).Error)
	// stripe: Money is the USD amount credited as quota.
	require.NoError(t, DB.Create(&TopUp{
		UserId: userId, Money: 20, TradeNo: "t-stripe-success",
		PaymentProvider: PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess, CreateTime: common.GetTimestamp(),
	}).Error)
	// creem: Amount is already a quota amount.
	require.NoError(t, DB.Create(&TopUp{
		UserId: userId, Amount: int64(3 * common.QuotaPerUnit), Money: 6, TradeNo: "t-creem-success",
		PaymentProvider: PaymentProviderCreem,
		Status:          common.TopUpStatusSuccess, CreateTime: common.GetTimestamp(),
	}).Error)
	// Pending orders never count.
	require.NoError(t, DB.Create(&TopUp{
		UserId: userId, Amount: 999, Money: 999, TradeNo: "t-epay-pending",
		PaymentProvider: PaymentProviderEpay,
		Status:          common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}).Error)
	// Redeemed code worth 10 units.
	require.NoError(t, DB.Create(&Redemption{
		UserId: userId, Key: "key-recharge-redeemed", Status: common.RedemptionCodeStatusUsed,
		Quota: int(10 * common.QuotaPerUnit), UsedUserId: userId,
	}).Error)
	// An unused code must not count.
	require.NoError(t, DB.Create(&Redemption{
		UserId: userId, Key: "key-recharge-unused", Status: common.RedemptionCodeStatusEnabled,
		Quota: int(500 * common.QuotaPerUnit),
	}).Error)
	// Admin grant worth 5 units.
	require.NoError(t, RecordAdminQuotaRecharge(userId, int(5*common.QuotaPerUnit)))
	// Another user's records must not leak into this account.
	require.NoError(t, DB.Create(&UserRechargeRecord{
		UserId: 7102, Money: 777, Quota: 777, Source: RechargeSourceAdmin,
		RefId: "admin:7102:test", CreateTime: common.GetTimestamp(),
	}).Error)

	// Credited quota: epay 5 + stripe 20 + creem 3 + redeemed 10 + admin 5 = 43.
	assert.InDelta(t, 43.0, GetUserTotalRechargeMoney(userId), 1e-6)

	// Switching the site currency changes the displayed amount accordingly.
	operation_setting.USDExchangeRate = 7
	assert.InDelta(t, 43.0*7, GetUserTotalRechargeMoney(userId), 1e-6)
}

// Backfill must recover historical admin grants from audit logs exactly once,
// honouring target_user_id and the legacy content-only rows.
func TestAdminRechargeBackfillIsIdempotentAndAttributed(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Log{}, &UserRechargeRecord{}))
	resetRechargeFixtures(t)
	pinRechargeCurrency(t, operation_setting.QuotaDisplayTypeCNY, 1)

	now := common.GetTimestamp()
	// Operator (user 1) grants 300 to themselves: no target_user_id in params.
	legacy := &Log{
		UserId: 1, CreatedAt: now, Type: LogTypeManage,
		Content: "Increased user quota by ¥300.000000 额度",
		Other:   `{"admin_info":{"admin_id":1},"op":{"action":"user.quota_add","params":{"quota":"¥300.000000 额度"}}}`,
	}
	// Operator grants 30 to a different user: target_user_id points elsewhere.
	attributed := &Log{
		UserId: 1, CreatedAt: now, Type: LogTypeManage,
		Content: "Increased user quota by ¥30.000000 额度",
		Other:   `{"op":{"action":"user.quota_add","params":{"quota":"¥30.000000 额度","target_user_id":16}}}`,
	}
	// A subtraction must not be treated as a recharge.
	subtraction := &Log{
		UserId: 1, CreatedAt: now, Type: LogTypeManage,
		Content: "Decreased user quota by ¥50.000000 额度",
	}
	for _, entry := range []*Log{legacy, attributed, subtraction} {
		require.NoError(t, DB.Create(entry).Error)
	}

	require.NoError(t, backfillAdminRechargeRecordsFromLogs([]*Log{legacy, attributed, subtraction}))
	// Running twice must not double count.
	require.NoError(t, backfillAdminRechargeRecordsFromLogs([]*Log{legacy, attributed, subtraction}))

	assert.InDelta(t, 300.0, GetUserTotalRechargeMoney(1), 1e-6, "legacy row attributed to the log owner")
	assert.InDelta(t, 30.0, GetUserTotalRechargeMoney(16), 1e-6, "attributed row follows target_user_id")

	var records int64
	require.NoError(t, DB.Model(&UserRechargeRecord{}).Count(&records).Error)
	assert.Equal(t, int64(2), records, "only the two additions are recorded")

	// The quota column is the authoritative base, not the money snapshot.
	var legacyRow UserRechargeRecord
	require.NoError(t, DB.Where("ref_id = ?", fmt.Sprintf("adminlog:%d", legacy.Id)).First(&legacyRow).Error)
	assert.EqualValues(t, int64(300*common.QuotaPerUnit), legacyRow.Quota)
}

// Legacy rows written before the quota column existed are fixed once by the
// startup backfill and must not be double counted afterwards.
func TestAdminRechargeLegacyRowsGetQuotaUnits(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&UserRechargeRecord{}))
	resetRechargeFixtures(t)
	pinRechargeCurrency(t, operation_setting.QuotaDisplayTypeCNY, 1)

	require.NoError(t, DB.Create(&UserRechargeRecord{
		UserId: 8101, Money: 10, Quota: 0, Source: RechargeSourceAdmin,
		RefId: "adminlog:test-legacy", CreateTime: common.GetTimestamp(),
	}).Error)

	// Before the migration the money fallback already produces the quota-based
	// figure, so users do not briefly lose their progress on upgrade.
	assert.InDelta(t, 10.0, GetUserTotalRechargeMoney(8101), 1e-6)

	require.NoError(t, backfillAdminRechargeRecordQuotaUnits())

	var row UserRechargeRecord
	require.NoError(t, DB.Where("ref_id = ?", "adminlog:test-legacy").First(&row).Error)
	assert.EqualValues(t, int64(10*common.QuotaPerUnit), row.Quota)
	// Still exactly one contribution after the migration.
	assert.InDelta(t, 10.0, GetUserTotalRechargeMoney(8101), 1e-6)
}

// Display strings must be decoded by the symbol they carry, so historical rows
// keep their real quota value regardless of the current display setting.
func TestDisplayStringToQuotaUnits(t *testing.T) {
	pinRechargeCurrency(t, operation_setting.QuotaDisplayTypeUSD, 1)

	units, ok := displayStringToQuotaUnits("¥300.000000 额度")
	require.True(t, ok)
	assert.EqualValues(t, int64(300*common.QuotaPerUnit), units)

	units, ok = displayStringToQuotaUnits("＄10.000000 额度")
	require.True(t, ok)
	assert.EqualValues(t, int64(10*common.QuotaPerUnit), units)

	units, ok = displayStringToQuotaUnits("5000000 点额度")
	require.True(t, ok)
	assert.EqualValues(t, int64(5000000), units)

	_, ok = displayStringToQuotaUnits("no digits here")
	assert.False(t, ok)
}

// The display-string money parser must understand both currency and token formats.
func TestParseRechargeMoneyFromDisplayString(t *testing.T) {
	cases := []struct {
		input string
		want  float64
		ok    bool
	}{
		{"¥300.000000 额度", 300, true},
		{"＄10.000000 额度", 10, true},
		{"¤25.5 额度", 25.5, true},
		{"5000000 点额度", 10, true}, // tokens display: quota / QuotaPerUnit
		{"no number here", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseRechargeMoneyFromDisplayString(tc.input)
		if !tc.ok {
			assert.False(t, ok, "input %q should fail", tc.input)
			continue
		}
		assert.True(t, ok, "input %q should parse", tc.input)
		assert.InDelta(t, tc.want, got, 1e-9, "input %q", tc.input)
	}
}

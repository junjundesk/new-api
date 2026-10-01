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

// The enterprise gate must count all three recharge channels under one display
// currency: successful online orders, redeemed codes, and admin quota grants.
func TestUserTotalRechargeSumsAllThreeSources(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&TopUp{}, &Redemption{}, &UserRechargeRecord{}))
	resetRechargeFixtures(t)

	// Pin the display currency so the expected conversion is deterministic.
	prevDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	prevRate := operation_setting.USDExchangeRate
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	operation_setting.USDExchangeRate = 1
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = prevDisplay
		operation_setting.USDExchangeRate = prevRate
	})

	const userId = 7101
	// One successful order worth 100 and one pending order that must be ignored.
	require.NoError(t, DB.Create(&TopUp{
		UserId: userId, Money: 100, TradeNo: "t-order-success",
		Status: common.TopUpStatusSuccess, CreateTime: common.GetTimestamp(),
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: userId, Money: 999, TradeNo: "t-order-pending",
		Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}).Error)
	// A redeemed code worth 10 currency units (QuotaPerUnit per unit).
	require.NoError(t, DB.Create(&Redemption{
		UserId: userId, Key: "key-redeemed-1", Status: common.RedemptionCodeStatusUsed,
		Quota: int(10 * common.QuotaPerUnit), UsedUserId: userId,
	}).Error)
	// An unused code must not count.
	require.NoError(t, DB.Create(&Redemption{
		UserId: userId, Key: "key-unused-1", Status: common.RedemptionCodeStatusEnabled,
		Quota: int(500 * common.QuotaPerUnit),
	}).Error)
	// An admin grant worth 5.
	require.NoError(t, RecordAdminQuotaRecharge(userId, int(5*common.QuotaPerUnit)))

	// Another user's records must not leak into this account.
	require.NoError(t, DB.Create(&UserRechargeRecord{
		UserId: 7102, Money: 777, Source: RechargeSourceAdmin,
		RefId: "admin:7102:1", CreateTime: common.GetTimestamp(),
	}).Error)

	total := GetUserTotalRechargeMoney(userId)
	assert.InDelta(t, 115.0, total, 1e-9, "100 order + 10 redeemed code + 5 admin grant")
}

// Backfill must recover historical admin grants from audit logs exactly once,
// honouring target_user_id and the legacy content-only rows.
func TestAdminRechargeBackfillIsIdempotentAndAttributed(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Log{}, &UserRechargeRecord{}))
	resetRechargeFixtures(t)

	prevDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	prevRate := operation_setting.USDExchangeRate
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	operation_setting.USDExchangeRate = 1
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = prevDisplay
		operation_setting.USDExchangeRate = prevRate
	})

	now := common.GetTimestamp()
	// Operator (user 1) grants 300 to user 1: no target_user_id in params.
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

	assert.InDelta(t, 300.0, GetUserTotalRechargeMoney(1), 1e-9, "legacy row attributed to the log owner")
	assert.InDelta(t, 30.0, GetUserTotalRechargeMoney(16), 1e-9, "attributed row follows target_user_id")

	var records int64
	require.NoError(t, DB.Model(&UserRechargeRecord{}).Count(&records).Error)
	assert.Equal(t, int64(2), records, "only the two additions are recorded")
}

// The display-string parser must understand both currency and token formats.
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

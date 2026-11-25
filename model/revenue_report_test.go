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
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRevenueDatabases(t *testing.T) {
	t.Helper()
	open := func(models ...any) *gorm.DB {
		t.Helper()
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
		require.NoError(t, db.AutoMigrate(models...))
		return db
	}
	oldMain, oldLog := DB, LOG_DB
	t.Cleanup(func() { DB, LOG_DB = oldMain, oldLog })
	DB = open(&Option{}, &TopUp{}, &SubscriptionOrder{}, &Redemption{}, &UserRechargeRecord{})
	LOG_DB = open(&Log{})
}

func TestRevenueReportUsesNetBilledQuotaAndKeepsWalletInflowsSeparate(t *testing.T) {
	setupRevenueDatabases(t)
	ctx := context.Background()
	oldUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = oldUnit })
	pinRechargeCurrency(t, operation_setting.QuotaDisplayTypeUSD, 1)
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, revenueZone)
	ts := now.Unix()
	settings := RevenueCostSettings{Groups: []RevenueGroupRate{
		{Group: "pro", CostRatio: "0.6", Models: []RevenueModelRate{{Model: "special", CostRatio: "1.25"}, {Model: "free", CostRatio: "0"}}},
		{Group: "retail", CostRatio: "0.5"},
	}}
	require.NoError(t, SaveRevenueCostSettings(ctx, settings))
	for _, row := range []Log{
		{Type: LogTypeConsume, Group: "pro", ModelName: "special", Quota: 100},
		{Type: LogTypeRefund, Group: "pro", ModelName: "special", Quota: 20},
		{Type: LogTypeConsume, Group: "pro", ModelName: "ordinary", Quota: 50},
		{Type: LogTypeConsume, Group: "pro", ModelName: "free", Quota: 10},
		{Type: LogTypeConsume, Group: "retail", ModelName: "special", Quota: 30},
		{Type: LogTypeConsume, Group: "legacy", ModelName: "special", Quota: 20},
		{Type: LogTypeTopup, Quota: 9999}, {Type: LogTypeManage, Quota: 9999}, {Type: LogTypeError, Quota: 9999},
	} {
		row.CreatedAt = ts
		require.NoError(t, LOG_DB.Create(&row).Error)
	}
	for _, order := range []TopUp{
		{TradeNo: "snapshot", Amount: 999, CreditedQuota: 123, PaymentProvider: PaymentProviderCreem},
		{TradeNo: "epay", Amount: 2, Money: 14, PaymentProvider: PaymentProviderEpay},
		{TradeNo: "legacy-stripe", Amount: 7, Money: 1.25, PaymentMethod: PaymentMethodStripe},
		{TradeNo: "creem", Amount: 150, Money: 4, PaymentProvider: PaymentProviderCreem},
		{TradeNo: "pending", Amount: 1000, Status: common.TopUpStatusPending},
		{TradeNo: "wallet-purchase", Amount: 50, PaymentMethod: PaymentMethodBalance},
		{TradeNo: "subscription-mirror", Money: 12, PaymentProvider: PaymentProviderStripe},
		{TradeNo: "older", Amount: 1, CompleteTime: ts - 86400},
	} {
		if order.Status == "" {
			order.Status = common.TopUpStatusSuccess
		}
		if order.CompleteTime == 0 {
			order.CompleteTime = ts
		}
		order.CreateTime = ts - 86400
		require.NoError(t, DB.Create(&order).Error)
	}
	require.NoError(t, DB.Create(&SubscriptionOrder{TradeNo: "subscription-mirror"}).Error)
	used := Redemption{Key: "already-deleted", Status: common.RedemptionCodeStatusUsed, UsedUserId: 1, Quota: 12, BonusQuota: 8, RedeemedTime: ts}
	require.NoError(t, DB.Create(&used).Error)
	require.NoError(t, DB.Delete(&used).Error)
	require.NoError(t, DB.Create(&Redemption{Key: "unused", Quota: 100, Status: common.RedemptionCodeStatusEnabled, RedeemedTime: ts}).Error)
	for _, r := range []UserRechargeRecord{{Source: RechargeSourceAdmin, RefId: "admin1", Quota: 30, Money: 99}, {Source: RechargeSourceAdmin, RefId: "legacy", Money: 0.25}, {Source: "other", RefId: "unrelated", Quota: 90}} {
		r.CreateTime = ts
		require.NoError(t, DB.Create(&r).Error)
	}
	report, err := GetRevenueReport(ctx, "today", now)
	require.NoError(t, err)
	require.Len(t, report.Days, 1)
	require.Len(t, report.Items, 5)
	totals := report.Summary
	assert.Equal(t, "190", totals.Total.String())
	assert.Equal(t, "20", totals.Unpriced.String())
	assert.Equal(t, "145", totals.Cost.String())
	assert.Equal(t, "25", totals.Profit.String())
	assert.Equal(t, "598", totals.Online.String())
	assert.Equal(t, "55", totals.Admin.String())
	assert.Equal(t, "12", totals.Redeemed.String())
	assert.Equal(t, "665", totals.Credited.String())
	assert.Equal(t, "2026-01-02", report.Days[0].Date)
	assert.Equal(t, "665", report.Days[0].Credited.String())
	assert.Equal(t, "25", report.Days[0].Profit.String())
	for _, item := range report.Items {
		if item.Group == "pro" && item.Model == "special" {
			assert.Equal(t, "-20", item.Profit.String())
		}
		if item.Group == "legacy" {
			assert.Nil(t, item.Ratio)
		}
	}
	yesterday, err := GetRevenueReport(ctx, "yesterday", now)
	require.NoError(t, err)
	require.Len(t, yesterday.Days, 1)
	assert.Equal(t, "100", yesterday.Summary.Credited.String())
	assert.Equal(t, "0", yesterday.Summary.Total.String())
	// Rates are reporting assumptions: reprice the selected history without
	// touching original charges, refunds, or credits.
	settings.Groups[0].Models = nil
	require.NoError(t, SaveRevenueCostSettings(ctx, settings))
	changed, err := GetRevenueReport(ctx, "today", now)
	require.NoError(t, err)
	assert.Equal(t, "99", changed.Summary.Cost.String())
	assert.Equal(t, "190", changed.Summary.Total.String())
	assert.Equal(t, "665", changed.Summary.Credited.String())
}

func TestRevenueReportCalendarBoundaries(t *testing.T) {
	setupRevenueDatabases(t)
	now := time.Date(2025, 1, 1, 16, 0, 0, 0, time.UTC) // midnight Jan 2 in UTC+8
	for _, at := range []time.Time{now, now.Add(-time.Second), now.AddDate(0, 0, -6), now.AddDate(0, 0, -7), now.AddDate(0, 0, -29), now.AddDate(0, 0, -30), now.Add(time.Second)} {
		require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, CreatedAt: at.Unix(), Group: "pro", ModelName: "m", Quota: 50}).Error)
	}
	for _, tc := range []struct {
		period, date, total string
		days                int
	}{{"today", "2025-01-02", "50", 1}, {"yesterday", "2025-01-01", "50", 1}, {"7d", "2025-01-02", "150", 3}, {"30d", "2025-01-02", "250", 5}, {"all", "2025-01-02", "300", 6}} {
		report, err := GetRevenueReport(context.Background(), tc.period, now)
		require.NoError(t, err)
		require.Len(t, report.Days, tc.days)
		assert.Equal(t, tc.date, report.Days[0].Date)
		assert.Equal(t, tc.total, report.Summary.Total.String())
		assert.Equal(t, tc.total, report.Summary.Unpriced.String())
	}
	_, err := GetRevenueReport(context.Background(), "weekly", now)
	assert.ErrorIs(t, err, ErrRevenueRange)
}

func TestRevenueInvalidRatesLeavePreviousBindingsIntact(t *testing.T) {
	setupRevenueDatabases(t)
	ctx := context.Background()
	valid := RevenueGroupRate{Group: "default", CostRatio: "0.1234", Models: []RevenueModelRate{{Model: "m", CostRatio: "0"}}}
	require.NoError(t, SaveRevenueCostSettings(ctx, RevenueCostSettings{[]RevenueGroupRate{valid}}))
	for _, s := range []RevenueCostSettings{{}, {[]RevenueGroupRate{valid, valid}}, {[]RevenueGroupRate{{Group: "group", CostRatio: "1", Models: []RevenueModelRate{{Model: "m", CostRatio: "0"}, {Model: " m ", CostRatio: "1"}}}}}, {[]RevenueGroupRate{{Group: "  ", CostRatio: "1"}}}} {
		require.Error(t, SaveRevenueCostSettings(ctx, s))
	}
	for _, raw := range []string{"", "-0.5", "NaN", "+Inf", "1e2", "0.00001", "1000.01"} {
		require.Error(t, SaveRevenueCostSettings(ctx, RevenueCostSettings{[]RevenueGroupRate{{Group: "d", CostRatio: raw}}}))
	}
	saved, err := GetRevenueCostSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, []RevenueGroupRate{valid}, saved.Groups)
}

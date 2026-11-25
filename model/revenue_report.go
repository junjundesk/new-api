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
	"errors"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var revenueZone = time.FixedZone("UTC+8", 8*60*60)
var ErrRevenueRange = errors.New("invalid revenue report range")

// Closed-open Unix seconds, including today's partial day but never future
// events. Server-side boundaries are shared by the main DB and log DB.
func revenueWindow(period string, now time.Time) (int64, int64, error) {
	now = now.In(revenueZone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, revenueZone)
	end := now.Unix() + 1
	switch period {
	case "today":
		return today.Unix(), end, nil
	case "yesterday":
		return today.AddDate(0, 0, -1).Unix(), today.Unix(), nil
	case "7d":
		return today.AddDate(0, 0, -6).Unix(), end, nil
	case "30d":
		return today.AddDate(0, 0, -29).Unix(), end, nil
	case "all":
		return 1, end, nil
	default:
		return 0, 0, ErrRevenueRange
	}
}

// Integer remainder works on SQLite, MySQL, PostgreSQL and ClickHouse. The
// column is always an internal timestamp column, never a query parameter.
func revenueDayExpr(column string) string {
	return column + " - ((" + column + " + 28800) % 86400) AS day_start"
}

type RevenueAmounts struct {
	Total    decimal.Decimal `json:"total"` // Quota actually charged, net of refunds, including unpriced groups.
	Cost     decimal.Decimal `json:"cost"`
	Profit   decimal.Decimal `json:"profit"` // Priced portion only.
	Unpriced decimal.Decimal `json:"unpriced"`
}

func (a *RevenueAmounts) add(b RevenueAmounts) {
	a.Total = a.Total.Add(b.Total)
	a.Cost = a.Cost.Add(b.Cost)
	a.Profit = a.Profit.Add(b.Profit)
	a.Unpriced = a.Unpriced.Add(b.Unpriced)
}

type RevenueTotals struct {
	RevenueAmounts
	Online   decimal.Decimal `json:"online"`
	Admin    decimal.Decimal `json:"admin"`
	Redeemed decimal.Decimal `json:"redeemed"`
	Credited decimal.Decimal `json:"credited"` // Wallet inflows, NOT additional sales or verified cash receipts.
}
type RevenueDay struct {
	Date string `json:"date"`
	RevenueTotals
}
type RevenueItem struct {
	Group string           `json:"group"`
	Model string           `json:"model"`
	Ratio *decimal.Decimal `json:"cost_ratio"` // null = no binding, not a zero-cost upstream.
	RevenueAmounts
}
type RevenueReport struct {
	AsOf    int64          `json:"as_of"`
	Summary RevenueTotals  `json:"summary"`
	Days    []*RevenueDay  `json:"days"`
	Items   []*RevenueItem `json:"items"`
}

func GetRevenueReport(ctx context.Context, period string, now time.Time) (*RevenueReport, error) {
	start, end, err := revenueWindow(period, now)
	if err != nil {
		return nil, err
	}
	settings, err := GetRevenueCostSettings(ctx)
	if err != nil {
		return nil, err
	}
	rates, err := settings.rates()
	if err != nil {
		return nil, err
	}
	report := &RevenueReport{AsOf: now.Unix(), Days: []*RevenueDay{}, Items: []*RevenueItem{}}
	days := map[int64]*RevenueDay{}
	dayFor := func(ts int64) *RevenueDay {
		if days[ts] == nil {
			days[ts] = &RevenueDay{Date: time.Unix(ts, 0).In(revenueZone).Format(time.DateOnly)}
			report.Days = append(report.Days, days[ts])
		}
		return days[ts]
	}
	// Aggregate in the actual log database, not quota_data (which is optional,
	// asynchronous and not a ledger). Refund logs store a positive amount.
	var usage []struct {
		DayStart              int64
		UseGroup, BilledModel string
		NetQuota              decimal.Decimal
	}
	groupExpr, modelExpr := "COALESCE("+logGroupCol+", '')", "COALESCE(model_name, '')"
	if common.UsingLogDatabase(common.DatabaseTypeMySQL) {
		// Case-insensitive collations must not merge distinct model IDs/rules.
		groupExpr, modelExpr = "BINARY "+groupExpr, "BINARY "+modelExpr
	}
	usageSelect := revenueDayExpr("created_at") + ", " + groupExpr + " AS use_group, " + modelExpr +
		" AS billed_model, SUM(CASE WHEN type = ? THEN -quota ELSE quota END) AS net_quota"
	err = LOG_DB.WithContext(ctx).Model(&Log{}).Select(usageSelect, LogTypeRefund).
		Where("created_at >= ? AND created_at < ? AND type IN ?", start, end, []int{LogTypeConsume, LogTypeRefund}).Group("day_start, use_group, billed_model").Scan(&usage).Error
	if err != nil {
		return nil, err
	}
	items := make(map[revenueKey]*RevenueItem)
	for _, u := range usage {
		key := revenueKey{u.UseGroup, u.BilledModel}
		item := items[key]
		if item == nil {
			item = &RevenueItem{Group: u.UseGroup, Model: u.BilledModel}
			ratio, ok := rates[key]
			if !ok {
				ratio, ok = rates[revenueKey{group: u.UseGroup}]
			}
			if ok {
				item.Ratio = &ratio
			}
			items[key] = item
			report.Items = append(report.Items, item)
		}
		a := RevenueAmounts{Total: u.NetQuota}
		if item.Ratio == nil {
			a.Unpriced = u.NetQuota
		} else {
			a.Cost = u.NetQuota.Mul(*item.Ratio)
			a.Profit = u.NetQuota.Sub(a.Cost)
		}
		item.add(a)
		dayFor(u.DayStart).add(a)
	}
	// Use the original funding records, never the descriptive top-up logs or
	// current user balances. A subscription checkout can mirror a top_up row
	// without crediting the wallet; it must not be counted as a wallet inflow.
	db := DB.WithContext(ctx)
	topups := db.Model(&TopUp{}).Where("status = ? AND COALESCE(payment_method,'') <> ? AND COALESCE(payment_provider,'') <> ?", common.TopUpStatusSuccess, PaymentMethodBalance, PaymentProviderBalance).
		Where("NOT EXISTS (?)", db.Model(&SubscriptionOrder{}).Select("1").Where("subscription_orders.trade_no = top_ups.trade_no"))
	codes := db.Unscoped().Model(&Redemption{}).Where("status = ? AND used_user_id > 0", common.RedemptionCodeStatusUsed)
	admin := db.Model(&UserRechargeRecord{}).Where("source = ?", RechargeSourceAdmin)
	// Older admin rows only have a display-money snapshot. Use the same legacy
	// conversion as enterprise cumulative recharge, rounding each grant to quota.
	// Keep the result of each legacy conversion integral; otherwise a REAL
	// branch can coerce even the int64 snapshots to floats before SUM.
	integerType := "BIGINT"
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		integerType = "SIGNED"
	}
	roundUnits := func(expr string) string { return "CAST(ROUND(" + expr + ") AS " + integerType + ")" }
	// Before payment_provider existed, Stripe/Creem were identified by method.
	provider := "COALESCE(NULLIF(payment_provider,''), payment_method)"
	topupAmount := "SUM(CASE WHEN credited_quota > 0 THEN credited_quota WHEN " + provider + " = ? THEN " + roundUnits("money * ?") +
		" WHEN " + provider + " = ? THEN amount ELSE " + roundUnits("amount * ?") + " END)"
	adminAmount := "SUM(CASE WHEN quota > 0 THEN quota WHEN money > 0 THEN " + roundUnits("money * ?") + " ELSE 0 END)"
	creditSources := []struct {
		query             *gorm.DB
		timestamp, amount string
		args              []any
		apply             func(*RevenueDay, decimal.Decimal)
	}{
		{topups, "complete_time", topupAmount, []any{PaymentProviderStripe, common.QuotaPerUnit, PaymentProviderCreem, common.QuotaPerUnit}, func(d *RevenueDay, v decimal.Decimal) { d.Online = v }},
		{codes, "redeemed_time", "SUM(quota)", nil, func(d *RevenueDay, v decimal.Decimal) { d.Redeemed = v }},
		{admin, "create_time", adminAmount, []any{moneyToQuotaUnitsAtCurrentRate(1)}, func(d *RevenueDay, v decimal.Decimal) { d.Admin = v }},
	}
	for _, source := range creditSources {
		var amounts []struct {
			DayStart int64
			Units    decimal.Decimal
		}
		err := source.query.Select(revenueDayExpr(source.timestamp)+", "+source.amount+" AS units", source.args...).Where(source.timestamp+" >= ? AND "+source.timestamp+" < ?", start, end).Group("day_start").Scan(&amounts).Error
		if err != nil {
			return nil, err
		}
		for _, row := range amounts {
			source.apply(dayFor(row.DayStart), row.Units)
		}
	}
	for _, d := range report.Days {
		d.Credited = d.Online.Add(d.Redeemed).Add(d.Admin)
		report.Summary.add(d.RevenueAmounts)
		report.Summary.Online = report.Summary.Online.Add(d.Online)
		report.Summary.Redeemed = report.Summary.Redeemed.Add(d.Redeemed)
		report.Summary.Admin = report.Summary.Admin.Add(d.Admin)
		report.Summary.Credited = report.Summary.Credited.Add(d.Credited)
	}
	sort.Slice(report.Days, func(i, j int) bool { return report.Days[i].Date > report.Days[j].Date })
	sort.Slice(report.Items, func(i, j int) bool {
		a, b := report.Items[i], report.Items[j]
		if a.Group != b.Group {
			return a.Group < b.Group
		}
		return a.Model < b.Model
	})
	return report, nil
}

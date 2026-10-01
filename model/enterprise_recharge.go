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
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// UserRechargeRecord 是企业累充的补充记账表。在线充值成功订单与兑换码可直接从
// top_ups / redemptions 汇总，只有「管理员增加额度」没有独立持久记录，因此在加额时
// 写入本表，并在启动时从历史审计日志回填。
//
// Quota 是统计口径的唯一基准（额度单位），Money 仅是写入当时展示货币下的快照，
// 供审计参考；累充金额在查询时按服务器当前货币设置换算，与余额展示保持一致。
type UserRechargeRecord struct {
	Id         int     `json:"id"`
	UserId     int     `json:"user_id" gorm:"index"`
	Money      float64 `json:"money"`
	Quota      int64   `json:"quota"`
	Source     string  `json:"source" gorm:"type:varchar(32)"`
	RefId      string  `json:"ref_id" gorm:"uniqueIndex;type:varchar(128)"`
	CreateTime int64   `json:"create_time" gorm:"bigint"`
}

const (
	RechargeSourceAdmin = "admin"
	// adminRechargeBackfillFlag 标记历史管理员加额日志已回填，避免每次启动重复扫描。
	adminRechargeBackfillFlag = "EnterpriseAdminRechargeBackfillV1"
	// adminRechargeQuotaBackfillFlag 标记已回填记录的额度列补全完成（旧版本只存了
	// Money 快照，Quota 为空，需要按当前货币口径反算补上）。
	adminRechargeQuotaBackfillFlag = "EnterpriseAdminRechargeQuotaBackfillV1"
)

// quotaUnitsToDisplayMoney 将额度按当前展示口径换算为金额，与 LogQuota / 余额展示
// 的换算保持一致：CNY 使用 USDExchangeRate，CUSTOM 使用自定义汇率，USD/TOKENS 按美元计。
func quotaUnitsToDisplayMoney(units float64) float64 {
	usd := units / common.QuotaPerUnit
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		return usd * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeCustom:
		rate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
		if rate <= 0 {
			rate = 1
		}
		return usd * rate
	default:
		return usd
	}
}

// moneyToQuotaUnitsAtCurrentRate 是 quotaUnitsToDisplayMoney 的反向换算：把一份
// 展示货币金额按当前设置折回额度单位，用于修复早期只存金额快照的记账行。
func moneyToQuotaUnitsAtCurrentRate(money float64) float64 {
	if money <= 0 {
		return 0
	}
	rate := 1.0
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		if operation_setting.USDExchangeRate > 0 {
			rate = operation_setting.USDExchangeRate
		}
	case operation_setting.QuotaDisplayTypeCustom:
		if r := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate; r > 0 {
			rate = r
		}
	}
	return money / rate * common.QuotaPerUnit
}

// getUserTotalRechargeQuotaUnits 汇总用户通过三类渠道实际获得额度的额度数：
//  1. 在线充值成功订单——按各渠道真实入账公式反推（stripe: money * QuotaPerUnit；
//     creem: Amount 本身即额度单位；其余: Amount * QuotaPerUnit），因此统计的是
//     「入账额度」而非「实付金额」，与余额口径一致；
//  2. 已兑换的兑换码（redemptions.quota）；
//  3. 管理员增加额度（user_recharge_records.quota，历史行回退按当前汇率反算）。
//
// 任一来源查询失败时按 0 计，不阻断调用方。
func getUserTotalRechargeQuotaUnits(userId int) float64 {
	total := 0.0

	var topUpUnits *float64
	err := DB.Model(&TopUp{}).
		Select(`COALESCE(SUM(CASE
			WHEN payment_provider = ? THEN money * ?
			WHEN payment_provider = ? THEN amount * ?
			ELSE amount * ? END), 0)`,
			PaymentProviderStripe, common.QuotaPerUnit,
			PaymentProviderCreem, 1.0,
			common.QuotaPerUnit).
		Where("user_id = ? AND status = ?", userId, common.TopUpStatusSuccess).
		Scan(&topUpUnits).Error
	if err == nil && topUpUnits != nil {
		total += *topUpUnits
	}

	var redeemedQuota *int64
	err = DB.Model(&Redemption{}).
		Unscoped().
		Select("COALESCE(SUM(quota), 0)").
		Where("used_user_id = ? AND status = ?", userId, common.RedemptionCodeStatusUsed).
		Scan(&redeemedQuota).Error
	if err == nil && redeemedQuota != nil {
		total += float64(*redeemedQuota)
	}

	var adminQuota *int64
	err = DB.Model(&UserRechargeRecord{}).
		Select("COALESCE(SUM(quota), 0)").
		Where("user_id = ? AND quota > 0", userId).
		Scan(&adminQuota).Error
	if err == nil && adminQuota != nil {
		total += float64(*adminQuota)
	}

	// 早期回填行只有 Money 快照、Quota 为空，按当前货币口径反算兜底。
	var legacyMoney *float64
	err = DB.Model(&UserRechargeRecord{}).
		Select("COALESCE(SUM(money), 0)").
		Where("user_id = ? AND quota <= 0 AND money > 0", userId).
		Scan(&legacyMoney).Error
	if err == nil && legacyMoney != nil {
		total += moneyToQuotaUnitsAtCurrentRate(*legacyMoney)
	}

	return total
}

// GetUserTotalRechargeMoney 返回用户累计充值金额，单位与展示货币一致（跟随服务器
// 当前货币设置实时换算，与余额展示同一口径）。TOKENS 展示模式下按美元金额换算，
// 与门槛（金额语义）保持同量纲比较。包含在线充值、兑换码、管理员加额三类来源；
// 查询失败时按 0 处理，不阻断主流程。
func GetUserTotalRechargeMoney(userId int) float64 {
	return quotaUnitsToDisplayMoney(getUserTotalRechargeQuotaUnits(userId))
}

// RecordAdminQuotaRecharge 记录一笔管理员增加额度的累充记账（以额度为单位）。
// 调用方在加额成功与审计日志写入之后调用；失败只记日志，不影响加额主流程。
func RecordAdminQuotaRecharge(userId int, quota int) error {
	if userId <= 0 || quota <= 0 {
		return nil
	}
	record := &UserRechargeRecord{
		UserId:     userId,
		Money:      quotaUnitsToDisplayMoney(float64(quota)),
		Quota:      int64(quota),
		Source:     RechargeSourceAdmin,
		RefId:      fmt.Sprintf("admin:%d:%d:%d", userId, time.Now().UnixNano(), quota),
		CreateTime: common.GetTimestamp(),
	}
	return DB.Create(record).Error
}

// parseRechargeMoneyFromDisplayString 解析 LogQuota 生成的展示串为金额数额。
// 形如 "¥300.000000 额度"（金额）、"5000000 点额度"（额度点数，按美元折算）。
func parseRechargeMoneyFromDisplayString(s string) (float64, bool) {
	value, ok := extractFirstNumber(s)
	if !ok {
		return 0, false
	}
	if strings.Contains(s, "点额度") {
		return value / common.QuotaPerUnit, true
	}
	return value, true
}

// displayStringToQuotaUnits 把 LogQuota 展示串反算为额度单位。判据是写入当时的
// 货币符号：¥ 按美元汇率折算、＄ 视为美元、自定义符号按自定义汇率，点额度则是
// 额度数本身。汇率取当前设置（历史行无法还原写入时汇率，按当前口径近似）。
func displayStringToQuotaUnits(s string) (int64, bool) {
	value, ok := extractFirstNumber(s)
	if !ok || value <= 0 {
		return 0, false
	}
	if strings.Contains(s, "点额度") {
		return int64(common.QuotaRound(value)), true
	}
	usd := value
	switch {
	case strings.Contains(s, "¥"):
		rate := operation_setting.USDExchangeRate
		if rate <= 0 {
			rate = 1
		}
		usd = value / rate
	case strings.Contains(s, "＄"), strings.Contains(s, "$"):
		usd = value
	default:
		symbol := operation_setting.GetGeneralSetting().CustomCurrencySymbol
		rate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
		if symbol != "" && rate > 0 && strings.Contains(s, symbol) {
			usd = value / rate
		}
	}
	return int64(common.QuotaRound(usd * common.QuotaPerUnit)), true
}

func extractFirstNumber(s string) (float64, bool) {
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, false
	}
	end := start
	for end < len(s) {
		c := s[end]
		if (c >= '0' && c <= '9') || c == '.' {
			end++
			continue
		}
		break
	}
	value, err := strconv.ParseFloat(s[start:end], 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// backfillAdminRechargeRecordsFromLogs 把历史「管理员增加额度」审计日志补入记账表。
// 目标用户取审计参数 target_user_id（缺省为日志 user_id），金额与额度均按展示串
// 解析；以 RefId=adminlog:<logId> 去重，可安全重复执行。
func backfillAdminRechargeRecordsFromLogs(logs []*Log) error {
	for _, entry := range logs {
		if entry == nil {
			continue
		}
		targetUserId := entry.UserId
		displayValue := ""
		action := ""
		if entry.Other != "" {
			var meta map[string]interface{}
			if err := common.Unmarshal([]byte(entry.Other), &meta); err == nil {
				if op, ok := meta["op"].(map[string]interface{}); ok {
					if actionText, ok := op["action"].(string); ok {
						action = actionText
					}
					if params, ok := op["params"].(map[string]interface{}); ok {
						switch target := params["target_user_id"].(type) {
						case float64:
							targetUserId = int(target)
						case string:
							if parsed, err := strconv.Atoi(target); err == nil {
								targetUserId = parsed
							}
						}
						if quotaText, ok := params["quota"].(string); ok {
							displayValue = quotaText
						}
					}
				}
			}
		}
		// 只有「增加额度」才计入：结构化 action 优先，缺失时以内容前缀为判据，
		// 避免把减额（user.quota_subtract / "Decreased ..."）等其他审计行误算为充值。
		if action != "" {
			if action != "user.quota_add" {
				continue
			}
		} else {
			if !strings.HasPrefix(entry.Content, "Increased user quota by") {
				continue
			}
			displayValue = strings.TrimPrefix(entry.Content, "Increased user quota by")
		}
		money, moneyOk := parseRechargeMoneyFromDisplayString(displayValue)
		if !moneyOk || money <= 0 || targetUserId <= 0 {
			continue
		}
		quotaUnits, quotaOk := displayStringToQuotaUnits(displayValue)
		if !quotaOk || quotaUnits <= 0 {
			quotaUnits = int64(common.QuotaRound(moneyToQuotaUnitsAtCurrentRate(money)))
		}

		refId := fmt.Sprintf("adminlog:%d", entry.Id)
		var count int64
		if err := DB.Model(&UserRechargeRecord{}).Where("ref_id = ?", refId).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		record := &UserRechargeRecord{
			UserId:     targetUserId,
			Money:      money,
			Quota:      quotaUnits,
			Source:     RechargeSourceAdmin,
			RefId:      refId,
			CreateTime: entry.CreatedAt,
		}
		if err := DB.Create(record).Error; err != nil {
			return err
		}
	}
	return nil
}

// backfillAdminRechargeRecordQuotaUnits 为早期只存了金额快照（Quota 为空）的记账行
// 补全额度列，使累充统计统一走额度口径。
func backfillAdminRechargeRecordQuotaUnits() error {
	var rows []*UserRechargeRecord
	if err := DB.Where("quota <= ? AND money > ?", 0, 0).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil || row.Money <= 0 {
			continue
		}
		units := int64(common.QuotaRound(moneyToQuotaUnitsAtCurrentRate(row.Money)))
		if units <= 0 {
			continue
		}
		if err := DB.Model(&UserRechargeRecord{}).Where("id = ?", row.Id).Update("quota", units).Error; err != nil {
			return err
		}
	}
	return nil
}

// BackfillAdminRechargeRecords 在启动时执行历史管理员加额回填（仅在日志库非
// ClickHouse 时）。先扫描审计日志补齐缺失记录，再为旧记录补全额度列；两步各自
// 完成后写入标记，后续启动跳过。
func BackfillAdminRechargeRecords() error {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return nil
	}

	var flag Option
	if err := DB.Where("key = ?", adminRechargeBackfillFlag).First(&flag).Error; err != nil || flag.Value != "true" {
		var logs []*Log
		if err := LOG_DB.Model(&Log{}).
			Where("type = ? AND content LIKE ?", LogTypeManage, "Increased user quota by%").
			Find(&logs).Error; err != nil {
			return err
		}
		if err := backfillAdminRechargeRecordsFromLogs(logs); err != nil {
			return err
		}
		if err := UpdateOption(adminRechargeBackfillFlag, "true"); err != nil {
			return err
		}
		common.SysLog(fmt.Sprintf("enterprise recharge backfill finished, %d admin quota add logs processed", len(logs)))
	}

	var quotaFlag Option
	if err := DB.Where("key = ?", adminRechargeQuotaBackfillFlag).First(&quotaFlag).Error; err != nil || quotaFlag.Value != "true" {
		if err := backfillAdminRechargeRecordQuotaUnits(); err != nil {
			return err
		}
		if err := UpdateOption(adminRechargeQuotaBackfillFlag, "true"); err != nil {
			return err
		}
		common.SysLog("enterprise recharge quota units backfill finished")
	}
	return nil
}

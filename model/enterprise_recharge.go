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

// UserRechargeRecord 是企业累充的补充记账表。在线充值成功金额可直接汇总
// top_ups.money，兑换码可直接汇总 redemptions.quota，两者都是持久表、可实时统计；
// 只有「管理员增加额度」没有独立持久记录，因此在加额时写入本表，并在启动时从
// 历史审计日志回填。金额统一按当前展示口径折算（与 LogQuota 一致）。
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
)

// quotaUnitsToDisplayMoney 将额度按当前展示口径换算为金额，与 LogQuota 的展示换算
// 保持一致：CNY 使用 USDExchangeRate，CUSTOM 使用自定义汇率，USD/TOKENS 按美元计。
func quotaUnitsToDisplayMoney(units int64) float64 {
	usd := float64(units) / common.QuotaPerUnit
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

// GetUserTotalRechargeMoney 统计用户累计充值金额（当前展示口径），包含三类来源：
// 1. 在线充值成功订单（top_ups.money）；
// 2. 已兑换的兑换码（redemptions.quota，按展示汇率折算）；
// 3. 管理员增加额度（user_recharge_records，含历史回填）。
// 任一来源查询失败时按 0 处理，不阻断主流程。
func GetUserTotalRechargeMoney(userId int) float64 {
	total := 0.0

	var topUpTotal *float64
	err := DB.Model(&TopUp{}).
		Select("COALESCE(SUM(money), 0)").
		Where("user_id = ? AND status = ?", userId, common.TopUpStatusSuccess).
		Scan(&topUpTotal).Error
	if err == nil && topUpTotal != nil {
		total += *topUpTotal
	}

	var redeemedQuota *int64
	err = DB.Model(&Redemption{}).
		Unscoped().
		Select("COALESCE(SUM(quota), 0)").
		Where("used_user_id = ? AND status = ?", userId, common.RedemptionCodeStatusUsed).
		Scan(&redeemedQuota).Error
	if err == nil && redeemedQuota != nil {
		total += quotaUnitsToDisplayMoney(*redeemedQuota)
	}

	var adminTotal *float64
	err = DB.Model(&UserRechargeRecord{}).
		Select("COALESCE(SUM(money), 0)").
		Where("user_id = ?", userId).
		Scan(&adminTotal).Error
	if err == nil && adminTotal != nil {
		total += *adminTotal
	}

	return total
}

// RecordAdminQuotaRecharge 记录一笔管理员增加额度的累充记账。调用方在加额成功与
// 审计日志写入之后调用；失败只记日志，不影响加额主流程。
func RecordAdminQuotaRecharge(userId int, quota int) error {
	if userId <= 0 || quota <= 0 {
		return nil
	}
	record := &UserRechargeRecord{
		UserId:     userId,
		Money:      quotaUnitsToDisplayMoney(int64(quota)),
		Quota:      int64(quota),
		Source:     RechargeSourceAdmin,
		RefId:      fmt.Sprintf("admin:%d:%d:%d", userId, time.Now().UnixNano(), quota),
		CreateTime: common.GetTimestamp(),
	}
	return DB.Create(record).Error
}

// parseRechargeMoneyFromDisplayString 解析 LogQuota 生成的展示串为累充金额。
// 形如 "¥300.000000 额度"（金额）、"3000000 点额度"（额度点数，按美元折算）。
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
// 目标用户取审计参数 target_user_id（缺省为日志 user_id），金额解析自展示串；
// 以 RefId=adminlog:<logId> 去重，可安全重复执行。
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
		money, ok := parseRechargeMoneyFromDisplayString(displayValue)
		if !ok || targetUserId <= 0 || money <= 0 {
			continue
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

// BackfillAdminRechargeRecords 在启动时执行一次历史管理员加额回填（仅在主库为
// 支持的数据库且日志库非 ClickHouse 时）。完成后写入标记，后续启动跳过。
func BackfillAdminRechargeRecords() error {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return nil
	}
	var flag Option
	if err := DB.Where("key = ?", adminRechargeBackfillFlag).First(&flag).Error; err == nil && flag.Value == "true" {
		return nil
	}
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
	return nil
}

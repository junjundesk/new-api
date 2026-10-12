package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// GetReferralSummary returns the invite page payload: claimable and pending
// reward, the invitee counts, and the tier ladder so the UI can show progress.
func GetReferralSummary(c *gin.Context) {
	setting := operation_setting.GetReferralSetting()
	if !setting.Enabled {
		c.JSON(200, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"enabled": false,
			},
		})
		return
	}

	userId := c.GetInt("id")
	summary, err := model.GetReferralSummary(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	user, err := model.GetUserById(userId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"enabled":                true,
			"aff_code":               user.AffCode,
			"pending_quota":          summary.PendingQuota,
			"total_quota":            summary.TotalQuota,
			"claimable_quota":        user.AffQuota,
			"history_quota":          user.AffHistoryQuota,
			"valid_invitees":         summary.ValidInvitees,
			"registered_count":       summary.RegisteredCount,
			"current_rate_bps":       summary.CurrentRateBps,
			"next_tier_invitees":     summary.NextTierInvitees,
			"next_tier_rate_bps":     summary.NextTierRateBps,
			"invitee_rate_bps":       setting.InviteeFirstTopUpRateBps,
			"invitee_reward_enabled": setting.InviteeFirstTopUpEnabled,
			"inviter_reward_enabled": setting.InviterEnabled,
			"bonus_duration":         setting.BonusDuration,
			"count_redeem_as_valid":  setting.CountRedeemAsValid,
			"tiers":                  setting.SortedInviterTiers(),
			"compliance_confirmed":   operation_setting.IsPaymentComplianceConfirmed(),
			"transfer_min_quota":     int(common.QuotaPerUnit),
			"withdrawal_available":   false, // 提现功能开发中，仅展示入口
		},
	})
}

// GetReferralCharges returns one page of the caller's referral reward history.
func GetReferralCharges(c *gin.Context) {
	userId := c.GetInt("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	rows, total, err := model.ListReferralCharges(userId, (page-1)*pageSize, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":     rows,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// referralSettingPrefix is the option key prefix registered by
// operation_setting.ReferralSetting.
const referralSettingPrefix = "referral_setting."

// isReferralPayoutOption blocks a referral setting that would let the program
// pay out quota until the administrator has confirmed the payment compliance
// terms. It reports the error to the client and returns true when the update
// was rejected.
//
// Values written through the generic option endpoint are strings, so the check
// works on the raw text: anything that is not an explicit off state is treated
// as a payout. A tier list therefore cannot smuggle a positive rate past the
// gate by embedding it in JSON.
func isReferralPayoutOption(c *gin.Context, key string, value string) bool {
	if !strings.HasPrefix(key, referralSettingPrefix) {
		return false
	}
	if operation_setting.IsPaymentComplianceConfirmed() {
		return false
	}
	if isReferralPayoutOff(value) {
		return false
	}
	common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
	return true
}

// isReferralPayoutOff reports whether a referral option value leaves the
// program unable to pay anything out.
func isReferralPayoutOff(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "false", "0", "0.0", "null", "[]", "{}":
		return true
	}
	return false
}

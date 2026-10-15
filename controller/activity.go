package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// 活动码：管理端维护「码 → 目标分组」的限时授权，用户端用真余额兑换。
const activityCodeMaxDurationHours = 87600 // 10 年

// ---------------------------------------------------------------------------
// 管理端
// ---------------------------------------------------------------------------

type activityCodeRequest struct {
	Id            int     `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Status        *int    `json:"status"`
	GroupName     string  `json:"group_name"`
	DurationHours float64 `json:"duration_hours"`
	RpmLimit      int     `json:"rpm_limit"`
	PriceQuota    int     `json:"price_quota"`
	RatioMode     string  `json:"ratio_mode"`
	RatioOverride float64 `json:"ratio_override"`
	StartTime     int64   `json:"start_time"`
	EndTime       int64   `json:"end_time"`
	TotalLimit    int     `json:"total_limit"`
	PerUserLimit  *int    `json:"per_user_limit"`
	PublicVisible *int    `json:"public_visible"`
}

// buildActivityCode 校验并组装活动码。请求体由调用方解析一次后传入：
// c.ShouldBindJSON 会消费请求体，函数内部再次绑定只会读到 EOF。
// existing 非空表示更新（活动码字符串不可修改）。
func buildActivityCode(c *gin.Context, req *activityCodeRequest, existing *model.ActivityCode) (*model.ActivityCode, bool) {
	activityCode := &model.ActivityCode{}
	if existing != nil {
		activityCode = existing
	} else {
		code := model.NormalizeActivityCode(req.Code)
		if !model.IsValidActivityCode(code) {
			common.ApiErrorI18n(c, i18n.MsgActivityCodeFormat)
			return nil, false
		}
		activityCode.Code = code
	}

	name := strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(name) == 0 || utf8.RuneCountInString(name) > 64 {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeNameLength)
		return nil, false
	}
	if _, ok := ratio_setting.GetGroupRatioCopy()[req.GroupName]; !ok {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeGroupNotConfigured, map[string]any{"Group": req.GroupName})
		return nil, false
	}
	if req.DurationHours < 0 || req.DurationHours > activityCodeMaxDurationHours {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeDurationInvalid)
		return nil, false
	}
	if req.RpmLimit < 0 || req.RpmLimit > model.ActivityCodeMaxRpm {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeRpmInvalid, map[string]any{"Max": model.ActivityCodeMaxRpm})
		return nil, false
	}
	if req.PriceQuota < 0 || req.PriceQuota > model.ActivityCodeMaxPriceQuota {
		common.ApiErrorI18n(c, i18n.MsgActivityCodePriceInvalid, map[string]any{"Max": model.ActivityCodeMaxPriceQuota})
		return nil, false
	}
	if req.RatioMode != model.ActivityRatioModeInherit && req.RatioMode != model.ActivityRatioModeOverride {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeRatioModeInvalid)
		return nil, false
	}
	if req.RatioMode == model.ActivityRatioModeOverride &&
		(req.RatioOverride < 0 || req.RatioOverride > dto.MaxChannelModelGroupRatio) {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeRatioInvalid, map[string]any{"Max": dto.MaxChannelModelGroupRatio})
		return nil, false
	}
	now := common.GetTimestamp()
	if req.StartTime < 0 || req.EndTime < 0 {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeWindowInvalid)
		return nil, false
	}
	if req.EndTime != 0 && (req.EndTime <= now || (req.StartTime != 0 && req.EndTime <= req.StartTime)) {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeWindowInvalid)
		return nil, false
	}
	if req.TotalLimit < 0 || (req.PerUserLimit != nil && *req.PerUserLimit < 0) {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeLimitInvalid)
		return nil, false
	}

	perUserLimit := 1
	if req.PerUserLimit != nil {
		perUserLimit = *req.PerUserLimit
	}
	publicVisible := 0
	if req.PublicVisible != nil && *req.PublicVisible != 0 {
		publicVisible = 1
	}

	activityCode.Name = name
	activityCode.Description = strings.TrimSpace(req.Description)
	activityCode.GroupName = req.GroupName
	activityCode.DurationSeconds = int64(req.DurationHours * 3600)
	activityCode.RpmLimit = req.RpmLimit
	activityCode.PriceQuota = req.PriceQuota
	activityCode.RatioMode = req.RatioMode
	activityCode.RatioOverride = req.RatioOverride
	activityCode.StartTime = req.StartTime
	activityCode.EndTime = req.EndTime
	activityCode.TotalLimit = req.TotalLimit
	activityCode.PerUserLimit = perUserLimit
	activityCode.PublicVisible = publicVisible
	if existing == nil {
		activityCode.Status = model.ActivityCodeStatusEnabled
		if req.Status != nil && *req.Status == model.ActivityCodeStatusDisabled {
			activityCode.Status = model.ActivityCodeStatusDisabled
		}
	}
	return activityCode, true
}

// GetAllActivityCodes 分页查询活动码。
func GetAllActivityCodes(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	activityCodes, total, err := model.GetActivityCodes(c.Query("keyword"), c.Query("status"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(activityCodes)
	common.ApiSuccess(c, pageInfo)
}

// GetActivityCode 读取单个活动码。
func GetActivityCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	activityCode, err := model.GetActivityCodeById(id)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeNotFound)
		return
	}
	common.ApiSuccess(c, activityCode)
}

// AddActivityCode 新建活动码。
func AddActivityCode(c *gin.Context) {
	var req activityCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.SysError("failed to bind activity code payload: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	activityCode, ok := buildActivityCode(c, &req, nil)
	if !ok {
		return
	}
	if _, err := model.GetActivityCodeByCode(activityCode.Code, true); err == nil {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeExists)
		return
	}
	activityCode.CreatedBy = c.GetInt("id")
	if err := activityCode.Insert(); err != nil {
		common.SysError("failed to insert activity code: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgCreateFailed)
		return
	}
	recordManageAudit(c, "activity.create", map[string]interface{}{
		"code":        activityCode.Code,
		"group":       activityCode.GroupName,
		"price_quota": activityCode.PriceQuota,
	})
	common.ApiSuccess(c, activityCode)
}

// UpdateActivityCode 更新活动码（活动码字符串与已兑换次数不可修改）。
func UpdateActivityCode(c *gin.Context) {
	var req activityCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.SysError("failed to bind activity code payload: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	existing, err := model.GetActivityCodeById(req.Id)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeNotFound)
		return
	}
	activityCode, ok := buildActivityCode(c, &req, existing)
	if !ok {
		return
	}
	if err := activityCode.Update(); err != nil {
		common.SysError("failed to update activity code: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgUpdateFailed)
		return
	}
	recordManageAudit(c, "activity.update", map[string]interface{}{
		"code":  activityCode.Code,
		"group": activityCode.GroupName,
	})
	common.ApiSuccess(c, activityCode)
}

// UpdateActivityCodeStatus 单独切换活动码启用状态。停用不会撤销已发放的授权。
func UpdateActivityCodeStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	var req struct {
		Status int `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := model.UpdateActivityCodeStatus(id, req.Status); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "activity.status", map[string]interface{}{"id": id, "status": req.Status})
	common.ApiSuccess(c, nil)
}

// DeleteActivityCode 软删除活动码，已发放的授权不受影响。
func DeleteActivityCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	if err := model.DeleteActivityCodeById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "activity.delete", map[string]interface{}{"id": id})
	common.ApiSuccess(c, nil)
}

// GetActivityCodeGrants 查看某个活动码的授权记录。
func GetActivityCodeGrants(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	pageInfo := common.GetPageQuery(c)
	grants, total, err := model.ListActivityGrantsByCode(id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(grants)
	common.ApiSuccess(c, pageInfo)
}

// RevokeActivityGrant 撤销一张活动授权（立即失效）。
func RevokeActivityGrant(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("grant_id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	var req struct {
		Remark string `json:"remark"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := model.RevokeActivityGrant(id, strings.TrimSpace(req.Remark)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "activity.revoke", map[string]interface{}{"grant_id": id, "remark": req.Remark})
	common.ApiSuccess(c, nil)
}

// ---------------------------------------------------------------------------
// 用户端
// ---------------------------------------------------------------------------

// GetActivityOffers 返回公开可见的活动码列表，以及站点是否启用了活动码。
func GetActivityOffers(c *gin.Context) {
	activityCodes, err := model.GetEnabledActivityCodes()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := common.GetTimestamp()
	offers := make([]gin.H, 0, len(activityCodes))
	for _, activityCode := range activityCodes {
		if activityCode.PublicVisible != 1 || activityCode.RemainingTotal() == 0 {
			continue
		}
		if activityCode.StartTime != 0 && now < activityCode.StartTime {
			continue
		}
		if activityCode.WindowClosed(now) {
			continue
		}
		if !ratio_setting.ContainsGroupRatio(activityCode.GroupName) {
			continue
		}
		offers = append(offers, gin.H{
			"code_id":          activityCode.Id,
			"code":             activityCode.Code,
			"name":             activityCode.Name,
			"description":      activityCode.Description,
			"group_name":       activityCode.GroupName,
			"group_desc":       setting.GetUsableGroupDescription(activityCode.GroupName),
			"duration_seconds": activityCode.DurationSeconds,
			"rpm_limit":        activityCode.RpmLimit,
			"ratio_mode":       activityCode.RatioMode,
			"ratio_override":   activityCode.RatioOverride,
			"price_quota":      activityCode.PriceQuota,
			"remaining_total":  activityCode.RemainingTotal(),
		})
	}
	common.ApiSuccess(c, gin.H{"enabled": len(activityCodes) > 0, "offers": offers})
}

// GetSelfActivityGrants 返回当前用户的活动授权（含已过期与已撤销）。
func GetSelfActivityGrants(c *gin.Context) {
	userId := c.GetInt("id")
	grants, err := model.ListActivityGrantsByUser(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"grants": grants,
		"active": service.GetActiveActivityViews(userId),
	})
}

// VerifyActivityCode 校验活动码并返回兑换预览（不扣费、不写库）。
// 活动码不存在时只返回通用错误，避免据此探测活动码是否存在。
func VerifyActivityCode(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	activityCode, err := model.GetActivityCodeByCode(req.Code, false)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeInvalid)
		return
	}
	common.ApiSuccess(c, buildActivityCodePreview(c, c.GetInt("id"), activityCode))
}

// RedeemActivityCode 兑换活动码。付费活动码必须带 confirm=true 与期望价格，
// 与校验结果不一致时要求前端重新确认，避免绕过确认弹窗或改价后误扣。
func RedeemActivityCode(c *gin.Context) {
	userId := c.GetInt("id")
	var req struct {
		Code               string `json:"code"`
		Confirm            bool   `json:"confirm"`
		ExpectedPriceQuota int    `json:"expected_price_quota"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	activityCode, err := model.GetActivityCodeByCode(req.Code, false)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgActivityCodeInvalid)
		return
	}
	preview := buildActivityCodePreview(c, userId, activityCode)
	if activityCode.PriceQuota > 0 && !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return
	}

	result, err := model.RedeemActivityCode(model.RedeemActivityCodeRequest{
		Code:               req.Code,
		UserId:             userId,
		Confirm:            req.Confirm,
		ExpectedPriceQuota: req.ExpectedPriceQuota,
	})
	if err != nil {
		if messageKey := activityRedeemErrorKey(err); messageKey != "" {
			if errors.Is(err, model.ErrActivityCodeNeedConfirm) || errors.Is(err, model.ErrActivityCodePriceChanged) {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": i18n.T(c, messageKey),
					"data":    preview,
				})
				return
			}
			common.ApiErrorI18n(c, messageKey)
			return
		}
		common.SysError("activity redeem failed: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgOperationFailed)
		return
	}
	common.ApiSuccess(c, result)
}

// activityRedeemErrorKey 把 model 层的兑换错误映射成 i18n 文案键。
func activityRedeemErrorKey(err error) string {
	switch {
	case errors.Is(err, model.ErrActivityCodeInvalid):
		return i18n.MsgActivityCodeInvalid
	case errors.Is(err, model.ErrActivityCodeDisabled):
		return i18n.MsgActivityCodeDisabled
	case errors.Is(err, model.ErrActivityCodeNotStarted):
		return i18n.MsgActivityCodeNotStarted
	case errors.Is(err, model.ErrActivityCodeEnded):
		return i18n.MsgActivityCodeEnded
	case errors.Is(err, model.ErrActivityCodeSoldOut):
		return i18n.MsgActivityCodeSoldOut
	case errors.Is(err, model.ErrActivityCodeUserLimitReached):
		return i18n.MsgActivityCodeUserLimit
	case errors.Is(err, model.ErrActivityCodeGroupUnavailable):
		return i18n.MsgActivityCodeGroupUnavailable
	case errors.Is(err, model.ErrActivityCodeBalanceNotEnough):
		return i18n.MsgActivityCodeInsufficientBalance
	case errors.Is(err, model.ErrActivityCodeNeedConfirm):
		return i18n.MsgActivityCodeNeedConfirm
	case errors.Is(err, model.ErrActivityCodePriceChanged):
		return i18n.MsgActivityCodePriceChanged
	}
	return ""
}

// activityBlockedReasonKey 把「当前不可兑换」的原因映射成 i18n 文案键。
func activityBlockedReasonKey(reason string) string {
	switch reason {
	case activityBlockedDisabled:
		return i18n.MsgActivityCodeDisabled
	case activityBlockedNotStarted:
		return i18n.MsgActivityCodeNotStarted
	case activityBlockedEnded:
		return i18n.MsgActivityCodeEnded
	case activityBlockedSoldOut:
		return i18n.MsgActivityCodeSoldOut
	case activityBlockedUserLimit:
		return i18n.MsgActivityCodeUserLimit
	case activityBlockedGroupUnavailable:
		return i18n.MsgActivityCodeGroupUnavailable
	case activityBlockedInsufficientBalance:
		return i18n.MsgActivityCodeInsufficientBalance
	}
	return ""
}

const (
	activityBlockedDisabled            = "disabled"
	activityBlockedNotStarted          = "not_started"
	activityBlockedEnded               = "ended"
	activityBlockedSoldOut             = "sold_out"
	activityBlockedUserLimit           = "user_limit"
	activityBlockedGroupUnavailable    = "group_unavailable"
	activityBlockedInsufficientBalance = "insufficient_balance"
)

// buildActivityCodePreview 组装用户看到的活动码预览与可兑换状态。
// 只有活动码确实存在时才会调用，因此这里的细节不会泄露活动码是否存在。
func buildActivityCodePreview(c *gin.Context, userId int, activityCode *model.ActivityCode) gin.H {
	now := common.GetTimestamp()
	balanceQuota, err := model.GetUserQuota(userId, true)
	if err != nil {
		common.SysError("failed to read user quota for activity preview: " + err.Error())
	}
	bonusQuota, err := model.GetActiveBonusRemaining(userId)
	if err != nil {
		common.SysError("failed to read bonus quota for activity preview: " + err.Error())
	}

	grants, err := model.ListActivityGrantsByUser(userId)
	if err != nil {
		common.SysError("failed to list activity grants for preview: " + err.Error())
	}
	usedByUser := 0
	hasActive := false
	var activeExpireTime int64
	for _, grant := range grants {
		if grant.CodeId != activityCode.Id {
			continue
		}
		usedByUser++
		if grant.Status != model.ActivityGrantStatusActive {
			continue
		}
		if grant.ExpireTime != 0 && grant.ExpireTime <= now {
			continue
		}
		if !hasActive {
			hasActive = true
			activeExpireTime = grant.ExpireTime
			continue
		}
		if grant.ExpireTime == 0 {
			activeExpireTime = 0
		} else if activeExpireTime != 0 && grant.ExpireTime > activeExpireTime {
			activeExpireTime = grant.ExpireTime
		}
	}

	blockedReason := ""
	switch {
	case activityCode.Status != model.ActivityCodeStatusEnabled:
		blockedReason = activityBlockedDisabled
	case activityCode.StartTime != 0 && now < activityCode.StartTime:
		blockedReason = activityBlockedNotStarted
	case activityCode.WindowClosed(now):
		blockedReason = activityBlockedEnded
	case activityCode.RemainingTotal() == 0:
		blockedReason = activityBlockedSoldOut
	case activityCode.PerUserLimit > 0 && usedByUser >= activityCode.PerUserLimit:
		blockedReason = activityBlockedUserLimit
	case !ratio_setting.ContainsGroupRatio(activityCode.GroupName):
		blockedReason = activityBlockedGroupUnavailable
	case activityCode.PriceQuota > 0 && balanceQuota < activityCode.PriceQuota:
		blockedReason = activityBlockedInsufficientBalance
	}

	message := ""
	if messageKey := activityBlockedReasonKey(blockedReason); messageKey != "" {
		message = i18n.T(c, messageKey)
	}

	return gin.H{
		"code_id":             activityCode.Id,
		"code":                activityCode.Code,
		"name":                activityCode.Name,
		"description":         activityCode.Description,
		"group_name":          activityCode.GroupName,
		"group_desc":          setting.GetUsableGroupDescription(activityCode.GroupName),
		"duration_seconds":    activityCode.DurationSeconds,
		"rpm_limit":           activityCode.RpmLimit,
		"ratio_mode":          activityCode.RatioMode,
		"ratio_override":      activityCode.RatioOverride,
		"price_quota":         activityCode.PriceQuota,
		"balance_quota":       balanceQuota,
		"bonus_quota":         bonusQuota,
		"can_afford":          activityCode.PriceQuota <= 0 || balanceQuota >= activityCode.PriceQuota,
		"need_confirm":        activityCode.PriceQuota > 0,
		"already_granted":     hasActive,
		"current_expire_time": activeExpireTime,
		"remaining_total":     activityCode.RemainingTotal(),
		"redeemable":          blockedReason == "",
		"blocked_reason":      blockedReason,
		"message":             message,
	}
}

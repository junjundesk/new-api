package controller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Bonus (赠金) — admin grant + user query
// ---------------------------------------------------------------------------

// BonusGrantRequest is the payload for granting bonus to one user or all users.
//
// ExpireInput accepts a human-friendly duration: a bare number is hours
// ("1" = 1 hour), and units may be spelled out ("1天" = 24h, "1天2小时" =
// 26h). Empty means the grant never expires.
type BonusGrantRequest struct {
	UserId      int    `json:"user_id"`      // required for single-user grant
	Amount      int64  `json:"amount"`       // bonus quota, > 0
	ExpireInput string `json:"expire_input"` // duration string; empty = never expires
	Remark      string `json:"remark"`
	AllUsers    bool   `json:"all_users"` // grant to every user
	// SendEmail controls the arrival notification. Defaults to true when the
	// client omits the field, matching the admin dialog's default-checked box.
	SendEmail *bool `json:"send_email"`
}

// shouldSendBonusEmail resolves the optional email toggle, defaulting to true.
func shouldSendBonusEmail(sendEmail *bool) bool {
	if sendEmail == nil {
		return true
	}
	return *sendEmail
}

// resolveBonusExpireTime converts the admin's duration input into an absolute
// unix expiry. Returns 0 for "never expires".
func resolveBonusExpireTime(input string) (int64, error) {
	if strings.TrimSpace(input) == "" {
		return 0, nil
	}
	d, err := model.ParseBonusDuration(input)
	if err != nil {
		return 0, err
	}
	return common.GetTimestamp() + int64(d.Seconds()), nil
}

// AdminGrantBonus grants bonus quota to a single user.
// POST /api/user/:id/bonus
func AdminGrantBonus(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	var req BonusGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount <= 0 {
		common.ApiErrorMsg(c, "赠金额度必须大于 0")
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManageTargetRole(c.GetInt("role"), user.Role) {
		common.ApiErrorMsg(c, "无权管理该用户")
		return
	}

	expireTime, err := resolveBonusExpireTime(req.ExpireInput)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	grant, err := model.GrantBonus(userId, req.Amount, expireTime, model.BonusSourceAdmin, req.Remark)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if shouldSendBonusEmail(req.SendEmail) {
		sendBonusGrantEmail(user, req.Amount, expireTime, req.Remark)
	}

	recordManageAuditFor(c, userId, "user.bonus_grant", map[string]interface{}{
		"amount":      logger.LogQuota(int(req.Amount)),
		"expire_time": expireTime,
		"remark":      req.Remark,
	})
	common.ApiSuccess(c, gin.H{
		"grant_id":    grant.Id,
		"expire_time": grant.ExpireTime,
	})
}

// AdminGrantBonusAll grants bonus quota to every (non-deleted) user.
// POST /api/user/bonus/grant_all
func AdminGrantBonusAll(c *gin.Context) {
	var req BonusGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount <= 0 {
		common.ApiErrorMsg(c, "赠金额度必须大于 0")
		return
	}
	expireTime, err := resolveBonusExpireTime(req.ExpireInput)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	users, err := model.GetAllActiveUsersForBonus()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	granted := 0
	sendEmail := shouldSendBonusEmail(req.SendEmail)
	for _, user := range users {
		if _, err := model.GrantBonus(user.Id, req.Amount, expireTime, model.BonusSourceAll, req.Remark); err != nil {
			common.SysError(fmt.Sprintf("failed to grant bonus to user %d: %s", user.Id, err.Error()))
			continue
		}
		granted++
		if sendEmail {
			sendBonusGrantEmail(&user, req.Amount, expireTime, req.Remark)
		}
	}

	recordManageAuditFor(c, 0, "user.bonus_grant_all", map[string]interface{}{
		"amount":      logger.LogQuota(int(req.Amount)),
		"expire_time": expireTime,
		"granted":     granted,
	})
	common.ApiSuccess(c, gin.H{"granted": granted, "total": len(users)})
}

// bonusSummary splits a user's active bonus into expiring and permanent
// amounts so the UI can label them separately. A single "remaining" number
// with one expiry date misrepresents a user who holds both kinds of grants.
func bonusSummary(grants []model.UserBonusGrant) gin.H {
	var expiring, permanent int64
	var nearestExpire int64
	for _, g := range grants {
		remaining := g.Remaining()
		if remaining <= 0 {
			continue
		}
		if g.ExpireTime == 0 {
			permanent += remaining
			continue
		}
		expiring += remaining
		if nearestExpire == 0 || g.ExpireTime < nearestExpire {
			nearestExpire = g.ExpireTime
		}
	}
	return gin.H{
		"bonus_remaining":           expiring + permanent,
		"bonus_expiring_remaining":  expiring,
		"bonus_permanent_remaining": permanent,
		"nearest_expire_time":       nearestExpire,
	}
}

// GetSelfBonus returns the caller's active bonus summary and grant list.
// GET /api/user/self/bonus
func GetSelfBonus(c *gin.Context) {
	userId := c.GetInt("id")
	grants, err := model.ListActiveBonusGrants(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary := bonusSummary(grants)
	summary["grants"] = grants
	common.ApiSuccess(c, summary)
}

// AdminListUserBonus lists all bonus grants for a user.
// GET /api/user/:id/bonus
func AdminListUserBonus(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	grants, err := model.ListUserBonusGrants(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	activeGrants, err := model.ListActiveBonusGrants(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary := bonusSummary(activeGrants)
	summary["grants"] = grants
	common.ApiSuccess(c, summary)
}

// sendBonusGrantEmail notifies the user their bonus arrived. Best-effort: a
// missing email or SMTP config must never fail the grant.
func sendBonusGrantEmail(user *model.User, amount int64, expireTime int64, remark string) {
	if user == nil || strings.TrimSpace(user.Email) == "" {
		return
	}
	expireText := "永久有效"
	if expireTime > 0 {
		expireText = fmt.Sprintf("%s 到期", time.Unix(expireTime, 0).Format("2006-01-02 15:04:05"))
	}
	displayName := user.DisplayName
	if strings.TrimSpace(displayName) == "" {
		displayName = user.Username
	}
	content := fmt.Sprintf(
		"<p>您好 %s，</p><p>您已收到一笔赠金：<b>%s</b>，有效期至：%s。</p>",
		displayName,
		logger.FormatQuota(int(amount)),
		expireText,
	)
	if strings.TrimSpace(remark) != "" {
		content += fmt.Sprintf("<p>备注：%s</p>", remark)
	}
	go func() {
		if err := common.SendEmail("赠金到账通知", user.Email, content); err != nil {
			common.SysError(fmt.Sprintf("failed to send bonus email to user %d: %s", user.Id, err.Error()))
		}
	}()
}

package controller

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Admin email — send a message to selected users or every user
// ---------------------------------------------------------------------------

// AdminSendEmailRequest is the payload for the admin email page.
//
// Exactly one targeting mode must be used: either AllUsers, or a non-empty
// UserIds list (the single-user UI sends a one-element list).
type AdminSendEmailRequest struct {
	UserIds  []int  `json:"user_ids"`
	AllUsers bool   `json:"all_users"`
	Subject  string `json:"subject"`
	Content  string `json:"content"`
}

// adminSendEmailRecipients collects the users a campaign targets, splitting out
// the ones that cannot receive mail so the caller can report the gap.
func adminSendEmailRecipients(req *AdminSendEmailRequest) (recipients []model.User, skipped int, err error) {
	var users []model.User
	if req.AllUsers {
		users, err = model.GetAllActiveUsersForBonus()
		if err != nil {
			return nil, 0, err
		}
	} else {
		users, err = model.GetUsersByIdsForEmail(req.UserIds)
		if err != nil {
			return nil, 0, err
		}
	}

	for _, user := range users {
		if strings.TrimSpace(user.Email) == "" {
			skipped++
			continue
		}
		recipients = append(recipients, user)
	}
	return recipients, skipped, nil
}

// AdminSendEmail sends a custom email to selected users or to every user.
// POST /api/user/email/send
func AdminSendEmail(c *gin.Context) {
	var req AdminSendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if !req.AllUsers && len(req.UserIds) == 0 {
		common.ApiErrorMsg(c, "请选择收件用户")
		return
	}
	if strings.TrimSpace(req.Subject) == "" {
		common.ApiErrorMsg(c, "邮件主题不能为空")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		common.ApiErrorMsg(c, "邮件内容不能为空")
		return
	}
	if !common.IsEmailConfigured() {
		common.ApiErrorMsg(c, "SMTP 未配置，无法发送邮件")
		return
	}

	recipients, skipped, err := adminSendEmailRecipients(&req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(recipients) == 0 {
		common.ApiErrorMsg(c, "没有可发送的收件人（用户未填写邮箱）")
		return
	}

	subject := strings.TrimSpace(req.Subject)
	content := req.Content

	// 全体用户数量不可控，同步发送会让 HTTP 请求超时；改为后台发送，
	// 结果写入系统日志。显式选中的收件人数量有限，同步发送并回报结果。
	if req.AllUsers {
		gopool.Go(func() {
			sent, failed := deliverAdminEmail(recipients, subject, content)
			common.SysLog(fmt.Sprintf("admin broadcast email queued subject=%q sent=%d failed=%d", subject, sent, failed))
		})
		recordManageAudit(c, "user.email_send_all", map[string]interface{}{
			"subject":  subject,
			"targeted": len(recipients),
			"skipped":  skipped,
			"async":    true,
		})
		common.ApiSuccess(c, gin.H{
			"queued":  len(recipients),
			"skipped": skipped,
			"async":   true,
		})
		return
	}

	sent, failed := deliverAdminEmail(recipients, subject, content)
	recordManageAudit(c, "user.email_send", map[string]interface{}{
		"subject":  subject,
		"targeted": len(recipients),
		"sent":     sent,
		"failed":   failed,
		"skipped":  skipped,
	})
	common.ApiSuccess(c, gin.H{
		"queued":  len(recipients),
		"sent":    sent,
		"failed":  failed,
		"skipped": skipped,
	})
}

// deliverAdminEmail sends one message per recipient, returning the success and
// failure counts. Individual failures never abort the batch.
func deliverAdminEmail(recipients []model.User, subject string, content string) (sent int, failed int) {
	for _, user := range recipients {
		if err := common.SendEmail(subject, user.Email, content); err != nil {
			failed++
			common.SysError(fmt.Sprintf("failed to send admin email to user %d: %s", user.Id, err.Error()))
			continue
		}
		sent++
	}
	return sent, failed
}

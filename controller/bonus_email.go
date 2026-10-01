package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// bonusEmailVars holds the values available to the bonus email template.
type bonusEmailVars struct {
	Username    string
	DisplayName string
	Amount      int64
	ExpireTime  int64
	Remark      string
}

// renderBonusEmail substitutes $placeholder tokens in the admin-configured
// bonus email template.
//
// Substitution is a single pass: values that themselves contain "$" are never
// re-scanned, so text from a remark cannot accidentally introduce placeholders.
func renderBonusEmail(template string, vars bonusEmailVars) string {
	amount := logger.FormatQuota(int(vars.Amount))

	expireTime := ""
	expireText := "永久有效"
	if vars.ExpireTime > 0 {
		stamp := time.Unix(vars.ExpireTime, 0).Format("2006-01-02 15:04:05")
		expireTime = stamp
		expireText = fmt.Sprintf("%s 到期", stamp)
	}

	remarkBlock := ""
	if remark := strings.TrimSpace(vars.Remark); remark != "" {
		remarkBlock = fmt.Sprintf("<p>备注：%s</p>", remark)
	}

	displayName := vars.DisplayName
	if strings.TrimSpace(displayName) == "" {
		displayName = vars.Username
	}

	replacer := strings.NewReplacer(
		"$username", vars.Username,
		"$display_name", displayName,
		"$amount", amount,
		"$expire_time", expireTime,
		"$expire_text", expireText,
		"$remark_block", remarkBlock,
		"$remark", strings.TrimSpace(vars.Remark),
		"$site_name", common.SystemName,
		"$base_url", siteBaseURL(),
	)
	return replacer.Replace(template)
}

// siteBaseURL returns the configured site address without a trailing slash,
// so templates can build absolute links (e.g. "$base_url/wallet").
func siteBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
}

// resolveBonusEmailTemplate returns the configured subject/content, falling
// back to the built-in defaults when the admin has not customized them.
func resolveBonusEmailTemplate() (string, string) {
	subject := strings.TrimSpace(operation_setting.GetBonusSetting().BonusEmailSubject)
	content := strings.TrimSpace(operation_setting.GetBonusSetting().BonusEmailContent)
	if subject == "" {
		subject = "赠金到账通知"
	}
	if content == "" {
		content = operation_setting.DefaultBonusEmailContent
	}
	return subject, content
}

// sendBonusGrantEmail notifies the user their bonus arrived. Best-effort: a
// missing email or SMTP config must never fail the grant.
func sendBonusGrantEmail(user *model.User, amount int64, expireTime int64, remark string) {
	if user == nil || strings.TrimSpace(user.Email) == "" {
		return
	}
	subjectTemplate, contentTemplate := resolveBonusEmailTemplate()
	vars := bonusEmailVars{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Amount:      amount,
		ExpireTime:  expireTime,
		Remark:      remark,
	}
	subject := renderBonusEmail(subjectTemplate, vars)
	content := renderBonusEmail(contentTemplate, vars)

	go func() {
		if err := common.SendEmail(subject, user.Email, content); err != nil {
			common.SysError(fmt.Sprintf("failed to send bonus email to user %d: %s", user.Id, err.Error()))
		}
	}()
}

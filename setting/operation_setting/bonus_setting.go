package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// BonusSetting controls the signup bonus (赠金) feature and its notification email.
type BonusSetting struct {
	// SignupBonusEnabled is the master switch for granting bonus on registration.
	SignupBonusEnabled bool `json:"signup_bonus_enabled"`
	// SignupBonusAmount is the bonus quota granted to each new user.
	SignupBonusAmount int64 `json:"signup_bonus_amount"`
	// SignupBonusDuration is the validity period, parsed by model.ParseBonusDuration
	// (bare number = hours, e.g. "1" = 1h, "1天" = 24h). Empty = never expires.
	SignupBonusDuration string `json:"signup_bonus_duration"`

	// BonusEmailSubject is the arrival-notification subject. Supports the same
	// placeholders as BonusEmailContent. Empty = built-in default.
	BonusEmailSubject string `json:"bonus_email_subject"`
	// BonusEmailContent is the arrival-notification body (HTML).
	//
	// Placeholders are written as $name:
	//
	//	$username      登录用户名
	//	$display_name  显示名（为空时回退到用户名）
	//	$amount        本次赠金额度，已按站点额度展示格式格式化
	//	$expire_time   到期时间，永不过期时为空字符串
	//	$expire_text   到期说明整句，如「2026-10-02 19:44:39 到期」或「永久有效」
	//	$remark        发放备注原文，可为空
	//	$remark_block  备注整段（有备注时为「<p>备注：…</p>」，无备注时为空字符串）
	//	$site_name     站点名称
	//
	// Empty = built-in default.
	BonusEmailContent string `json:"bonus_email_content"`
}

// DefaultBonusEmailContent is used when the admin leaves the template empty.
// It is exported so the settings UI can show it as the placeholder.
const DefaultBonusEmailContent = `<p>您好 $display_name，</p>
<p>您已收到一笔赠金：<b>$amount</b>。</p>
<p>有效期：$expire_text</p>
$remark_block`

var bonusSetting = BonusSetting{
	SignupBonusEnabled:  false,
	SignupBonusAmount:   0,
	SignupBonusDuration: "24",
	BonusEmailSubject:   "赠金到账通知",
	BonusEmailContent:   DefaultBonusEmailContent,
}

func init() {
	config.GlobalConfig.Register("bonus_setting", &bonusSetting)
}

func GetBonusSetting() *BonusSetting {
	return &bonusSetting
}

package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// BonusSetting controls the signup bonus (赠金) feature.
type BonusSetting struct {
	// SignupBonusEnabled is the master switch for granting bonus on registration.
	SignupBonusEnabled bool `json:"signup_bonus_enabled"`
	// SignupBonusAmount is the bonus quota granted to each new user.
	SignupBonusAmount int64 `json:"signup_bonus_amount"`
	// SignupBonusDuration is the validity period, parsed by model.ParseBonusDuration
	// (bare number = hours, e.g. "1" = 1h, "1天" = 24h). Empty = never expires.
	SignupBonusDuration string `json:"signup_bonus_duration"`
}

var bonusSetting = BonusSetting{
	SignupBonusEnabled:  false,
	SignupBonusAmount:   0,
	SignupBonusDuration: "24",
}

func init() {
	config.GlobalConfig.Register("bonus_setting", &bonusSetting)
}

func GetBonusSetting() *BonusSetting {
	return &bonusSetting
}

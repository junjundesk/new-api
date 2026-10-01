package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// EnterpriseSetting 企业身份配置
type EnterpriseSetting struct {
	Enabled                bool    `json:"enabled"`                  // 是否启用企业身份功能
	TotalRechargeThreshold float64 `json:"total_recharge_threshold"` // 累充金额门槛(单位:元/USD,与 Price 同单位)
}

// 默认配置：关闭，门槛 1000
var enterpriseSetting = EnterpriseSetting{
	Enabled:                false,
	TotalRechargeThreshold: 1000,
}

func init() {
	config.GlobalConfig.Register("enterprise_setting", &enterpriseSetting)
}

// GetEnterpriseSetting 获取企业身份配置
func GetEnterpriseSetting() *EnterpriseSetting {
	return &enterpriseSetting
}

// IsEnterpriseEnabled 是否启用企业身份功能
func IsEnterpriseEnabled() bool {
	return enterpriseSetting.Enabled
}

// GetEnterpriseTotalRechargeThreshold 获取累充金额门槛
func GetEnterpriseTotalRechargeThreshold() float64 {
	return enterpriseSetting.TotalRechargeThreshold
}

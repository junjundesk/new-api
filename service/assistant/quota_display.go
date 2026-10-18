package assistant

import (
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// QuotaDisplay is the user-facing representation of a quota amount. It never
// changes billing state; it only mirrors the site's display settings.
type QuotaDisplay struct {
	Amount       float64 `json:"display_amount"`
	Unit         string  `json:"display_unit"`
	Type         string  `json:"display_type"`
	QuotaPerUnit float64 `json:"quota_per_unit"`
}

func DisplayQuota(quota int64) QuotaDisplay {
	displayType := operation_setting.GetQuotaDisplayType()
	if common.QuotaPerUnit <= 0 || displayType == operation_setting.QuotaDisplayTypeTokens {
		return QuotaDisplay{
			Amount:       float64(quota),
			Unit:         "tokens",
			Type:         operation_setting.QuotaDisplayTypeTokens,
			QuotaPerUnit: common.QuotaPerUnit,
		}
	}
	rate := operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
	amount := float64(quota) / common.QuotaPerUnit * rate
	amount = math.Round(amount*1_000_000) / 1_000_000
	unit := operation_setting.GetCurrencySymbol()
	if unit == "" {
		unit = displayType
	}
	return QuotaDisplay{
		Amount:       amount,
		Unit:         unit,
		Type:         displayType,
		QuotaPerUnit: common.QuotaPerUnit,
	}
}

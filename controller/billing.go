package controller

import (
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// quotaToDisplayAmount 把内部 quota 换算为站点展示单位（货币金额或 tokens），
// 与前端 currency.ts 的口径保持一致：USD 除以 QuotaPerUnit；CNY / CUSTOM
// 再乘各自汇率；TOKENS 直接使用 tokens 数值。
func quotaToDisplayAmount(quota int) float64 {
	amount := float64(quota) / common.QuotaPerUnit
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		amount *= operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeCustom:
		if rate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate; rate > 0 {
			amount *= rate
		}
	case operation_setting.QuotaDisplayTypeTokens:
		// TOKENS 模式保持 tokens 数值
	}
	return amount
}

// roundDisplayAmount 按前端展示精度取整：金额 >= 1 保留 2 位小数，< 1 保留 4 位。
func roundDisplayAmount(amount float64) float64 {
	digits := 4.0
	if math.Abs(amount) >= 1 {
		digits = 2
	}
	scale := math.Pow(10, digits)
	return math.Round(amount*scale) / scale
}

// billingStatToken 返回计费面板接口应当按密钥自身额度统计的令牌。
// 全局关闭“显示令牌统计”或该密钥开启 display_wallet_quota 时返回 nil，
// 表示改按所属账户的钱包余额统计。
func billingStatToken(c *gin.Context) (*model.Token, error) {
	if !common.DisplayTokenStatEnabled {
		return nil, nil
	}
	token, err := model.GetTokenById(c.GetInt("token_id"))
	if err != nil {
		return nil, err
	}
	if token.DisplayWalletQuota {
		return nil, nil
	}
	return token, nil
}

func GetSubscription(c *gin.Context) {
	var remainQuota int
	var usedQuota int
	var expiredTime int64
	token, err := billingStatToken(c)
	if err != nil {
		c.JSON(200, gin.H{
			"error": types.OpenAIError{
				Message: err.Error(),
				Type:    "upstream_error",
			},
		})
		return
	}
	if token != nil {
		expiredTime = token.ExpiredTime
		remainQuota = token.RemainQuota
		usedQuota = token.UsedQuota
	} else {
		userId := c.GetInt("id")
		remainQuota, err = model.GetUserQuota(userId, false)
		if err == nil {
			usedQuota, err = model.GetUserUsedQuota(userId)
		}
		if err != nil {
			c.JSON(200, gin.H{
				"error": types.OpenAIError{
					Message: err.Error(),
					Type:    "upstream_error",
				},
			})
			return
		}
	}
	if expiredTime <= 0 {
		expiredTime = 0
	}
	// OpenAI 兼容接口中的 *_USD 字段按站点展示类型换算，与前端展示口径一致。
	amount := quotaToDisplayAmount(remainQuota + usedQuota)
	if token != nil && token.UnlimitedQuota {
		amount = 100000000
	}
	subscription := OpenAISubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       amount,
		HardLimitUSD:       amount,
		SystemHardLimitUSD: amount,
		AccessUntil:        expiredTime,
	}
	c.JSON(200, subscription)
	return
}

func GetUsage(c *gin.Context) {
	var quota int
	token, err := billingStatToken(c)
	if err != nil {
		c.JSON(200, gin.H{
			"error": types.OpenAIError{
				Message: err.Error(),
				Type:    "new_api_error",
			},
		})
		return
	}
	if token != nil {
		quota = token.UsedQuota
	} else {
		quota, err = model.GetUserUsedQuota(c.GetInt("id"))
		if err != nil {
			c.JSON(200, gin.H{
				"error": types.OpenAIError{
					Message: err.Error(),
					Type:    "new_api_error",
				},
			})
			return
		}
	}
	amount := quotaToDisplayAmount(quota)
	usage := OpenAIUsageResponse{
		Object:     "list",
		TotalUsage: amount * 100,
	}
	c.JSON(200, usage)
	return
}

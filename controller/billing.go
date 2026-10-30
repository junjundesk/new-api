package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

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
	quota := remainQuota + usedQuota
	amount := float64(quota)
	// OpenAI 兼容接口中的 *_USD 字段含义保持“额度单位”对应值：
	// 我们将其解释为以“站点展示类型”为准：
	// - USD: 直接除以 QuotaPerUnit
	// - CNY: 先转 USD 再乘汇率
	// - TOKENS: 直接使用 tokens 数量
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		amount = amount / common.QuotaPerUnit * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeTokens:
		// amount 保持 tokens 数值
	default:
		amount = amount / common.QuotaPerUnit
	}
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
	amount := float64(quota)
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		amount = amount / common.QuotaPerUnit * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeTokens:
		// tokens 保持原值
	default:
		amount = amount / common.QuotaPerUnit
	}
	usage := OpenAIUsageResponse{
		Object:     "list",
		TotalUsage: amount * 100,
	}
	c.JSON(200, usage)
	return
}

package service

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

const (
	BillingSourceWallet       = "wallet"
	BillingSourceSubscription = "subscription"
	BillingSourceBonus        = "bonus"
)

// PreConsumeBilling 根据用户计费偏好创建 BillingSession 并执行预扣费。
// 会话存储在 relayInfo.Billing 上，供后续 Settle / Refund 使用。
func PreConsumeBilling(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	if relayInfo != nil && relayInfo.QuotaClamp != nil {
		return types.NewErrorWithStatusCode(
			relayInfo.QuotaClamp,
			types.ErrorCodeModelPriceError,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if preConsumedQuota < 0 {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("pre-consume quota cannot be negative: %d", preConsumedQuota),
			types.ErrorCodeModelPriceError,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	session, apiErr := NewBillingSession(c, relayInfo, preConsumedQuota)
	if apiErr != nil {
		return apiErr
	}
	relayInfo.Billing = session
	return nil
}

// PrepareBillingForSelectedChannel refreshes only the routing-dependent part
// of synchronous pricing. Base prices and expression/request state stay frozen.
func PrepareBillingForSelectedChannel(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	if info.TieredBillingSnapshot != nil {
		return PrepareTieredBillingForSelectedGroup(c, info)
	}
	price := &info.PriceData
	quota, err := common.QuotaFromFloatStrict(price.PreConsumeQuotaBeforeGroup * price.GroupRatioInfo.GroupRatio)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeModelPriceError, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	price.QuotaToPreConsume = quota
	free := price.GroupRatioInfo.GroupRatio == 0 || (price.UsePrice && price.ModelPrice == 0) || (!price.UsePrice && price.ModelRatio == 0)
	price.FreeModel = info.Billing == nil && free && !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	return ReserveBilling(c, quota, info)
}

// ReserveBilling creates a session for a formerly free attempt, or raises the
// existing reservation before sending. Cheaper attempts refund at settlement.
// BillingSession retains its trusted-user and wallet-arrears contracts.
func ReserveBilling(c *gin.Context, quota int, info *relaycommon.RelayInfo) *types.NewAPIError {
	if info.QuotaClamp != nil {
		return types.NewErrorWithStatusCode(info.QuotaClamp, types.ErrorCodeModelPriceError, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if info.Billing == nil {
		if info.PriceData.FreeModel {
			return nil
		}
		return PreConsumeBilling(c, quota, info)
	}
	if err := info.Billing.Reserve(quota); err != nil {
		if apiErr, ok := err.(*types.NewAPIError); ok {
			return apiErr
		}
		return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}
	info.FinalPreConsumedQuota = info.Billing.GetPreConsumedQuota()
	return nil
}

// ---------------------------------------------------------------------------
// SettleBilling — 后结算辅助函数
// ---------------------------------------------------------------------------

// SettleBilling 执行计费结算。如果 RelayInfo 上有 BillingSession 则通过 session 结算，
// 否则回退到旧的 PostConsumeQuota 路径（兼容按次计费等场景）。
func SettleBilling(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) error {
	if relayInfo.Billing != nil {
		preConsumed := relayInfo.Billing.GetPreConsumedQuota()
		delta := actualQuota - preConsumed

		if delta > 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后补扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else if delta < 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后返还扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(-delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费与实际消耗一致，无需调整：%s（按次计费）",
				logger.FormatQuota(actualQuota),
			))
		}

		if err := relayInfo.Billing.Settle(actualQuota); err != nil {
			return err
		}

		// 发送额度通知：订阅/赠金各自使用对应来源的剩余额度，
		// 否则会用钱包余额（赠金计费时钱包分文未动）误报「额度即将用尽」。
		if actualQuota != 0 {
			switch relayInfo.BillingSource {
			case BillingSourceSubscription:
				checkAndSendSubscriptionQuotaNotify(relayInfo)
			case BillingSourceBonus:
				checkAndSendBonusQuotaNotify(relayInfo)
			default:
				checkAndSendQuotaNotify(relayInfo, actualQuota-preConsumed, preConsumed)
			}
		}
		return nil
	}

	// 回退：无 BillingSession 时使用旧路径
	quotaDelta := actualQuota - relayInfo.FinalPreConsumedQuota
	if quotaDelta != 0 {
		return PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
	}
	return nil
}

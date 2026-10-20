package service

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPreConsumeBillingSkipsActivityFree(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 123,
		PriceData: hosttypes.PriceData{
			GroupRatioInfo: hosttypes.GroupRatioInfo{
				GroupRatio: 0,
				Source:     hosttypes.GroupRatioSourceActivity,
			},
		},
	}

	require.Nil(t, PreConsumeBilling(ctx, 0, info))
	require.Nil(t, info.Billing, "activity-free requests must not create a billing session")
}

func TestPrepareBillingForSelectedChannelKeepsActivityFree(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 123,
		PriceData: hosttypes.PriceData{
			PreConsumeQuotaBeforeGroup: 1000,
			GroupRatioInfo: hosttypes.GroupRatioInfo{
				GroupRatio: 0,
				Source:     hosttypes.GroupRatioSourceActivity,
			},
		},
	}

	require.Nil(t, PrepareBillingForSelectedChannel(ctx, info))
	require.True(t, info.PriceData.FreeModel)
	require.Zero(t, info.PriceData.QuotaToPreConsume)
	require.Nil(t, info.Billing)
}

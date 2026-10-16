package service

import (
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// The consume log keeps the price a request would have had without its
// activity-code grant so the usage log can show that original amount struck
// through next to the discounted charge.
func TestOriginalTextQuotaWithoutActivityUsesPreGrantRatio(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	originalRatio := 0.5
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     relaytypes.RelayFormatOpenAI,
		OriginModelName: "gpt-4o",
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 2,
			GroupRatioInfo: hosttypes.GroupRatioInfo{
				GroupRatio:            0,
				HasSpecialRatio:       true,
				Source:                hosttypes.GroupRatioSourceActivity,
				ActivityOriginalRatio: &originalRatio,
			},
		},
		StartTime: time.Now(),
	}
	usage := &dto.Usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	require.Equal(t, 0, summary.Quota, "a fully free activity grant charges nothing")

	originalQuota, ok := originalTextQuotaWithoutActivity(ctx, relayInfo, usage, nil)
	require.True(t, ok)
	// (100 prompt * 1) + (20 completion * 2) = 140 tokens at a 0.5 group ratio.
	require.Equal(t, 70, originalQuota)
	require.Nil(t, relayInfo.QuotaClamp, "the shadow computation must not audit a clamp on the real request")

	other := map[string]interface{}{}
	appendActivityDiscountInfo(other, relayInfo.PriceData.GroupRatioInfo, summary.Quota, originalQuota)
	require.Equal(t, 70, other["activity_original_quota"])
	require.Equal(t, 0.5, other["activity_original_ratio"])
}

func TestOriginalTextQuotaWithoutActivitySkippedWithoutGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     relaytypes.RelayFormatOpenAI,
		OriginModelName: "gpt-4o",
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 2,
			GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 0.08},
		},
		StartTime: time.Now(),
	}

	_, ok := originalTextQuotaWithoutActivity(ctx, relayInfo, &dto.Usage{PromptTokens: 100, TotalTokens: 100}, nil)
	require.False(t, ok)
}

func TestAppendActivityDiscountInfoSkipsNonDiscounts(t *testing.T) {
	ratio := 0.08
	withGrant := hosttypes.GroupRatioInfo{ActivityOriginalRatio: &ratio}

	cases := []struct {
		name     string
		info     hosttypes.GroupRatioInfo
		actual   int
		original int
	}{
		{name: "no activity grant", info: hosttypes.GroupRatioInfo{}, actual: 0, original: 500},
		{name: "grant did not lower the price", info: withGrant, actual: 0, original: 0},
		{name: "grant is worse than the user's own ratio", info: withGrant, actual: 100, original: 50},
	}
	for _, tc := range cases {
		other := map[string]interface{}{}
		appendActivityDiscountInfo(other, tc.info, tc.actual, tc.original)
		require.Empty(t, other, tc.name)
	}
}

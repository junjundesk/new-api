package relay

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaykittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsResponsesEventStreamContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{name: "plain", contentType: "text/event-stream", want: true},
		{name: "mixed case with charset", contentType: "Text/Event-Stream; charset=utf-8", want: true},
		{name: "json", contentType: "application/json", want: false},
		{name: "empty", contentType: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isResponsesEventStreamContentType(tt.contentType))
		})
	}
}

func TestRecalcQuotaFromRatiosIgnoresInvalidMultipliers(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			Quota: 100,
		},
	}
	info.PriceData.AddOtherRatio("duration", 2)

	quota, ok := recalcQuotaFromRatios(info, map[string]float64{
		"duration": 3,
		"zero":     0,
		"negative": -1,
		"nan":      math.NaN(),
		"inf":      math.Inf(1),
	})

	require.True(t, ok)
	assert.Equal(t, 150, quota)
	assert.True(t, info.PriceData.HasOtherRatio("duration"))
}

func TestRecalcQuotaFromRatiosRejectsAllInvalidAdjustedRatios(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			Quota: 100,
		},
	}
	info.PriceData.AddOtherRatio("duration", 2)

	quota, ok := recalcQuotaFromRatios(info, map[string]float64{
		"zero":     0,
		"negative": -1,
		"nan":      math.NaN(),
		"inf":      math.Inf(1),
	})

	require.False(t, ok)
	assert.Equal(t, 0, quota)
	assert.True(t, info.PriceData.HasOtherRatio("duration"))
}

func newResponsesUpstreamInfo(baseURL string, relayFormat relaykittypes.RelayFormat, relayMode int, requestURLPath string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAIResponses,
			ApiType:           constant.APITypeOpenAI,
			ChannelBaseUrl:    baseURL,
			ApiKey:            "test-key",
			UpstreamModelName: "gpt-test",
		},
		RelayFormat:    relayFormat,
		RelayMode:      relayMode,
		RequestURLPath: requestURLPath,
	}
}

// 固定 responses 渠道必须把 Gemini 客户端请求转成 responses 发上游，
// 再把 responses 响应按 Gemini 格式回转给客户端。
func TestChatCompletionsViaResponsesSendsResponsesBodyForGeminiClient(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","status":"completed","model":"gpt-test","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],"usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}`))
	}))
	defer upstream.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gpt-test:generateContent", bytes.NewReader(nil))

	info := newResponsesUpstreamInfo(upstream.URL, relaykittypes.RelayFormatGemini, relayconstant.RelayModeGemini, "/v1beta/models/gpt-test:generateContent")
	chatRequest := &dto.GeneralOpenAIRequest{
		Model:    "gpt-test",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}

	usage, apiErr := chatCompletionsViaResponses(c, info, &openai.Adaptor{}, chatRequest)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, "/v1/responses", upstreamPath)
	var upstreamPayload map[string]any
	require.NoError(t, common.Unmarshal(upstreamBody, &upstreamPayload))
	assert.Contains(t, upstreamPayload, "input")
	assert.NotContains(t, upstreamPayload, "messages")
	var clientPayload map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &clientPayload))
	assert.Contains(t, clientPayload, "candidates")
}

// 上游把流式响应写在非 200 响应里时，不能把 IsStream 状态泄漏给后续重试渠道。
func TestChatCompletionsViaResponsesRestoresIsStreamOnError(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"boom","type":"server_error"}}`))
	}))
	defer upstream.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))

	info := newResponsesUpstreamInfo(upstream.URL, relaykittypes.RelayFormatOpenAI, relayconstant.RelayModeChatCompletions, "/v1/chat/completions")
	chatRequest := &dto.GeneralOpenAIRequest{
		Model:    "gpt-test",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}

	_, apiErr := chatCompletionsViaResponses(c, info, &openai.Adaptor{}, chatRequest)

	require.NotNil(t, apiErr)
	assert.False(t, info.IsStream)
	assert.Equal(t, relayconstant.RelayModeChatCompletions, info.RelayMode)
	assert.Equal(t, "/v1/chat/completions", info.RequestURLPath)
}

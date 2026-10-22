package openaichat

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newResponsesClientInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAIChat,
			ChannelBaseUrl:    "https://upstream.test",
			UpstreamModelName: "gpt-test",
		},
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
	}
}

func TestGetRequestURLRewritesResponsesModeToChatCompletions(t *testing.T) {
	info := newResponsesClientInfo()
	info.RequestURLPath = "/v1/responses"

	got, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	require.Equal(t, "https://upstream.test/v1/chat/completions", got)
}

func TestConvertOpenAIResponsesRequestProducesChatCompletionsBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := newResponsesClientInfo()
	request := dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: json.RawMessage(`"hello"`),
	}

	got, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.NoError(t, err)
	chatRequest, ok := got.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatRequest.Messages, 1)
	assert.Equal(t, "user", chatRequest.Messages[0].Role)
	assert.Equal(t, "hello", chatRequest.Messages[0].Content)
}

func TestDoResponseConvertsChatBodyToResponsesForResponsesClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := newResponsesClientInfo()
	body := `{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, apiErr)
	usageDto, ok := usage.(*dto.Usage)
	require.True(t, ok)
	assert.Equal(t, 2, usageDto.TotalTokens)
	got := recorder.Body.String()
	assert.Contains(t, got, `"object":"response"`)
	assert.Contains(t, got, "hi")
}

func TestConvertOpenAIResponsesRequestIgnoresStatefulFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := newResponsesClientInfo()
	request := dto.OpenAIResponsesRequest{
		Model:              "gpt-test",
		Input:              json.RawMessage(`"hello"`),
		PreviousResponseID: "resp_1",
		Conversation:       json.RawMessage(`"conv_1"`),
		ContextManagement:  json.RawMessage(`{"type":"auto"}`),
		Prompt:             json.RawMessage(`{"id":"pmpt_1"}`),
	}

	got, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.NoError(t, err)
	chatRequest, ok := got.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatRequest.Messages, 1)
	assert.Equal(t, "user", chatRequest.Messages[0].Role)
	assert.Equal(t, "hello", chatRequest.Messages[0].Content)
}

func TestConvertOpenAIResponsesRequestDropsBuiltInTools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := newResponsesClientInfo()
	request := dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[{"type":"function","name":"lookup","parameters":{"type":"object"}},{"type":"web_search_preview"}]`),
	}

	got, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.NoError(t, err)
	chatRequest, ok := got.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatRequest.Tools, 1)
	assert.Equal(t, "function", chatRequest.Tools[0].Type)
	assert.Equal(t, "lookup", chatRequest.Tools[0].Function.Name)
}

func TestConvertOpenAIResponsesRequestRejectsEmptyInputAfterIgnoringState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := newResponsesClientInfo()
	request := dto.OpenAIResponsesRequest{
		Model:              "gpt-test",
		PreviousResponseID: "resp_1",
	}

	_, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input")
}

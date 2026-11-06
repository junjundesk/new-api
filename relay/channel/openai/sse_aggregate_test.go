package openai

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClaudeTestRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-4o",
		},
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
		DisablePing: true,
		IsStream:    true,
	}
}

func newClaudeTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return c, recorder, newClaudeTestRelayInfo()
}

func TestAggregateChatSSEToJSONMergesChunks(t *testing.T) {
	body := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}

data: {"choices":[{"index":0,"delta":{"content":" world","reasoning_content":"think "}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_","arguments":""}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"weather","arguments":"{\"city\":"}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Paris\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`

	aggregated, err := aggregateChatSSEToJSON([]byte(body))
	require.NoError(t, err)

	var parsed dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(aggregated, &parsed))
	require.Len(t, parsed.Choices, 1)

	choice := parsed.Choices[0]
	assert.Equal(t, 0, choice.Index)
	assert.Equal(t, "Hello world", choice.Message.StringContent())
	assert.Equal(t, "think ", choice.Message.GetReasoningContent())
	assert.Equal(t, "tool_calls", choice.FinishReason)

	toolCalls := choice.Message.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_1", toolCalls[0].ID)
	assert.Equal(t, "get_weather", toolCalls[0].Function.Name)
	assert.JSONEq(t, `{"city":"Paris"}`, toolCalls[0].Function.Arguments)

	assert.Equal(t, 10, parsed.Usage.PromptTokens)
	assert.Equal(t, 5, parsed.Usage.CompletionTokens)
	assert.Equal(t, 15, parsed.Usage.TotalTokens)
}

func TestAggregateChatSSEToJSONSparseToolCallIndex(t *testing.T) {
	body := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":100000,"id":"call_big","function":{"name":"big","arguments":"{}"}}]}}]}
data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_zero","function":{"name":"zero","arguments":"{}"}}]}}]}`

	aggregated, err := aggregateChatSSEToJSON([]byte(body))
	require.NoError(t, err)

	var parsed dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(aggregated, &parsed))
	require.Len(t, parsed.Choices, 1)

	toolCalls := parsed.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 2)
	names := []string{toolCalls[0].Function.Name, toolCalls[1].Function.Name}
	assert.ElementsMatch(t, []string{"big", "zero"}, names)
}

func TestAggregateChatSSEToJSONHandlesDoneAndTrailingLine(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\r\n\r\ndata: [DONE]  \r\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}"

	aggregated, err := aggregateChatSSEToJSON([]byte(body))
	require.NoError(t, err)

	var parsed dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(aggregated, &parsed))
	require.Len(t, parsed.Choices, 1)
	assert.Equal(t, "hi", parsed.Choices[0].Message.StringContent())
	assert.Equal(t, "stop", parsed.Choices[0].FinishReason)
}

func TestAggregateChatSSEToJSONErrors(t *testing.T) {
	t.Run("no chunk", func(t *testing.T) {
		_, err := aggregateChatSSEToJSON([]byte(": keepalive\n\nevent: ping\n\ndata: [DONE]\n"))
		require.Error(t, err)
	})

	t.Run("no choices", func(t *testing.T) {
		_, err := aggregateChatSSEToJSON([]byte(`data: {"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		require.Error(t, err)
	})

	t.Run("stream error event", func(t *testing.T) {
		_, err := aggregateChatSSEToJSON([]byte(`data: {"error":{"type":"server_error","message":"boom"}}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})
}

func TestLooksLikeSSE(t *testing.T) {
	assert.True(t, looksLikeSSE([]byte("data: {\"a\":1}\n\n")))
	assert.True(t, looksLikeSSE([]byte("\n\nevent: message_start\ndata: {}\n")))
	assert.True(t, looksLikeSSE([]byte(": keepalive\n")))
	assert.False(t, looksLikeSSE([]byte(`{"choices":[{"index":0}]}`)))
	assert.False(t, looksLikeSSE(nil))
}

func TestFinalizeClaudeStreamEmitsTerminalEvents(t *testing.T) {
	c, recorder, info := newClaudeTestContext(t)

	contentChunk := `{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"}}]}`
	require.NoError(t, handleClaudeFormat(c, contentChunk, info))
	require.False(t, info.ClaudeConvertInfo.Done)

	// 模拟流结束但转换器没有发出终态（例如 finish chunk 不带 usage）。
	finalizeClaudeStream(c, info)

	body := recorder.Body.String()
	assert.Contains(t, body, "event: message_start")
	assert.Contains(t, body, "event: message_delta")
	assert.Contains(t, body, "event: message_stop")
	assert.True(t, info.ClaudeConvertInfo.Done)

	// 再次收尾必须幂等，不重复发终态。
	sizeBefore := recorder.Body.Len()
	finalizeClaudeStream(c, info)
	assert.Equal(t, sizeBefore, recorder.Body.Len())
}

func TestFinalizeClaudeStreamSkipsWithoutMessageStart(t *testing.T) {
	c, recorder, info := newClaudeTestContext(t)

	finalizeClaudeStream(c, info)

	assert.Empty(t, recorder.Body.String())
	assert.False(t, info.ClaudeConvertInfo.Done)
}

// errAfterReader 先返回一段数据，再返回错误，用于模拟上游中途断流。
type errAfterReader struct {
	data []byte
	sent bool
}

func (r *errAfterReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		n := copy(p, r.data)
		return n, nil
	}
	return 0, errors.New("connection reset by peer")
}

func TestOaiStreamHandlerWritesErrorEventOnInterruptedStream(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	c, recorder, info := newClaudeTestContext(t)

	streamData := "data: {\"id\":\"chatcmpl-1\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hi\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1,\"total_tokens\":4}}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&errAfterReader{data: []byte(streamData)}),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	_, newAPIError := OaiStreamHandler(c, info, resp)
	require.Nil(t, newAPIError)

	body := recorder.Body.String()
	assert.Contains(t, body, "event: error")
	assert.Contains(t, body, `"type":"stream_error"`)
	assert.NotContains(t, body, "event: message_stop")
}

func TestOpenaiHandlerMapsHTTP200ErrorBodyTo502(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := newClaudeTestRelayInfo()
	info.RelayFormat = types.RelayFormatOpenAI

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream exploded","type":"server_error"}}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	_, newAPIError := OpenaiHandler(c, info, resp)
	require.NotNil(t, newAPIError)
	assert.Equal(t, http.StatusBadGateway, newAPIError.StatusCode)
	assert.Contains(t, newAPIError.Error(), "upstream exploded")
}

func TestOaiStreamHandlerReturnsRetryableErrorBeforeAnyOutput(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	c, recorder, info := newClaudeTestContext(t)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&errAfterReader{data: nil}),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	_, newAPIError := OaiStreamHandler(c, info, resp)
	require.NotNil(t, newAPIError)
	assert.False(t, types.IsSkipRetryError(newAPIError))
	assert.NotEqual(t, http.StatusOK, newAPIError.StatusCode)
	// 还没有输出任何内容时不应写 SSE 错误事件，直接返回错误让 controller 重试。
	assert.Empty(t, recorder.Body.String())
}

func TestOpenaiHandlerAggregatesSSEForNonStreamingClient(t *testing.T) {
	c, recorder, info := newClaudeTestContext(t)
	info.IsStream = false

	body := "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hel\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1,\"total_tokens\":4}}"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	usage, newAPIError := OpenaiHandler(c, info, resp)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 3, usage.PromptTokens)

	out := recorder.Body.String()
	assert.Contains(t, out, `"type":"message"`)
	assert.Contains(t, out, `"text":"hello"`)
	// 聚合后不能把上游的 text/event-stream 透传给非流式客户端。
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
}

func TestUnwrapChatCompletionBodyGenericEnvelope(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	data := []byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`)
	wrapped := []byte(`{"data":` + string(data) + `}`)

	unwrapped, newAPIError := unwrapChatCompletionBody(c, wrapped)
	require.Nil(t, newAPIError)
	assert.JSONEq(t, string(data), string(unwrapped))

	// 顶层已有 choices 时不误拆。
	unwrapped, newAPIError = unwrapChatCompletionBody(c, data)
	require.Nil(t, newAPIError)
	assert.JSONEq(t, string(data), string(unwrapped))

	// success=false 仍然报错。
	_, newAPIError = unwrapChatCompletionBody(c, []byte(`{"success":false,"data":{"choices":[]}}`))
	require.NotNil(t, newAPIError)
}

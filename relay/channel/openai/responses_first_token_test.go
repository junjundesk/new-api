package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponsesStreamOutputEvent 保护首字口径的事件分类：只有携带模型生成内容
// 的事件才算首字，生命周期事件（response.created / response.in_progress 等）
// 只说明请求已被上游受理。
func TestResponsesStreamOutputEvent(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eventType string
		want      bool
	}{
		{"created", "response.created", false},
		{"in progress", "response.in_progress", false},
		{"output item added", "response.output_item.added", false},
		{"content part added", "response.content_part.added", false},
		{"completed", "response.completed", false},
		{"output text delta", "response.output_text.delta", true},
		{"refusal delta", "response.refusal.delta", true},
		{"reasoning text delta", "response.reasoning_text.delta", true},
		{"reasoning summary delta", "response.reasoning_summary_text.delta", true},
		{"function call arguments delta", "response.function_call_arguments.delta", true},
		{"audio delta", "response.audio.delta", true},
		{"audio transcript delta", "response.audio.transcript.delta", true},
		{"image partial image", "response.image_generation_call.partial_image", true},
	} {
		assert.Equal(t, tc.want, responsesStreamOutputEvent(tc.eventType), tc.name)
	}
}

// newFirstTokenRelayInfo 构造「协议首帧已按 response.created 打过点」的 RelayInfo：
// StartTime 在 1 秒前，首响应时间先落在 500ms 的受理时刻。
func newFirstTokenRelayInfo() (info *relaycommon.RelayInfo, acceptedAt time.Time) {
	info = &relaycommon.RelayInfo{
		OriginModelName: "deepseek-v4.1-flash",
		DisablePing:     true,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-v4.1-flash"},
	}
	info.StartTime = time.Now().Add(-time.Second)
	acceptedAt = info.StartTime.Add(500 * time.Millisecond)
	info.FirstResponseTime = acceptedAt
	return info, acceptedAt
}

// runResponsesFirstTokenStream 把 Responses SSE 事件喂给转发 handler，用于断言
// 首字打点；传输层模拟与其它 Responses 流测试保持一致。
func runResponsesFirstTokenStream(t *testing.T, info *relaycommon.RelayInfo, events ...string) {
	t.Helper()

	var body strings.Builder
	for _, event := range events {
		body.WriteString("data: ")
		body.WriteString(event)
		body.WriteString("\n\n")
	}
	body.WriteString("data: [DONE]\n\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "responses-first-token-test")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body.String())),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
}

// TestOaiResponsesStreamHandlerMarksFirstTokenOnOutput 保护首字时间的回归：
// 上游在受理时先下发生命周期事件，首字时间不能被这些事件提前。
func TestOaiResponsesStreamHandlerMarksFirstTokenOnOutput(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	t.Run("lifecycle events only keep accepted timestamp", func(t *testing.T) {
		info, acceptedAt := newFirstTokenRelayInfo()

		runResponsesFirstTokenStream(t, info,
			`{"type":"response.created"}`,
			`{"type":"response.in_progress"}`,
			`{"type":"response.output_item.added","output_index":0,"item":{"type":"reasoning"}}`,
			`{"type":"response.completed","response":{"status":"completed"}}`,
		)

		assert.Equal(t, acceptedAt, info.FirstResponseTime)
	})

	t.Run("first output delta re-marks first token", func(t *testing.T) {
		info, acceptedAt := newFirstTokenRelayInfo()

		runResponsesFirstTokenStream(t, info,
			`{"type":"response.created"}`,
			`{"type":"response.in_progress"}`,
			`{"type":"response.output_item.added","output_index":0,"item":{"type":"reasoning"}}`,
			`{"type":"response.reasoning_summary_text.delta","delta":"thinking"}`,
			`{"type":"response.output_text.delta","delta":"hi"}`,
		)

		assert.True(t, info.FirstResponseTime.After(acceptedAt),
			"frt=%v should be re-marked after acceptedAt=%v", info.FirstResponseTime, acceptedAt)
		assert.True(t, info.FirstResponseTime.After(info.StartTime))
	})
}

// TestOaiResponsesToChatStreamHandlerMarksFirstTokenOnOutput 覆盖反向转换链路
// （客户端 chat、上游 Responses）上的同一口径。
func TestOaiResponsesToChatStreamHandlerMarksFirstTokenOnOutput(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","created_at":1710000000}}`,
		`data: {"type":"response.in_progress","response":{"id":"resp_1","model":"gpt-test"}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	c, _, resp, info := newResponsesChatTestContext(t, body, true)
	info.StartTime = time.Now().Add(-time.Second)
	acceptedAt := info.StartTime.Add(500 * time.Millisecond)
	info.FirstResponseTime = acceptedAt

	usage, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.True(t, info.FirstResponseTime.After(acceptedAt),
		"frt=%v should be re-marked after acceptedAt=%v", info.FirstResponseTime, acceptedAt)
}

package openai

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The upstream cannot deliver the next chunk until the HTTP client has read the
// first text. This tests a real flush, not a timer or the contents of a recorder.
func TestChatFirstTextDoesNotWaitForNextEvent(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	const role = `{"choices":[{"index":0,"delta":{"role":"assistant"}}]}`
	const first = `{"id":"c1","choices":[{"index":0,"delta":{"content":"你"}}]}`
	const second = `{"id":"c1","choices":[{"index":0,"delta":{"content":"好"},"finish_reason":"stop"}]}`
	const usageEvent = `{"choices":[],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "chat"},
		Request:     &dto.GeneralOpenAIRequest{Stream: common.GetPointer(true)},
		RelayMode:   relayconstant.RelayModeChatCompletions, RelayFormat: types.RelayFormatOpenAI,
		IsStream: true, DisablePing: true, ShouldIncludeUsage: false}
	require.NoError(t, common.UnmarshalJsonStr(`{"eager_chat_text":true}`, &info.ChannelSetting))
	reader, tail := io.Pipe()
	var usage *dto.Usage
	var relayErr *types.NewAPIError
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		body := io.MultiReader(strings.NewReader("data: "+role+"\n\ndata: "+first+"\n\n"), reader)
		usage, relayErr = OaiStreamHandler(c, info, &http.Response{Body: io.NopCloser(body)})
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { _ = tail.Close(); _ = reader.Close() })
	// Deadline only guards against a deadlock if the one-frame delay regresses.
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	stream := bufio.NewReader(resp.Body)
	for _, frame := range []string{role, first} {
		line, err := stream.ReadString('\n')
		require.NoError(t, err, "client must receive the first text before the upstream is released")
		require.Equal(t, "data: "+frame+"\n", line)
		line, err = stream.ReadString('\n')
		require.NoError(t, err)
		require.Equal(t, "\n", line)
	}
	_, err = io.WriteString(tail, "data: "+second+"\n\ndata: "+usageEvent+"\n\ndata: [DONE]\n\n")
	require.NoError(t, err)
	require.NoError(t, tail.Close())
	rest, err := io.ReadAll(stream)
	require.NoError(t, err)
	assert.Equal(t, "data: "+second+"\n\ndata: [DONE]\n\n", string(rest))
	server.Close()
	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	assert.Equal(t, 6, usage.TotalTokens, "hiding the usage frame must not hide upstream billing")
}

func TestEagerChatPreservesWireAndBilling(t *testing.T) {
	old := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = old })
	const content = `{"id":"x","model":"chat","choices":[{"index":0,"delta":{"content":"hello"}}]}`
	const mixed = `{"choices":[{"delta":{"content":"world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`
	const thinking = `{"choices":[{"delta":{"reasoning_content":"why"}}]}`
	const tool = `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"lookup","arguments":"{}"}}]}}]}`
	const numbers = `{"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`
	for _, tc := range []struct {
		name, model               string
		frames                    []string
		includeUsage, interrupted bool
	}{
		{"only text with estimated usage", "chat", []string{content}, true, false},
		{"mixed text and usage even when usage-only frames are hidden", "chat", []string{content, mixed}, false, false},
		{"reasoning and tool calls plus explicit usage", "chat", []string{thinking, content, tool, numbers}, true, false},
		{"audio usage in penultimate event", "chat-audio", []string{content, numbers, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`}, false, false},
		{"interrupted after first text", "chat", []string{content}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var legacyWire string
			var legacyUsage *dto.Usage
			for _, eager := range []bool{false, true} {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
				info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: tc.model, ChannelSetting: dto.ChannelSettings{EagerChatText: eager}},
					RelayMode: relayconstant.RelayModeChatCompletions, RelayFormat: types.RelayFormatOpenAI,
					IsStream: true, DisablePing: true, ShouldIncludeUsage: tc.includeUsage}
				info.SetEstimatePromptTokens(3)
				data := "data: " + strings.Join(tc.frames, "\n\ndata: ") + "\n\n"
				var body io.Reader = strings.NewReader(data + "data: [DONE]\n\n")
				if tc.interrupted {
					body = &errAfterReader{data: []byte(data)}
				}
				usage, apiErr := OaiStreamHandler(c, info, &http.Response{Body: io.NopCloser(body)})
				require.Nil(t, apiErr)
				require.NotNil(t, usage)
				wire := w.Body.String()
				assert.Equal(t, 1, strings.Count(wire, content), "plain text must be delivered exactly once")
				assert.Equal(t, !tc.interrupted, strings.HasSuffix(wire, "data: [DONE]\n\n"))
				if !eager {
					legacyWire, legacyUsage = wire, usage
					continue
				}
				assert.Equal(t, legacyWire, wire)
				assert.Equal(t, legacyUsage, usage)
			}
		})
	}
}

package assistant

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateChatCompletionStreamsTextAndToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"get_usage_summary\",\"arguments\":\"{\\\"days\\\":\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"7}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewHTTPClient(Config{Protocol: "chat", BaseURL: server.URL + "/v1", APIKey: "sk-test", Model: "gpt-test", MaxOutputTokens: 100})
	deltas := make([]string, 0)
	result, err := client.CreateChatCompletion(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil, func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Hel", "lo"}, deltas)
	assert.Equal(t, "Hello", result.Content)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "call_1", result.ToolCalls[0].ID)
	assert.Equal(t, "get_usage_summary", result.ToolCalls[0].Name)
	assert.Equal(t, "{\"days\":7}", result.ToolCalls[0].Arguments)
	assert.Equal(t, "tool_calls", result.FinishReason)
}

func TestChatProtocolTestConnectionUsesChatCompletions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"OK\"},\"finish_reason\":\"stop\"}]}")
	}))
	defer server.Close()

	text, _, err := TestConnection(context.Background(), Config{Protocol: "chat", BaseURL: server.URL + "/v1", APIKey: "sk-test", Model: "gpt-test", MaxOutputTokens: 100})
	require.NoError(t, err)
	assert.Equal(t, "OK", text)
}

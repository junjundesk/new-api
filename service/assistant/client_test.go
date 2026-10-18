package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testInput(t *testing.T) []json.RawMessage {
	t.Helper()
	item, err := common.Marshal(map[string]string{"role": "user", "content": "hi"})
	require.NoError(t, err)
	return []json.RawMessage{item}
}

func TestCreateResponseSendsResponsesRequestAndParsesStream(t *testing.T) {
	received := map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/responses", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &received))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"get_account_summary\",\"arguments\":\"{}\"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\"}]},{\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"get_account_summary\",\"arguments\":\"{}\"}],\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"total_tokens\":5}}}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, APIKey: "sk-test", Model: "gpt-test", MaxOutputTokens: 100}
	client := NewHTTPClient(cfg)
	var deltas []string
	tools := []map[string]any{{"type": "function", "name": "get_account_summary", "parameters": map[string]any{"type": "object"}}}
	result, err := client.CreateResponse(context.Background(), testInput(t), tools, func(text string) error {
		deltas = append(deltas, text)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Hello"}, deltas)
	require.NotNil(t, result)
	assert.Equal(t, "resp_1", result.ID)
	require.NotNil(t, result.Usage)
	assert.Equal(t, 5, result.Usage.TotalTokens)

	calls := extractFunctionCalls(result.Output)
	require.Len(t, calls, 1)
	assert.Equal(t, "get_account_summary", calls[0].Name)
	assert.Equal(t, "call_1", calls[0].CallID)
	assert.Equal(t, "{}", calls[0].Arguments)

	assert.Equal(t, false, received["store"])
	assert.Equal(t, true, received["stream"])
	requestTools, ok := received["tools"].([]any)
	require.True(t, ok)
	require.Len(t, requestTools, 1)
	firstTool, ok := requestTools[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "get_account_summary", firstTool["name"])
}

func TestCreateResponseParsesNonStreamJSONFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"id\":\"resp_json\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"JSON answer\"}]}]}")
	}))
	defer server.Close()

	client := NewHTTPClient(Config{BaseURL: server.URL, APIKey: "sk-test", Model: "gpt-test", MaxOutputTokens: 100})
	var deltas []string
	result, err := client.CreateResponse(context.Background(), testInput(t), nil, func(text string) error {
		deltas = append(deltas, text)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"JSON answer"}, deltas)
	require.NotNil(t, result)
	assert.Equal(t, "resp_json", result.ID)
}

func TestCreateResponseRedactsAPIKeyFromUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "{\"error\":{\"message\":\"bad key sk-test-secret\"}}")
	}))
	defer server.Close()

	client := NewHTTPClient(Config{BaseURL: server.URL, APIKey: "sk-test-secret", Model: "gpt-test", MaxOutputTokens: 100})
	_, err := client.CreateResponse(context.Background(), testInput(t), nil, nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "sk-test-secret")
	assert.Contains(t, err.Error(), "[REDACTED]")
}

func TestCreateResponseHonorsContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	client := NewHTTPClient(Config{BaseURL: server.URL, APIKey: "sk-test", Model: "gpt-test", MaxOutputTokens: 100})
	_, err := client.CreateResponse(ctx, testInput(t), nil, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout"))
}

func TestTestConnectionReturnsText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"id\":\"resp_test\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"OK\"}]}]}")
	}))
	defer server.Close()

	text, latency, err := TestConnection(context.Background(), Config{BaseURL: server.URL, APIKey: "sk-test", Model: "gpt-test", MaxOutputTokens: 16})
	require.NoError(t, err)
	assert.Equal(t, "OK", text)
	assert.GreaterOrEqual(t, latency, time.Duration(0))
}

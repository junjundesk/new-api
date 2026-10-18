package assistant

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResponsesClient struct {
	responses   []*ResponsesResult
	deltas      [][]string
	chatResults []*ChatResult
	chatDeltas  [][]string
	calls       int
	chatCalls   int
}

func (f *fakeResponsesClient) CreateResponse(ctx context.Context, input []json.RawMessage, tools []map[string]any, onDelta func(string) error) (*ResponsesResult, error) {
	index := f.calls
	f.calls++
	if index >= len(f.responses) {
		return &ResponsesResult{Status: "completed"}, nil
	}
	if index < len(f.deltas) && onDelta != nil {
		for _, delta := range f.deltas[index] {
			if err := onDelta(delta); err != nil {
				return nil, err
			}
		}
	}
	return f.responses[index], nil
}

func (f *fakeResponsesClient) CreateChatCompletion(ctx context.Context, messages []map[string]any, tools []map[string]any, onDelta func(string) error) (*ChatResult, error) {
	index := f.chatCalls
	f.chatCalls++
	if index >= len(f.chatResults) {
		return &ChatResult{FinishReason: "stop"}, nil
	}
	if index < len(f.chatDeltas) && onDelta != nil {
		for _, delta := range f.chatDeltas[index] {
			if err := onDelta(delta); err != nil {
				return nil, err
			}
		}
	}
	return f.chatResults[index], nil
}

type fakeToolExecutor struct {
	definitions []map[string]any
	executed    []string
	result      ToolResult
}

func (f *fakeToolExecutor) Definitions() []map[string]any {
	return f.definitions
}

func (f *fakeToolExecutor) Execute(ctx context.Context, userID int, call ToolCall, audit AuditFunc) ToolResult {
	f.executed = append(f.executed, call.Name)
	return f.result
}

func (f *fakeToolExecutor) DisplayToolName(name string) string {
	if name == "admin_db_query" {
		return "unsupported_tool"
	}
	return name
}

func rawToolCall(t *testing.T, callID, name string, arguments map[string]any) json.RawMessage {
	t.Helper()
	rawArguments, err := common.Marshal(arguments)
	require.NoError(t, err)
	item, err := common.Marshal(map[string]any{
		"type":      "function_call",
		"call_id":   callID,
		"name":      name,
		"arguments": string(rawArguments),
	})
	require.NoError(t, err)
	return item
}

func rawMessage(t *testing.T, text string) json.RawMessage {
	t.Helper()
	item, err := common.Marshal(map[string]any{
		"type": "message",
		"role": "assistant",
		"content": []map[string]any{
			{"type": "output_text", "text": text},
		},
	})
	require.NoError(t, err)
	return item
}

func testConfig() Config {
	return Config{Model: "gpt-test", Protocol: "responses", ContextWindow: 1000000, MaxOutputTokens: 128000}
}

func TestOrchestratorRunsToolThenStreamsFinalText(t *testing.T) {
	toolItem := rawToolCall(t, "call_1", "get_usage_summary", map[string]any{"days": 7})
	client := &fakeResponsesClient{
		responses: []*ResponsesResult{
			{Status: "completed", Output: []json.RawMessage{toolItem}},
			{Status: "completed", Output: []json.RawMessage{rawMessage(t, "final")}},
		},
		deltas: [][]string{{}, {"final"}},
	}
	executor := &fakeToolExecutor{
		definitions: []map[string]any{{"type": "function", "name": "get_usage_summary"}},
		result:      ToolResult{OK: true, Data: map[string]any{"quota": 1}},
	}
	events := make([]Event, 0)
	orchestrator := NewOrchestrator(testConfig(), client, executor, func(event Event) error {
		events = append(events, event)
		return nil
	}, nil)
	_, err := orchestrator.Run(context.Background(), 1, []ChatMessage{{Role: "user", Content: "hi"}}, "show usage")
	require.NoError(t, err)
	require.Len(t, executor.executed, 1)
	assert.Equal(t, "get_usage_summary", executor.executed[0])

	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	assert.Equal(t, []string{"meta", "tool_start", "tool_result", "delta"}, types)
	assert.Equal(t, "call_1", events[1].ToolCallID)
	assert.Equal(t, "get_usage_summary", events[1].Name)
	assert.NotNil(t, events[2].OK)
	assert.True(t, *events[2].OK)
	assert.Equal(t, "final", events[3].Text)
}

func TestOrchestratorHidesToolsOutsideProfileSet(t *testing.T) {
	toolItem := rawToolCall(t, "call_1", "admin_db_query", map[string]any{"table": "users"})
	client := &fakeResponsesClient{responses: []*ResponsesResult{
		{Status: "completed", Output: []json.RawMessage{toolItem}},
		{Status: "completed", Output: []json.RawMessage{rawMessage(t, "done")}},
	}}
	executor := &fakeToolExecutor{
		definitions: []map[string]any{{"type": "function", "name": "list_operations"}},
		result:      ToolResult{OK: false, Error: "tool is not available for this assistant profile"},
	}
	events := make([]Event, 0)
	orchestrator := NewOrchestrator(testConfig(), client, executor, func(event Event) error {
		events = append(events, event)
		return nil
	}, nil)
	_, runErr := orchestrator.Run(context.Background(), 1, nil, "hi")
	require.NoError(t, runErr)

	var start *Event
	for index := range events {
		assert.NotEqual(t, "admin_db_query", events[index].Name)
		if events[index].Type == "tool_start" {
			start = &events[index]
		}
	}
	require.NotNil(t, start)
	assert.Equal(t, "unsupported_tool", start.Name)
	assert.Nil(t, start.Arguments)
}

func TestOrchestratorRunsMultipleToolRoundsWithoutCap(t *testing.T) {
	toolItem := rawToolCall(t, "call_1", "get_account_summary", map[string]any{})
	client := &fakeResponsesClient{responses: []*ResponsesResult{
		{Status: "completed", Output: []json.RawMessage{toolItem}},
		{Status: "completed", Output: []json.RawMessage{toolItem}},
		{Status: "completed", Output: []json.RawMessage{rawMessage(t, "done")}},
	}}
	executor := &fakeToolExecutor{definitions: []map[string]any{{"type": "function", "name": "get_account_summary"}}, result: ToolResult{OK: true}}
	orchestrator := NewOrchestrator(testConfig(), client, executor, func(event Event) error { return nil }, nil)
	_, runErr := orchestrator.Run(context.Background(), 1, nil, "hi")
	require.NoError(t, runErr)
	assert.Equal(t, []string{"get_account_summary", "get_account_summary"}, executor.executed)
}

func TestOrchestratorChatProtocolLoopsTools(t *testing.T) {
	client := &fakeResponsesClient{
		chatResults: []*ChatResult{
			{FinishReason: "tool_calls", ToolCalls: []ChatToolCall{{ID: "call_1", Name: "get_usage_summary", Arguments: "{}"}}},
			{FinishReason: "stop", Content: "final"},
		},
		chatDeltas: [][]string{{}, {"final"}},
	}
	executor := &fakeToolExecutor{
		definitions: []map[string]any{{"type": "function", "name": "get_usage_summary"}},
		result:      ToolResult{OK: true, Data: map[string]any{"quota": 1}},
	}
	events := make([]Event, 0)
	cfg := testConfig()
	cfg.Protocol = "chat"
	orchestrator := NewOrchestrator(cfg, client, executor, func(event Event) error {
		events = append(events, event)
		return nil
	}, nil)
	_, runErr := orchestrator.Run(context.Background(), 1, nil, "show usage")
	require.NoError(t, runErr)
	assert.Equal(t, []string{"get_usage_summary"}, executor.executed)
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	assert.Equal(t, []string{"meta", "tool_start", "tool_result", "delta"}, types)
	assert.Equal(t, "final", events[3].Text)
}

func TestOrchestratorReturnsIncompleteError(t *testing.T) {
	client := &fakeResponsesClient{responses: []*ResponsesResult{
		{Status: "incomplete", IncompleteReason: "max_output_tokens", Output: []json.RawMessage{rawMessage(t, "partial")}},
	}}
	executor := &fakeToolExecutor{definitions: []map[string]any{}, result: ToolResult{OK: true}}
	orchestrator := NewOrchestrator(testConfig(), client, executor, func(event Event) error { return nil }, nil)
	_, err := orchestrator.Run(context.Background(), 1, nil, "hi")
	var incompleteError *IncompleteError
	require.ErrorAs(t, err, &incompleteError)
	assert.Equal(t, "max_output_tokens", incompleteError.Reason)
}

func TestBuildResponsesInputSkipsEmptyAndUnknownHistory(t *testing.T) {
	items, err := buildResponsesInput(testConfig(), []ChatMessage{
		{Role: "user", Content: "first"},
		{Role: "system", Content: "ignored"},
		{Role: "assistant", Content: ""},
		{Role: "assistant", Content: "second"},
	}, "third")
	require.NoError(t, err)
	require.Len(t, items, 3)
	var first map[string]string
	require.NoError(t, common.Unmarshal(items[0], &first))
	assert.Equal(t, "first", first["content"])
	var last map[string]string
	require.NoError(t, common.Unmarshal(items[2], &last))
	assert.Equal(t, "third", last["content"])
}

func TestTrimHistoryForContext(t *testing.T) {
	cfg := testConfig()
	cfg.ContextWindow = 50
	cfg.MaxOutputTokens = 20
	history := []ChatMessage{
		{Role: "user", Content: "old message that is far too long"},
		{Role: "assistant", Content: "recent"},
	}
	trimmed, err := trimHistoryForContext(cfg, history, "hi")
	require.NoError(t, err)
	require.Len(t, trimmed, 1)
	assert.Equal(t, "recent", trimmed[0].Content)

	cfg.ContextWindow = 10
	cfg.MaxOutputTokens = 8
	_, err = trimHistoryForContext(cfg, nil, "message too long")
	var contextError *ContextExceededError
	require.ErrorAs(t, err, &contextError)
}

func TestBuildChatMessagesIncludesSystemPrompt(t *testing.T) {
	cfg := testConfig()
	cfg.Protocol = "chat"
	cfg.SystemPrompt = "be helpful"
	messages, err := buildChatMessages(cfg, []ChatMessage{{Role: "user", Content: "earlier"}}, "now")
	require.NoError(t, err)
	require.Len(t, messages, 3)
	assert.Equal(t, "system", messages[0]["role"])
	assert.Equal(t, "earlier", messages[1]["content"])
	assert.Equal(t, "now", messages[2]["content"])
}

func TestOrchestratorAccumulatesUsageAcrossToolRounds(t *testing.T) {
	toolItem := rawToolCall(t, "call_1", "get_usage_summary", map[string]any{})
	client := &fakeResponsesClient{responses: []*ResponsesResult{
		{Status: "completed", Output: []json.RawMessage{toolItem}, Usage: &Usage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12}},
		{Status: "completed", Output: []json.RawMessage{rawMessage(t, "done")}, Usage: &Usage{InputTokens: 20, OutputTokens: 3, TotalTokens: 23}},
	}}
	executor := &fakeToolExecutor{definitions: []map[string]any{{"type": "function", "name": "get_usage_summary"}}, result: ToolResult{OK: true}}
	orchestrator := NewOrchestrator(testConfig(), client, executor, func(event Event) error { return nil }, nil)
	usage, err := orchestrator.Run(context.Background(), 1, nil, "hi")
	require.NoError(t, err)
	assert.Equal(t, 30, usage.InputTokens)
	assert.Equal(t, 5, usage.OutputTokens)
	assert.Equal(t, 35, usage.TotalTokens)
	assert.Equal(t, 20, orchestrator.ContextTokens(), "context size is the latest request input, not the summed usage")
}

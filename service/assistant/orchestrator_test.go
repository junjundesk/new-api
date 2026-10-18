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
	responses []*ResponsesResult
	deltas    [][]string
	calls     int
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

type fakeToolExecutor struct {
	definitions []map[string]any
	executed    []string
	result      ToolResult
}

func (f *fakeToolExecutor) Definitions(allowTokenActions bool) []map[string]any {
	return f.definitions
}

func (f *fakeToolExecutor) Execute(ctx context.Context, userID int, call ToolCall, audit AuditFunc) ToolResult {
	f.executed = append(f.executed, call.Name)
	return f.result
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
	orchestrator := NewOrchestrator(Config{Model: "gpt-test", MaxToolRounds: 2}, client, executor, func(event Event) error {
		events = append(events, event)
		return nil
	}, nil)
	err := orchestrator.Run(context.Background(), 1, []ChatMessage{{Role: "user", Content: "hi"}}, "show usage")
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

func TestOrchestratorStopsAtMaxToolRounds(t *testing.T) {
	toolItem := rawToolCall(t, "call_1", "get_account_summary", map[string]any{})
	client := &fakeResponsesClient{responses: []*ResponsesResult{
		{Status: "completed", Output: []json.RawMessage{toolItem}},
		{Status: "completed", Output: []json.RawMessage{toolItem}},
	}}
	executor := &fakeToolExecutor{definitions: []map[string]any{{"type": "function", "name": "get_account_summary"}}, result: ToolResult{OK: true}}
	orchestrator := NewOrchestrator(Config{Model: "gpt-test", MaxToolRounds: 1}, client, executor, func(event Event) error { return nil }, nil)
	err := orchestrator.Run(context.Background(), 1, nil, "hi")
	var roundsError *TooManyToolRoundsError
	require.ErrorAs(t, err, &roundsError)
	assert.Equal(t, 1, roundsError.Max)
}

func TestOrchestratorReturnsIncompleteError(t *testing.T) {
	client := &fakeResponsesClient{responses: []*ResponsesResult{
		{Status: "incomplete", IncompleteReason: "max_output_tokens", Output: []json.RawMessage{rawMessage(t, "partial")}},
	}}
	executor := &fakeToolExecutor{definitions: []map[string]any{}, result: ToolResult{OK: true}}
	orchestrator := NewOrchestrator(Config{Model: "gpt-test", MaxToolRounds: 1}, client, executor, func(event Event) error { return nil }, nil)
	err := orchestrator.Run(context.Background(), 1, nil, "hi")
	var incompleteError *IncompleteError
	require.ErrorAs(t, err, &incompleteError)
	assert.Equal(t, "max_output_tokens", incompleteError.Reason)
}

func TestBuildInputSkipsEmptyAndUnknownHistory(t *testing.T) {
	items, err := buildInput([]ChatMessage{
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

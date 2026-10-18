package assistant

import (
	"context"
	"encoding/json"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const maxToolResultBytes = 32 << 10

// Orchestrator runs one assistant turn: it calls the model, executes any
// requested tools, feeds the results back, and streams text deltas through
// emit. It does not emit terminal error/done events; the controller owns that.
type Orchestrator struct {
	cfg      Config
	client   Client
	executor ToolExecutor
	emit     EmitFunc
	audit    AuditFunc
}

func NewOrchestrator(cfg Config, client Client, executor ToolExecutor, emit EmitFunc, audit AuditFunc) *Orchestrator {
	return &Orchestrator{cfg: cfg, client: client, executor: executor, emit: emit, audit: audit}
}

func (o *Orchestrator) Run(ctx context.Context, userID int, history []ChatMessage, message string) error {
	if err := o.emit(Event{Type: "meta", Model: o.cfg.Model}); err != nil {
		return err
	}
	if o.cfg.Protocol == "chat" {
		return o.runChat(ctx, userID, history, message)
	}
	return o.runResponses(ctx, userID, history, message)
}

func (o *Orchestrator) onDelta(text string) error {
	if text == "" {
		return nil
	}
	return o.emit(Event{Type: "delta", Text: text})
}

func (o *Orchestrator) runResponses(ctx context.Context, userID int, history []ChatMessage, message string) error {
	input, err := buildResponsesInput(o.cfg, history, message)
	if err != nil {
		return err
	}
	tools := o.executor.Definitions()
	for {
		result, err := o.client.CreateResponse(ctx, input, tools, o.onDelta)
		if err != nil {
			return err
		}
		calls := extractFunctionCalls(result.Output)
		if len(calls) == 0 {
			if result.Status == "incomplete" || result.IncompleteReason != "" {
				return &IncompleteError{Reason: result.IncompleteReason}
			}
			return nil
		}
		input = append(input, result.Output...)
		for _, call := range calls {
			toolResult := o.executeTool(ctx, userID, call)
			output, err := marshalToolOutput(toolResult)
			if err != nil {
				return err
			}
			item, err := common.Marshal(map[string]any{
				"type":    "function_call_output",
				"call_id": call.CallID,
				"output":  output,
			})
			if err != nil {
				return err
			}
			input = append(input, item)
		}
	}
}

func (o *Orchestrator) runChat(ctx context.Context, userID int, history []ChatMessage, message string) error {
	messages, err := buildChatMessages(o.cfg, history, message)
	if err != nil {
		return err
	}
	tools := chatToolDefinitions(o.executor.Definitions())
	for {
		result, err := o.client.CreateChatCompletion(ctx, messages, tools, o.onDelta)
		if err != nil {
			return err
		}
		if len(result.ToolCalls) == 0 {
			if result.FinishReason == "length" {
				return &IncompleteError{Reason: "max_output_tokens"}
			}
			return nil
		}
		assistantCalls := make([]map[string]any, 0, len(result.ToolCalls))
		for _, call := range result.ToolCalls {
			assistantCalls = append(assistantCalls, map[string]any{
				"id":   call.ID,
				"type": "function",
				"function": map[string]any{
					"name":      call.Name,
					"arguments": call.Arguments,
				},
			})
		}
		assistantMessage := map[string]any{
			"role":       "assistant",
			"content":    result.Content,
			"tool_calls": assistantCalls,
		}
		if result.Content == "" {
			assistantMessage["content"] = nil
		}
		messages = append(messages, assistantMessage)
		for _, call := range result.ToolCalls {
			toolCall := ToolCall{CallID: call.ID, Name: call.Name, Arguments: call.Arguments}
			toolResult := o.executeTool(ctx, userID, toolCall)
			output, err := marshalToolOutput(toolResult)
			if err != nil {
				return err
			}
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": call.ID,
				"content":      output,
			})
		}
	}
}

func (o *Orchestrator) executeTool(ctx context.Context, userID int, call ToolCall) ToolResult {
	var arguments any
	if call.Arguments != "" {
		if err := common.Unmarshal([]byte(call.Arguments), &arguments); err != nil {
			arguments = map[string]any{"raw": call.Arguments}
		}
	}
	_ = o.emit(Event{
		Type:       "tool_start",
		ToolCallID: call.CallID,
		Name:       call.Name,
		Arguments:  arguments,
	})
	toolResult := o.executor.Execute(ctx, userID, call, o.audit)
	ok := toolResult.OK
	_ = o.emit(Event{
		Type:       "tool_result",
		ToolCallID: call.CallID,
		Name:       call.Name,
		OK:         &ok,
		Result:     toolResult.Data,
		Error:      toolResult.Error,
	})
	return toolResult
}

func buildResponsesInput(cfg Config, history []ChatMessage, message string) ([]json.RawMessage, error) {
	trimmed, err := trimHistoryForContext(cfg, history, message)
	if err != nil {
		return nil, err
	}
	items := make([]json.RawMessage, 0, len(trimmed)+1)
	for _, entry := range trimmed {
		item, err := common.Marshal(map[string]string{
			"role":    entry.Role,
			"content": entry.Content,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	current, err := common.Marshal(map[string]string{
		"role":    "user",
		"content": message,
	})
	if err != nil {
		return nil, err
	}
	items = append(items, current)
	return items, nil
}

func buildChatMessages(cfg Config, history []ChatMessage, message string) ([]map[string]any, error) {
	trimmed, err := trimHistoryForContext(cfg, history, message)
	if err != nil {
		return nil, err
	}
	messages := make([]map[string]any, 0, len(trimmed)+2)
	messages = append(messages, map[string]any{
		"role":    "system",
		"content": buildInstructions(cfg.SystemPrompt, cfg.AdminScope),
	})
	for _, entry := range trimmed {
		messages = append(messages, map[string]any{
			"role":    entry.Role,
			"content": entry.Content,
		})
	}
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": message,
	})
	return messages, nil
}

// trimHistoryForContext keeps the newest history turns that fit into the model
// context window while reserving room for the maximum output. The estimate is
// intentionally conservative (one rune per token).
func trimHistoryForContext(cfg Config, history []ChatMessage, message string) ([]ChatMessage, error) {
	budget := cfg.ContextWindow - cfg.MaxOutputTokens
	if budget <= 0 {
		return nil, &ContextExceededError{ContextWindow: cfg.ContextWindow, MaxOutput: cfg.MaxOutputTokens}
	}
	used := estimateTokens(cfg.SystemPrompt) + estimateTokens(message)
	if used > budget {
		return nil, &ContextExceededError{ContextWindow: cfg.ContextWindow, MaxOutput: cfg.MaxOutputTokens}
	}
	remaining := budget - used
	kept := make([]ChatMessage, 0, len(history))
	for index := len(history) - 1; index >= 0; index-- {
		entry := history[index]
		if entry.Role != "user" && entry.Role != "assistant" {
			continue
		}
		if entry.Content == "" {
			continue
		}
		cost := estimateTokens(entry.Content)
		if cost > remaining {
			break
		}
		kept = append(kept, entry)
		remaining -= cost
	}
	for left, right := 0, len(kept)-1; left < right; left, right = left+1, right-1 {
		kept[left], kept[right] = kept[right], kept[left]
	}
	return kept, nil
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return utf8.RuneCountInString(text)
}

func chatToolDefinitions(definitions []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        definition["name"],
				"description": definition["description"],
				"parameters":  definition["parameters"],
			},
		})
	}
	return result
}

func marshalToolOutput(result ToolResult) (string, error) {
	payload, err := common.Marshal(result)
	if err != nil {
		return "", err
	}
	if len(payload) <= maxToolResultBytes {
		return string(payload), nil
	}
	truncated, err := common.Marshal(ToolResult{
		OK:        result.OK,
		Error:     result.Error,
		Truncated: true,
		Data:      map[string]any{"message": "Tool result was too large and has been truncated."},
	})
	if err != nil {
		return "", err
	}
	return string(truncated), nil
}

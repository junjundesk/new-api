package assistant

import (
	"context"
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
)

const maxToolResultBytes = 32 << 10

// Orchestrator runs one assistant turn: it calls the model, executes any
// requested tools, feeds the results back, and streams text deltas through
// emit. It does not emit terminal error/done events; the controller owns that.
type Orchestrator struct {
	cfg      Config
	client   ResponsesClient
	executor ToolExecutor
	emit     EmitFunc
	audit    AuditFunc
}

func NewOrchestrator(cfg Config, client ResponsesClient, executor ToolExecutor, emit EmitFunc, audit AuditFunc) *Orchestrator {
	return &Orchestrator{cfg: cfg, client: client, executor: executor, emit: emit, audit: audit}
}

func (o *Orchestrator) Run(ctx context.Context, userID int, history []ChatMessage, message string) error {
	input, err := buildInput(history, message)
	if err != nil {
		return err
	}
	if err := o.emit(Event{Type: "meta", Model: o.cfg.Model}); err != nil {
		return err
	}
	tools := o.executor.Definitions(o.cfg.AllowTokenActions)
	maxRounds := o.cfg.MaxToolRounds
	if maxRounds <= 0 {
		maxRounds = 5
	}
	if maxRounds > 10 {
		maxRounds = 10
	}

	for round := 0; round <= maxRounds; round++ {
		result, err := o.client.CreateResponse(ctx, input, tools, func(text string) error {
			if text == "" {
				return nil
			}
			return o.emit(Event{Type: "delta", Text: text})
		})
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
		if round == maxRounds {
			return &TooManyToolRoundsError{Max: maxRounds}
		}
		input = append(input, result.Output...)

		for _, call := range calls {
			var arguments any
			if call.Arguments != "" {
				if err := common.Unmarshal([]byte(call.Arguments), &arguments); err != nil {
					arguments = map[string]any{"raw": call.Arguments}
				}
			}
			if err := o.emit(Event{
				Type:       "tool_start",
				ToolCallID: call.CallID,
				Name:       call.Name,
				Arguments:  arguments,
			}); err != nil {
				return err
			}

			toolResult := o.executor.Execute(ctx, userID, call, o.audit)
			ok := toolResult.OK
			if err := o.emit(Event{
				Type:       "tool_result",
				ToolCallID: call.CallID,
				Name:       call.Name,
				OK:         &ok,
				Result:     toolResult.Data,
				Error:      toolResult.Error,
			}); err != nil {
				return err
			}

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
	return &TooManyToolRoundsError{Max: maxRounds}
}

func buildInput(history []ChatMessage, message string) ([]json.RawMessage, error) {
	items := make([]json.RawMessage, 0, len(history)+1)
	for _, entry := range history {
		if entry.Role != "user" && entry.Role != "assistant" {
			continue
		}
		content := entry.Content
		if content == "" {
			continue
		}
		item, err := common.Marshal(map[string]string{
			"role":    entry.Role,
			"content": content,
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

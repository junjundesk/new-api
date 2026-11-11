package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"
)

const maxToolResultBytes = 32 << 10

// maxToolRounds bounds the model/tool round trips of one turn. A normal task
// needs a handful; the bound only stops a model that keeps calling tools
// without converging. After the last round the model is asked once more
// without tools so the user still gets an answer.
const maxToolRounds = 40

// Orchestrator runs one assistant turn: it calls the model, executes any
// requested tools, feeds the results back, and streams text deltas through
// emit. It does not emit terminal error/done events; the controller owns that.
type Orchestrator struct {
	cfg      Config
	client   Client
	executor ToolExecutor
	emit     EmitFunc
	audit    AuditFunc
	usage    Usage
	// contextTokens is the input size of the most recent upstream request, not
	// the sum across tool rounds, so the UI can show the live context size.
	contextTokens int
}

func NewOrchestrator(cfg Config, client Client, executor ToolExecutor, emit EmitFunc, audit AuditFunc) *Orchestrator {
	return &Orchestrator{cfg: cfg, client: client, executor: executor, emit: emit, audit: audit}
}

// Run executes one assistant turn and returns the upstream token usage
// accumulated across every tool round.
func (o *Orchestrator) Run(ctx context.Context, userID int, history []ChatMessage, message string) (Usage, error) {
	if err := o.emit(Event{Type: "meta", Model: o.cfg.Model}); err != nil {
		return o.usage, err
	}
	var err error
	if o.cfg.Protocol == "chat" {
		err = o.runChat(ctx, userID, history, message)
	} else {
		err = o.runResponses(ctx, userID, history, message)
	}
	return o.usage, err
}

// ContextTokens returns the input token count of the latest upstream request.
// It is zero until the first request completes.
func (o *Orchestrator) ContextTokens() int {
	return o.contextTokens
}

func (o *Orchestrator) addUsage(usage *Usage) {
	if usage == nil {
		return
	}
	o.contextTokens = usage.InputTokens
	o.usage.InputTokens += usage.InputTokens
	o.usage.OutputTokens += usage.OutputTokens
	total := usage.TotalTokens
	if total <= 0 {
		total = usage.InputTokens + usage.OutputTokens
	}
	o.usage.TotalTokens += total
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
	for round := 0; ; round++ {
		if round >= maxToolRounds {
			tools = nil
		}
		result, err := o.client.CreateResponse(ctx, input, tools, o.onDelta)
		if err != nil {
			return err
		}
		o.addUsage(result.Usage)
		calls := extractFunctionCalls(result.Output)
		if len(calls) == 0 {
			if result.Status == "incomplete" || result.IncompleteReason != "" {
				return &IncompleteError{Reason: result.IncompleteReason}
			}
			return nil
		}
		if tools == nil {
			return &IncompleteError{Reason: "tool_round_limit"}
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
	for round := 0; ; round++ {
		if round >= maxToolRounds {
			tools = nil
		}
		result, err := o.client.CreateChatCompletion(ctx, messages, tools, o.onDelta)
		if err != nil {
			return err
		}
		o.addUsage(result.Usage)
		if len(result.ToolCalls) == 0 {
			if result.FinishReason == "length" {
				return &IncompleteError{Reason: "max_output_tokens"}
			}
			return nil
		}
		if tools == nil {
			return &IncompleteError{Reason: "tool_round_limit"}
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
	if presenter, ok := o.executor.(toolArgumentPresenter); ok {
		arguments = presenter.DisplayArguments(call)
	} else if call.Arguments != "" {
		if err := common.Unmarshal([]byte(call.Arguments), &arguments); err != nil {
			// Unparseable arguments may still carry a credential; never echo them.
			arguments = map[string]any{"raw": redactedPlaceholder}
		} else {
			arguments = redactOperationValue(assistantops.Operation{}, arguments)
		}
	}
	displayName := call.Name
	if resolver, ok := o.executor.(toolNameResolver); ok {
		displayName = resolver.DisplayToolName(call.Name)
	}
	if displayName != call.Name {
		arguments = nil
	}
	_ = o.emit(Event{
		Type:       "tool_start",
		ToolCallID: call.CallID,
		Name:       displayName,
		Arguments:  arguments,
	})
	toolResult := o.executor.Execute(ctx, userID, call, o.audit)
	ok := toolResult.OK
	_ = o.emit(Event{
		Type:       "tool_result",
		ToolCallID: call.CallID,
		Name:       displayName,
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
		"content": buildInstructions(cfg),
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

// marshalToolOutput renders a tool result for the model. A result that does
// not fit is not thrown away: its largest list is shortened until it fits, and
// a note says how many rows were kept and how to get the rest.
func marshalToolOutput(result ToolResult) (string, error) {
	payload, err := common.Marshal(result)
	if err != nil {
		return "", err
	}
	if len(payload) <= maxToolResultBytes {
		return string(payload), nil
	}
	if shortened, ok := shortenToolResult(result, payload); ok {
		return shortened, nil
	}
	fallback, err := common.Marshal(ToolResult{
		OK:        result.OK,
		Error:     result.Error,
		Truncated: true,
		Note:      "The result was too large to return and contains no list that could be shortened. Request fewer fields or a narrower filter.",
	})
	if err != nil {
		return "", err
	}
	return string(fallback), nil
}

// shortenToolResult keeps as many leading elements of the largest list in the
// result as fit into maxToolResultBytes.
func shortenToolResult(result ToolResult, payload []byte) (string, bool) {
	var generic map[string]any
	if err := common.Unmarshal(payload, &generic); err != nil {
		return "", false
	}
	holder, key, list := largestList(generic["data"], 0)
	if holder == nil || len(list) < 2 {
		return "", false
	}
	render := func(keep int) (string, bool) {
		holder[key] = list[:keep]
		generic["truncated"] = true
		generic["note"] = fmt.Sprintf("Result too large: only the first %d of %d rows in %q are included. Ask for a smaller page (limit/page_size), more specific filters, or fewer columns, then page through the rest.", keep, len(list), key)
		out, err := common.Marshal(generic)
		if err != nil || len(out) > maxToolResultBytes {
			return "", false
		}
		return string(out), true
	}
	// Binary search the largest prefix that fits.
	low, high := 1, len(list)-1
	best, found := "", false
	for low <= high {
		middle := (low + high) / 2
		if out, ok := render(middle); ok {
			best, found = out, true
			low = middle + 1
		} else {
			high = middle - 1
		}
	}
	return best, found
}

// largestList finds the list with the most elements inside a decoded JSON
// value and returns the object that holds it together with its key.
func largestList(value any, depth int) (map[string]any, string, []any) {
	object, ok := value.(map[string]any)
	if !ok || depth > 4 {
		return nil, "", nil
	}
	var holder map[string]any
	var key string
	var list []any
	for name, item := range object {
		candidateHolder, candidateKey, candidate := object, name, []any(nil)
		if items, isList := item.([]any); isList {
			candidate = items
		} else {
			candidateHolder, candidateKey, candidate = largestList(item, depth+1)
		}
		if len(candidate) > len(list) {
			holder, key, list = candidateHolder, candidateKey, candidate
		}
	}
	return holder, key, list
}

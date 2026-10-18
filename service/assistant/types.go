package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Config is a request-scoped snapshot of one assistant profile.
type Config struct {
	Protocol        string
	BaseURL         string
	APIKey          string
	Model           string
	SystemPrompt    string
	ContextWindow   int
	MaxOutputTokens int
	RequestTimeout  time.Duration
}

// ChatMessage is one user/assistant text turn from the browser history.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage is the subset of the upstream usage object we display/log.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ToolCall is one upstream function/tool call.
type ToolCall struct {
	CallID    string
	Name      string
	Arguments string
	Item      json.RawMessage
}

// ResponsesResult is the normalized result of one upstream Responses call.
type ResponsesResult struct {
	ID               string
	Status           string
	Output           []json.RawMessage
	Usage            *Usage
	IncompleteReason string
}

// ChatToolCall is one tool call returned by a Chat Completions response.
type ChatToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// ChatResult is the normalized result of one upstream Chat Completions call.
type ChatResult struct {
	Content      string
	ToolCalls    []ChatToolCall
	FinishReason string
	Usage        *Usage
}

// Client is the upstream abstraction. HTTPClient implements both protocols;
// tests use fakes.
type Client interface {
	CreateResponse(ctx context.Context, input []json.RawMessage, tools []map[string]any, onDelta func(string) error) (*ResponsesResult, error)
	CreateChatCompletion(ctx context.Context, messages []map[string]any, tools []map[string]any, onDelta func(string) error) (*ChatResult, error)
}

// ToolResult is both the model-facing function output and the UI tool result.
type ToolResult struct {
	OK        bool   `json:"ok"`
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Event is one SSE event sent to the browser.
type Event struct {
	Type       string `json:"type"`
	Model      string `json:"model,omitempty"`
	Text       string `json:"text,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Arguments  any    `json:"arguments,omitempty"`
	OK         *bool  `json:"ok,omitempty"`
	Result     any    `json:"result,omitempty"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
	Code       string `json:"code,omitempty"`
	RetryAfter int64  `json:"retry_after,omitempty"`
	Usage      *Usage `json:"usage,omitempty"`
}

// EmitFunc writes one SSE event. Returning an error cancels the run.
type EmitFunc func(Event) error

// AuditFunc records a user security audit event.
type AuditFunc func(action string, params map[string]any)

// ToolExecutor defines the tools exposed to the model. All tools are available
// to every authenticated user and are scoped to that user's own data.
type ToolExecutor interface {
	Definitions() []map[string]any
	Execute(ctx context.Context, userID int, call ToolCall, audit AuditFunc) ToolResult
}

// UpstreamError describes a non-2xx or failed upstream call. Message is
// already redacted and safe to log; it is not necessarily safe to show users.
type UpstreamError struct {
	StatusCode int
	RequestID  string
	Message    string
}

func (e *UpstreamError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("assistant upstream error (status=%d request_id=%s): %s", e.StatusCode, e.RequestID, e.Message)
	}
	return fmt.Sprintf("assistant upstream error (request_id=%s): %s", e.RequestID, e.Message)
}

// BusyError is returned when the per-user or global concurrency slot is full.
type BusyError struct{}

func (e *BusyError) Error() string { return "assistant is busy" }

// IncompleteError reports an upstream response that stopped early (for
// example max_output_tokens or content_filter).
type IncompleteError struct {
	Reason string
}

func (e *IncompleteError) Error() string {
	return fmt.Sprintf("assistant response incomplete (%s)", e.Reason)
}

// ContextExceededError reports that the current message plus output budget
// cannot fit into the configured model context window.
type ContextExceededError struct {
	ContextWindow int
	MaxOutput     int
}

func (e *ContextExceededError) Error() string {
	return fmt.Sprintf("assistant context exceeded (context=%d max_output=%d)", e.ContextWindow, e.MaxOutput)
}

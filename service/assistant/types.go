package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Config is a request-scoped snapshot of one assistant profile.
type Config struct {
	AdminScope      bool
	Profile         string
	ActorID         int
	ActorUsername   string
	ActorRole       int
	ActorSessionID  string
	AuthVersion     int64
	SessionVersion  int64
	ActorIP         string
	ActorUserAgent  string
	ConfirmWrites   bool
	AdminDBTools    bool
	Protocol        string
	BaseURL         string
	APIKey          string
	Model           string
	SystemPrompt    string
	ContextWindow   int
	MaxOutputTokens int
	ReasoningEffort string
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
	Type           string `json:"type"`
	Model          string `json:"model,omitempty"`
	Text           string `json:"text,omitempty"`
	ToolCallID     string `json:"tool_call_id,omitempty"`
	Name           string `json:"name,omitempty"`
	Arguments      any    `json:"arguments,omitempty"`
	OK             *bool  `json:"ok,omitempty"`
	Result         any    `json:"result,omitempty"`
	Message        string `json:"message,omitempty"`
	Error          string `json:"error,omitempty"`
	Code           string `json:"code,omitempty"`
	RetryAfter     int64  `json:"retry_after,omitempty"`
	Usage          *Usage `json:"usage,omitempty"`
	ContextTokens  int    `json:"context_tokens,omitempty"`
	ContextWindow  int    `json:"context_window,omitempty"`
	ConfirmationID string `json:"confirmation_id,omitempty"`
	Operation      string `json:"operation_id,omitempty"`
	Risk           string `json:"risk,omitempty"`
	Summary        string `json:"summary,omitempty"`
	Params         any    `json:"params,omitempty"`
	ExpiresAt      int64  `json:"expires_at,omitempty"`
	Status         string `json:"status,omitempty"`
}

// EmitFunc writes one SSE event. Returning an error cancels the run.
type EmitFunc func(Event) error

// AuditFunc records a user security audit event.
type AuditFunc func(action string, params map[string]any)

// ToolExecutor defines the tools exposed to the model for one assistant
// profile. Each profile only receives the tools its permissions allow.
type ToolExecutor interface {
	Definitions() []map[string]any
	Execute(ctx context.Context, userID int, call ToolCall, audit AuditFunc) ToolResult
}

// toolNameResolver lets an executor collapse a tool it does not expose for its
// profile to a neutral display name in the client transcript.
type toolNameResolver interface {
	DisplayToolName(name string) string
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

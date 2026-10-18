package assistant

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
)

const (
	maxResponseBytes  = 10 << 20
	maxErrorBodyBytes = 64 << 10
)

// HTTPClient calls an OpenAI-compatible Responses endpoint.
type HTTPClient struct {
	cfg    Config
	client *http.Client
}

func NewHTTPClient(cfg Config) *HTTPClient {
	client := service.GetHttpClient()
	if client == nil {
		client = &http.Client{}
	}
	return &HTTPClient{cfg: cfg, client: client}
}

func (c *HTTPClient) endpoint() string {
	return strings.TrimRight(c.cfg.BaseURL, "/") + "/responses"
}

func (c *HTTPClient) CreateResponse(ctx context.Context, input []json.RawMessage, tools []map[string]any, onDelta func(string) error) (*ResponsesResult, error) {
	return c.createResponse(ctx, input, tools, onDelta, true)
}

func (c *HTTPClient) createResponse(ctx context.Context, input []json.RawMessage, tools []map[string]any, onDelta func(string) error, stream bool) (*ResponsesResult, error) {
	inputRaw, err := common.Marshal(input)
	if err != nil {
		return nil, err
	}
	instructionsRaw, err := common.Marshal(buildInstructions(c.cfg.SystemPrompt))
	if err != nil {
		return nil, err
	}
	var toolsRaw json.RawMessage
	if len(tools) > 0 {
		toolsRaw, err = common.Marshal(tools)
		if err != nil {
			return nil, err
		}
	}

	request := dto.OpenAIResponsesRequest{
		Model:              c.cfg.Model,
		Input:              inputRaw,
		Instructions:       instructionsRaw,
		Tools:              toolsRaw,
		PreviousResponseID: "",
		Store:              json.RawMessage("false"),
		Stream:             &stream,
		MaxOutputTokens:    uintPtr(c.cfg.MaxOutputTokens),
	}
	if stream {
		toolChoice, err := common.Marshal("auto")
		if err != nil {
			return nil, err
		}
		request.ToolChoice = toolChoice
		request.ParallelToolCalls = json.RawMessage("false")
	}
	body, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	requestID := firstHeader(resp, "x-request-id", "openai-request-id")
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, c.upstreamError(resp.StatusCode, requestID, errorBody)
	}

	reader := io.LimitReader(resp.Body, maxResponseBytes)
	contentType := resp.Header.Get("Content-Type")
	if stream && strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return c.parseStream(reader, onDelta)
	}
	responseBody, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return c.parseResponse(responseBody, onDelta)
}

func firstHeader(resp *http.Response, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(resp.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type responsesResponse struct {
	ID                string            `json:"id"`
	Status            string            `json:"status"`
	Output            []json.RawMessage `json:"output"`
	Usage             *responsesUsage   `json:"usage"`
	Error             any               `json:"error"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
}

type responsesStreamEvent struct {
	Type     string          `json:"type"`
	Delta    string          `json:"delta"`
	Item     json.RawMessage `json:"item"`
	Response json.RawMessage `json:"response"`
	Error    any             `json:"error"`
}

func (c *HTTPClient) parseResponse(body []byte, onDelta func(string) error) (*ResponsesResult, error) {
	var response responsesResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("invalid upstream response: %w", err)
	}
	if message := errorMessage(response.Error); message != "" {
		return nil, c.newUpstreamError(0, "", message)
	}
	result := resultFromResponse(&response)
	if onDelta != nil {
		if text := extractOutputText(result.Output); text != "" {
			if err := onDelta(text); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func (c *HTTPClient) parseStream(body io.Reader, onDelta func(string) error) (*ResponsesResult, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)

	var dataLines []string
	var collected []json.RawMessage
	var result *ResponsesResult
	sawDelta := false

	processEvent := func(data string) error {
		if data == "" || data == "[DONE]" {
			return nil
		}
		var event responsesStreamEvent
		if err := common.Unmarshal([]byte(data), &event); err != nil {
			// Some compatible providers emit keepalive/non-JSON data lines.
			return nil
		}
		switch event.Type {
		case "response.output_text.delta":
			if event.Delta != "" && onDelta != nil {
				sawDelta = true
				if err := onDelta(event.Delta); err != nil {
					return err
				}
			}
		case "response.output_item.done":
			if len(event.Item) > 0 {
				collected = append(collected, event.Item)
			}
		case "response.completed":
			var response responsesResponse
			if err := common.Unmarshal(event.Response, &response); err != nil {
				return fmt.Errorf("invalid completed response: %w", err)
			}
			if message := errorMessage(response.Error); message != "" {
				return c.newUpstreamError(0, "", message)
			}
			result = resultFromResponse(&response)
			result.Output = mergeFunctionCalls(result.Output, collected)
		case "response.failed":
			var response responsesResponse
			_ = common.Unmarshal(event.Response, &response)
			message := errorMessage(response.Error)
			if message == "" {
				message = "upstream response failed"
			}
			return c.newUpstreamError(0, "", message)
		case "response.incomplete":
			var response responsesResponse
			if err := common.Unmarshal(event.Response, &response); err != nil {
				return fmt.Errorf("invalid incomplete response: %w", err)
			}
			result = resultFromResponse(&response)
			result.Output = mergeFunctionCalls(result.Output, collected)
		case "error":
			message := errorMessage(event.Error)
			if message == "" {
				message = "upstream stream error"
			}
			return c.newUpstreamError(0, "", message)
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(dataLines) > 0 {
				if err := processEvent(strings.Join(dataLines, "\n")); err != nil {
					return nil, err
				}
				dataLines = dataLines[:0]
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(dataLines) > 0 {
		if err := processEvent(strings.Join(dataLines, "\n")); err != nil {
			return nil, err
		}
	}

	if result == nil {
		if len(collected) == 0 {
			return nil, errors.New("upstream stream ended without a completed response")
		}
		result = &ResponsesResult{Status: "completed", Output: collected}
	}
	if len(result.Output) == 0 {
		result.Output = collected
	}
	if !sawDelta && onDelta != nil {
		if text := extractOutputText(result.Output); text != "" {
			if err := onDelta(text); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func resultFromResponse(response *responsesResponse) *ResponsesResult {
	status := response.Status
	if status == "" {
		status = "completed"
	}
	result := &ResponsesResult{
		ID:     response.ID,
		Status: status,
		Output: response.Output,
	}
	if response.Usage != nil {
		result.Usage = &Usage{
			InputTokens:  response.Usage.InputTokens,
			OutputTokens: response.Usage.OutputTokens,
			TotalTokens:  response.Usage.TotalTokens,
		}
	}
	if response.IncompleteDetails != nil {
		result.IncompleteReason = response.IncompleteDetails.Reason
	}
	return result
}

func errorMessage(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if message, ok := typed["message"].(string); ok {
			return strings.TrimSpace(message)
		}
	}
	return ""
}

func (c *HTTPClient) newUpstreamError(statusCode int, requestID, message string) *UpstreamError {
	return &UpstreamError{
		StatusCode: statusCode,
		RequestID:  requestID,
		Message:    truncate(redact(message, c.cfg.APIKey), 500),
	}
}

func (c *HTTPClient) upstreamError(statusCode int, requestID string, body []byte) *UpstreamError {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
		Message string `json:"message"`
	}
	message := strings.TrimSpace(string(body))
	if err := common.Unmarshal(body, &payload); err == nil {
		if payload.Error.Message != "" {
			message = payload.Error.Message
		} else if payload.Message != "" {
			message = payload.Message
		}
	}
	if message == "" {
		message = http.StatusText(statusCode)
	}
	return c.newUpstreamError(statusCode, requestID, message)
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func redact(value, secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func uintPtr(value int) *uint {
	if value <= 0 {
		return nil
	}
	converted := uint(value)
	return &converted
}

type outputItem struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func mergeFunctionCalls(primary, fallback []json.RawMessage) []json.RawMessage {
	if len(fallback) == 0 {
		return primary
	}
	if len(primary) == 0 {
		return fallback
	}
	seen := make(map[string]struct{})
	for _, raw := range primary {
		var item outputFunctionCall
		if err := common.Unmarshal(raw, &item); err == nil && item.Type == "function_call" {
			seen[item.CallID] = struct{}{}
		}
	}
	merged := append([]json.RawMessage{}, primary...)
	for _, raw := range fallback {
		var item outputFunctionCall
		if err := common.Unmarshal(raw, &item); err != nil || item.Type != "function_call" {
			continue
		}
		if _, ok := seen[item.CallID]; ok {
			continue
		}
		seen[item.CallID] = struct{}{}
		merged = append(merged, raw)
	}
	return merged
}

func extractOutputText(output []json.RawMessage) string {
	var builder strings.Builder
	for _, raw := range output {
		var item outputItem
		if err := common.Unmarshal(raw, &item); err != nil {
			continue
		}
		if item.Type != "message" && item.Role != "assistant" {
			continue
		}
		for _, content := range item.Content {
			if content.Type == "output_text" || content.Type == "text" {
				builder.WriteString(content.Text)
			}
		}
	}
	return builder.String()
}

type outputFunctionCall struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func extractFunctionCalls(output []json.RawMessage) []ToolCall {
	calls := make([]ToolCall, 0)
	for index, raw := range output {
		var item outputFunctionCall
		if err := common.Unmarshal(raw, &item); err != nil {
			continue
		}
		if item.Type != "function_call" || item.Name == "" {
			continue
		}
		callID := strings.TrimSpace(item.CallID)
		if callID == "" {
			callID = strings.TrimSpace(item.ID)
		}
		if callID == "" {
			callID = fmt.Sprintf("call_%d", index)
		}
		calls = append(calls, ToolCall{
			CallID:    callID,
			Name:      item.Name,
			Arguments: argumentsString(item.Arguments),
			Item:      raw,
		})
	}
	return calls
}

func argumentsString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := common.Unmarshal(raw, &value); err == nil {
		return value
	}
	return string(raw)
}

// TestConnection performs a tiny non-streaming Responses request. It uses a
// short timeout regardless of the configured chat timeout.
func TestConnection(ctx context.Context, cfg Config) (string, time.Duration, error) {
	start := time.Now()
	cfg.MaxOutputTokens = 16
	client := NewHTTPClient(cfg)
	input := []json.RawMessage{json.RawMessage(`{"role":"user","content":"Reply with OK."}`)}
	result, err := client.createResponse(ctx, input, nil, nil, false)
	if err != nil {
		return "", time.Since(start), err
	}
	text := strings.TrimSpace(extractOutputText(result.Output))
	if text == "" {
		text = "OK"
	}
	return text, time.Since(start), nil
}

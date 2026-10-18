package assistant

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type chatCompletionRequest struct {
	Model             string           `json:"model"`
	Messages          []map[string]any `json:"messages"`
	Tools             []map[string]any `json:"tools,omitempty"`
	ToolChoice        string           `json:"tool_choice,omitempty"`
	ParallelToolCalls bool             `json:"parallel_tool_calls"`
	Stream            bool             `json:"stream"`
	MaxTokens         int              `json:"max_tokens,omitempty"`
}

type chatCompletionToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatCompletionMessage struct {
	Role      string                   `json:"role"`
	Content   string                   `json:"content"`
	ToolCalls []chatCompletionToolCall `json:"tool_calls,omitempty"`
}

type chatCompletionChoice struct {
	Message chatCompletionMessage `json:"message"`
	Delta   struct {
		Content   string                   `json:"content"`
		ToolCalls []chatCompletionToolCall `json:"tool_calls"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
	Usage   *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// CreateChatCompletion calls the OpenAI Chat Completions endpoint and streams
// text deltas through onDelta.
func (c *HTTPClient) CreateChatCompletion(ctx context.Context, messages []map[string]any, tools []map[string]any, onDelta func(string) error) (*ChatResult, error) {
	return c.createChatCompletion(ctx, messages, tools, onDelta, true)
}

func (c *HTTPClient) createChatCompletion(ctx context.Context, messages []map[string]any, tools []map[string]any, onDelta func(string) error, stream bool) (*ChatResult, error) {
	toolChoice := ""
	if len(tools) > 0 {
		toolChoice = "auto"
	}
	requestBody := chatCompletionRequest{
		Model:             c.cfg.Model,
		Messages:          messages,
		Tools:             tools,
		ToolChoice:        toolChoice,
		ParallelToolCalls: false,
		Stream:            stream,
		MaxTokens:         c.cfg.MaxOutputTokens,
	}
	body, err := common.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
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
	response, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	requestID := firstHeader(response, "x-request-id", "openai-request-id")
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
		return nil, c.upstreamError(response.StatusCode, requestID, errorBody)
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if stream && strings.Contains(contentType, "text/event-stream") {
		return c.parseChatStream(response.Body, onDelta)
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}
	return c.parseChatResponse(responseBody, onDelta)
}

func (c *HTTPClient) parseChatResponse(body []byte, onDelta func(string) error) (*ChatResult, error) {
	var response chatCompletionResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("invalid upstream response: %w", err)
	}
	if len(response.Choices) == 0 {
		return nil, errors.New("upstream chat response has no choices")
	}
	choice := response.Choices[0]
	result := &ChatResult{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		ToolCalls:    chatToolCallsFromCompletion(choice.Message.ToolCalls),
		Usage:        chatUsage(response.Usage),
	}
	if onDelta != nil && result.Content != "" {
		if err := onDelta(result.Content); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (c *HTTPClient) parseChatStream(body io.Reader, onDelta func(string) error) (*ChatResult, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	var dataLines []string
	content := strings.Builder{}
	finishReason := ""
	toolCalls := make(map[int]*ChatToolCall)
	order := make([]int, 0)
	var usage *Usage

	process := func(data string) error {
		if data == "" || data == "[DONE]" {
			return nil
		}
		var chunk chatCompletionResponse
		if err := common.Unmarshal([]byte(data), &chunk); err != nil {
			return nil
		}
		if chunk.Usage != nil {
			usage = chatUsage(chunk.Usage)
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				if onDelta != nil {
					if err := onDelta(choice.Delta.Content); err != nil {
						return err
					}
				}
			}
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
			for _, delta := range choice.Delta.ToolCalls {
				call, ok := toolCalls[delta.Index]
				if !ok {
					call = &ChatToolCall{}
					toolCalls[delta.Index] = call
					order = append(order, delta.Index)
				}
				if delta.ID != "" {
					call.ID = delta.ID
				}
				if delta.Function.Name != "" {
					call.Name = delta.Function.Name
				}
				call.Arguments += delta.Function.Arguments
			}
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(dataLines) > 0 {
				if err := process(strings.Join(dataLines, "\n")); err != nil {
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
		if err := process(strings.Join(dataLines, "\n")); err != nil {
			return nil, err
		}
	}
	sort.Ints(order)
	result := &ChatResult{
		Content:      content.String(),
		FinishReason: finishReason,
		ToolCalls:    make([]ChatToolCall, 0, len(order)),
		Usage:        usage,
	}
	for _, index := range order {
		call := toolCalls[index]
		if call == nil || call.Name == "" {
			continue
		}
		if call.ID == "" {
			call.ID = fmt.Sprintf("call_%d", index)
		}
		result.ToolCalls = append(result.ToolCalls, *call)
	}
	return result, nil
}

func chatToolCallsFromCompletion(calls []chatCompletionToolCall) []ChatToolCall {
	result := make([]ChatToolCall, 0, len(calls))
	for index, call := range calls {
		if call.Function.Name == "" {
			continue
		}
		id := call.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", index)
		}
		result = append(result, ChatToolCall{ID: id, Name: call.Function.Name, Arguments: call.Function.Arguments})
	}
	return result
}

func chatUsage(usage *struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}) *Usage {
	if usage == nil {
		return nil
	}
	return &Usage{InputTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens}
}

package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

const (
	// maxAggregatedChoices 限制聚合结果中的 choice 数量，防御异常上游构造大量稀疏下标。
	maxAggregatedChoices = 32
	// maxAggregatedToolCalls 限制单个 choice 中的工具调用数量，避免无限增长。
	maxAggregatedToolCalls = 1024
	// maxAggregatedToolCallIndex 是可信的 tool_call index 上限；超过该值说明上游下标异常，
	// 聚合时会重新分配安全下标（用 map 存储，避免按 index 分配大数组导致 OOM）。
	maxAggregatedToolCallIndex = 4096
)

// sseToolCallState 保存一条流式 tool_call 的合并状态。
type sseToolCallState struct {
	id        string
	name      string
	typ       string
	arguments strings.Builder
}

// sseChoiceState 保存单个 choice 的聚合状态。
type sseChoiceState struct {
	role         string
	content      strings.Builder
	reasoning    strings.Builder
	finishReason string
	toolCalls    map[int]*sseToolCallState
	// remappedIndex 记录异常稀疏 index 被重新分配后的安全下标。
	remappedIndex map[int]int
}

// sseChatChunk 是聚合阶段读取的 chat.completion.chunk 最小结构。
type sseChatChunk struct {
	Id      string                                    `json:"id"`
	Object  string                                    `json:"object"`
	Created int64                                     `json:"created"`
	Model   string                                    `json:"model"`
	Choices []dto.ChatCompletionsStreamResponseChoice `json:"choices"`
	Usage   *dto.Usage                                `json:"usage"`
	Error   json.RawMessage                           `json:"error"`
}

// looksLikeSSE 判断响应体是否是 SSE 文本：首个非空行以 data:/event:/id:/retry:/: 开头。
// 普通 JSON 响应体首行以 { 或 [ 开头，不会命中。
func looksLikeSSE(body []byte) bool {
	for _, rawLine := range bytes.Split(body, []byte("\n")) {
		line := strings.TrimSpace(strings.TrimRight(string(rawLine), "\r"))
		if line == "" {
			continue
		}
		return strings.HasPrefix(line, "data:") ||
			strings.HasPrefix(line, "event:") ||
			strings.HasPrefix(line, "id:") ||
			strings.HasPrefix(line, "retry:") ||
			strings.HasPrefix(line, ":")
	}
	return false
}

// aggregateChatSSEToJSON 把上游返回的 SSE 流（chat.completion.chunk 序列）聚合成
// 单个标准 chat.completion JSON。用于客户端请求非流式、但上游仍返回 text/event-stream
// 的场景，避免把 SSE 帧直接发给非流式客户端。
//
// 处理规则：
//   - content 按字符串顺序拼接；
//   - reasoning 依次兼容 reasoning_content / reasoning / reasoning_details（ExtractReasoning）；
//   - tool_calls 按 index 合并，arguments 拼接，id/name 后到补齐，稀疏 index 重新分配；
//   - finish_reason 取首个非空值，usage 取最后一个非 null；
//   - 兼容 [DONE]、空行、event/id/retry 行以及最后一条没有尾随换行的 data。
//
// 没有任何可解析的 chunk 或解析不出 choices 时返回错误。
func aggregateChatSSEToJSON(body []byte) ([]byte, error) {
	choices := make(map[int]*sseChoiceState)
	var (
		id        string
		model     string
		object    string
		created   int64
		lastUsage *dto.Usage
		chunkCnt  int
	)

	for _, rawLine := range bytes.Split(body, []byte("\n")) {
		line := strings.TrimSpace(strings.TrimRight(string(rawLine), "\r"))
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			// event:/id:/retry: 等 SSE 字段不参与聚合
			continue
		}
		payload := strings.TrimSpace(line[len("data:"):])
		if payload == "" || strings.HasPrefix(payload, "[DONE]") {
			continue
		}

		var chunk sseChatChunk
		if err := common.UnmarshalJsonStr(payload, &chunk); err != nil {
			// 兼容夹杂在 SSE 中的非 JSON data 行（例如纯文本心跳）
			continue
		}
		chunkCnt++

		if len(chunk.Error) > 0 && string(bytes.TrimSpace(chunk.Error)) != "null" {
			return nil, fmt.Errorf("upstream chat sse error: %s", sseErrorMessage(chunk.Error))
		}

		if chunk.Id != "" {
			id = chunk.Id
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.Object != "" {
			object = chunk.Object
		}
		if chunk.Created > 0 {
			created = chunk.Created
		}
		if chunk.Usage != nil {
			lastUsage = chunk.Usage
		}

		for _, choice := range chunk.Choices {
			if choice.Index < 0 {
				continue
			}
			state, ok := choices[choice.Index]
			if !ok {
				if len(choices) >= maxAggregatedChoices {
					return nil, fmt.Errorf("too many choices in chat sse response")
				}
				state = &sseChoiceState{
					toolCalls:     make(map[int]*sseToolCallState),
					remappedIndex: make(map[int]int),
				}
				choices[choice.Index] = state
			}
			if choice.Delta.Role != "" {
				state.role = choice.Delta.Role
			}
			if content := choice.Delta.GetContentString(); content != "" {
				state.content.WriteString(content)
			}
			if reasoning := choice.Delta.ExtractReasoning(); reasoning != "" {
				state.reasoning.WriteString(reasoning)
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" && state.finishReason == "" {
				state.finishReason = *choice.FinishReason
			}
			for pos, toolCall := range choice.Delta.ToolCalls {
				state.mergeToolCall(pos, toolCall)
			}
		}
	}

	if chunkCnt == 0 {
		return nil, fmt.Errorf("no chat completion chunk found in sse body")
	}
	if len(choices) == 0 {
		return nil, fmt.Errorf("no choices found in chat sse response")
	}

	indexes := make([]int, 0, len(choices))
	for index := range choices {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	outChoices := make([]dto.OpenAITextResponseChoice, 0, len(indexes))
	for _, index := range indexes {
		state := choices[index]
		message := dto.Message{Role: state.role}
		if message.Role == "" {
			message.Role = "assistant"
		}
		message.SetStringContent(state.content.String())
		if reasoning := state.reasoning.String(); reasoning != "" {
			message.ReasoningContent = &reasoning
		}
		if len(state.toolCalls) > 0 {
			toolCalls := make([]dto.ToolCallResponse, 0, len(state.toolCalls))
			for _, toolIndex := range state.sortedToolIndexes() {
				toolCall := state.toolCalls[toolIndex]
				typ := toolCall.typ
				if typ == "" {
					typ = "function"
				}
				toolCalls = append(toolCalls, dto.ToolCallResponse{
					ID:   toolCall.id,
					Type: typ,
					Function: dto.FunctionResponse{
						Name:      toolCall.name,
						Arguments: toolCall.arguments.String(),
					},
				})
			}
			message.SetToolCalls(toolCalls)
		}
		outChoices = append(outChoices, dto.OpenAITextResponseChoice{
			Index:        index,
			Message:      message,
			FinishReason: state.finishReason,
		})
	}

	if object == "" {
		object = "chat.completion"
	}
	response := dto.OpenAITextResponse{
		Id:      id,
		Model:   model,
		Object:  object,
		Created: created,
		Choices: outChoices,
	}
	if lastUsage != nil {
		response.Usage = *lastUsage
	}
	return common.Marshal(response)
}

// mergeToolCall 把一条流式 tool_call 分片合并进当前 choice 状态。
func (s *sseChoiceState) mergeToolCall(position int, toolCall dto.ToolCallResponse) {
	index := -1
	if toolCall.Index != nil {
		index = *toolCall.Index
	} else if toolCall.ID != "" {
		// 没有 index 时优先按 id 找回已存在的工具调用。
		for existingIndex, existing := range s.toolCalls {
			if existing.id == toolCall.ID {
				index = existingIndex
				break
			}
		}
	}
	if index < 0 {
		// 仍无法匹配时按本 chunk 内的位置分配，兼容不带 index 的旧式上游。
		index = position
	}
	if index > maxAggregatedToolCallIndex {
		// 稀疏/异常 index：重新分配一个安全下标，避免外部按 index 分配大数组导致 OOM。
		// 重新分配的下标从 maxAggregatedToolCallIndex+1 开始，不会与真实下标冲突。
		remapped, ok := s.remappedIndex[index]
		if !ok {
			remapped = maxAggregatedToolCallIndex + 1 + len(s.remappedIndex)
			s.remappedIndex[index] = remapped
		}
		index = remapped
	}

	state, ok := s.toolCalls[index]
	if !ok {
		if len(s.toolCalls) >= maxAggregatedToolCalls {
			return
		}
		state = &sseToolCallState{}
		s.toolCalls[index] = state
	}

	if toolCall.ID != "" && state.id == "" {
		state.id = toolCall.ID
	}
	if typ, ok := toolCall.Type.(string); ok && typ != "" && state.typ == "" {
		state.typ = typ
	}
	// name 正常一次性到达；若上游把 name 分片发送，则按前缀/后缀关系补齐。
	if name := toolCall.Function.Name; name != "" {
		switch {
		case state.name == "":
			state.name = name
		case state.name == name || strings.HasSuffix(state.name, name):
			// 已包含，忽略重复分片
		default:
			state.name += name
		}
	}
	state.arguments.WriteString(toolCall.Function.Arguments)
}

// sortedToolIndexes 返回按 index 排序的工具调用下标，保证输出顺序稳定。
func (s *sseChoiceState) sortedToolIndexes() []int {
	indexes := make([]int, 0, len(s.toolCalls))
	for index := range s.toolCalls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	return indexes
}

// sseErrorMessage 从 SSE data 里的 error 字段提取可读信息。
func sseErrorMessage(raw json.RawMessage) string {
	var parsed struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	if err := common.Unmarshal(raw, &parsed); err == nil && parsed.Message != "" {
		return parsed.Message
	}
	return strings.TrimSpace(string(raw))
}

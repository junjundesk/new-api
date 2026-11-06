package oaichat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/reasonmap"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/samber/lo"
)

// maxConsecutiveBlankArgs 是连续空白参数字符的阈值：超过后认为上游陷入
// 无限输出空白的 bug，中止该 tool call。
const maxConsecutiveBlankArgs = 500

// inlineThinkTagPairs 是内联思考标签对，只识别正文开头的块。
var inlineThinkTagPairs = [][2]string{
	{"<think>", "</think>"},
	{"<thinking>", "</thinking>"},
}

func generateStopBlock(index int) *dto.ClaudeResponse {
	return &dto.ClaudeResponse{
		Type:  "content_block_stop",
		Index: kitutil.GetPointer[int](index),
	}
}

// stopOpenBlocks 关闭当前打开的块：文本/思考块至多一个，工具块按 Anthropic index
// 升序逐个关闭；只对真正 start 过的工具块发 content_block_stop。
func stopOpenBlocks(state *convmeta.ClaudeConvertInfo) []*dto.ClaudeResponse {
	if state == nil {
		return nil
	}
	var responses []*dto.ClaudeResponse
	switch state.LastMessagesType {
	case convmeta.LastMessageTypeText, convmeta.LastMessageTypeThinking:
		responses = append(responses, generateStopBlock(state.Index))
	case convmeta.LastMessageTypeTools:
		responses = append(responses, stopStartedToolBlocks(state)...)
	}
	state.LastMessagesType = convmeta.LastMessageTypeNone
	return responses
}

// stopStartedToolBlocks 按 Anthropic index 升序关闭所有已启动且未关闭的工具块。
func stopStartedToolBlocks(state *convmeta.ClaudeConvertInfo) []*dto.ClaudeResponse {
	indices := make([]int, 0, len(state.ToolBlocks))
	for _, block := range state.ToolBlocks {
		if block == nil || !block.Started || block.Closed {
			continue
		}
		indices = append(indices, block.AnthropicIndex)
	}
	if len(indices) == 0 {
		return nil
	}
	sort.Ints(indices)
	for _, block := range state.ToolBlocks {
		if block != nil && block.Started && !block.Closed {
			block.Closed = true
		}
	}
	responses := make([]*dto.ClaudeResponse, 0, len(indices))
	for _, index := range indices {
		responses = append(responses, generateStopBlock(index))
	}
	return responses
}

// nextBlockIndex 分配下一个 content block index。
func nextBlockIndex(state *convmeta.ClaudeConvertInfo) int {
	index := state.NextBlockIndex
	state.NextBlockIndex = index + 1
	return index
}

// ensureNonToolBlock 确保当前打开的是指定类型（text/thinking）的块，必要时先关闭
// 已有块再分配新 index 并发出 content_block_start。
func ensureNonToolBlock(state *convmeta.ClaudeConvertInfo, blockType string) []*dto.ClaudeResponse {
	if state.LastMessagesType == blockType {
		return nil
	}
	responses := stopOpenBlocks(state)
	index := nextBlockIndex(state)
	state.Index = index
	state.LastMessagesType = blockType
	var block *dto.ClaudeMediaMessage
	if blockType == convmeta.LastMessageTypeThinking {
		block = &dto.ClaudeMediaMessage{Type: "thinking", Thinking: kitutil.GetPointer[string]("")}
	} else {
		block = &dto.ClaudeMediaMessage{Type: "text", Text: kitutil.GetPointer[string]("")}
	}
	return append(responses, &dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        kitutil.GetPointer[int](index),
		ContentBlock: block,
	})
}

func thinkingDeltaResponse(index int, thinking string) *dto.ClaudeResponse {
	return &dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: kitutil.GetPointer[int](index),
		Delta: &dto.ClaudeMediaMessage{
			Type:     "thinking_delta",
			Thinking: kitutil.GetPointer[string](thinking),
		},
	}
}

func textDeltaResponse(index int, text string) *dto.ClaudeResponse {
	return &dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: kitutil.GetPointer[int](index),
		Delta: &dto.ClaudeMediaMessage{
			Type: "text_delta",
			Text: kitutil.GetPointer[string](text),
		},
	}
}

func inputJSONDeltaResponse(index int, partialJSON string) *dto.ClaudeResponse {
	return &dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: kitutil.GetPointer[int](index),
		Delta: &dto.ClaudeMediaMessage{
			Type:        "input_json_delta",
			PartialJson: kitutil.GetPointer[string](partialJSON),
		},
	}
}

func toolUseStartResponse(index int, id string, name string) *dto.ClaudeResponse {
	return &dto.ClaudeResponse{
		Type:  "content_block_start",
		Index: kitutil.GetPointer[int](index),
		ContentBlock: &dto.ClaudeMediaMessage{
			Id:    id,
			Type:  "tool_use",
			Name:  name,
			Input: map[string]interface{}{},
		},
	}
}

// appendInlineThinkSegments 按 thinking 在前、text 在后的顺序下发拆分结果，
// 各自独立开块/切块。
func appendInlineThinkSegments(state *convmeta.ClaudeConvertInfo, responses []*dto.ClaudeResponse, thinking string, text string) []*dto.ClaudeResponse {
	if thinking != "" {
		responses = append(responses, ensureNonToolBlock(state, convmeta.LastMessageTypeThinking)...)
		responses = append(responses, thinkingDeltaResponse(state.Index, thinking))
	}
	if text != "" {
		responses = append(responses, ensureNonToolBlock(state, convmeta.LastMessageTypeText)...)
		responses = append(responses, textDeltaResponse(state.Index, text))
	}
	return responses
}

// ---------------------------------------------------------------------------
// 流首内联 think 拆分
// ---------------------------------------------------------------------------

const (
	thinkPrefixNeedMore = iota
	thinkPrefixReasoning
	thinkPrefixText
)

// leadingThinkPrefixDecision 判断缓冲区开头是否最终会构成某个 think 开标签。
func leadingThinkPrefixDecision(buffer string) int {
	trimmed := strings.TrimLeftFunc(buffer, unicode.IsSpace)
	if trimmed == "" {
		return thinkPrefixNeedMore
	}
	for _, pair := range inlineThinkTagPairs {
		if strings.HasPrefix(trimmed, pair[0]) {
			return thinkPrefixReasoning
		}
	}
	for _, pair := range inlineThinkTagPairs {
		if strings.HasPrefix(pair[0], trimmed) {
			return thinkPrefixNeedMore
		}
	}
	return thinkPrefixText
}

// stripLeadingThinkOpenTag 剥掉开头的空白与开标签，返回标签之后的内容。
func stripLeadingThinkOpenTag(text string) (string, bool) {
	trimmed := strings.TrimLeftFunc(text, unicode.IsSpace)
	for _, pair := range inlineThinkTagPairs {
		if rest, found := strings.CutPrefix(trimmed, pair[0]); found {
			return rest, true
		}
	}
	return "", false
}

// findThinkCloseTag 在文本里找最早出现的闭合标签。容忍开闭标签不配对：
// 只认配对闭合会让整段正文被当成推理内容吞掉。
func findThinkCloseTag(text string) (int, string) {
	bestIndex := -1
	bestTag := ""
	for _, pair := range inlineThinkTagPairs {
		index := strings.Index(text, pair[1])
		if index >= 0 && (bestIndex < 0 || index < bestIndex) {
			bestIndex = index
			bestTag = pair[1]
		}
	}
	return bestIndex, bestTag
}

// splitLeadingThinkBlock 把正文开头的 think 块拆成（思考, 正文）。
// 只处理首段非空文本：标签前允许空白，闭合标签后紧邻的一个换行被吞掉；
// 找不到开标签或标签未闭合时 ok=false，整段按普通文本处理。
func splitLeadingThinkBlock(text string) (string, string, bool) {
	body, found := stripLeadingThinkOpenTag(text)
	if !found {
		return "", text, false
	}
	index, closeTag := findThinkCloseTag(body)
	if index < 0 {
		return "", text, false
	}
	thinking := strings.Trim(body[:index], "\r\n")
	answer := strings.TrimLeft(body[index+len(closeTag):], "\r\n")
	return thinking, answer, true
}

// reasoningHoldbackLen 返回 Reasoning 态下需扣住不下发的尾部长度：末尾可能是某个
// 闭合标签真前缀的最长后缀，以及它前面的换行分隔符。
func reasoningHoldbackLen(buffer string) int {
	partial := 0
	for _, pair := range inlineThinkTagPairs {
		closeTag := pair[1]
		for length := len(closeTag) - 1; length >= 1; length-- {
			if length > len(buffer) {
				continue
			}
			if strings.HasSuffix(buffer, closeTag[:length]) {
				if length > partial {
					partial = length
				}
				break
			}
		}
	}
	body := buffer[:len(buffer)-partial]
	return len(buffer) - len(strings.TrimRight(body, "\r\n"))
}

// pushInlineThinkText 在 Text 态透传正文，吞掉标签后紧邻的换行分隔符。
func pushInlineThinkText(state *convmeta.ClaudeConvertInfo, delta string) string {
	if !state.InlineThinkStripSeparator {
		return delta
	}
	rest := strings.TrimLeft(delta, "\r\n")
	if rest == "" {
		return ""
	}
	state.InlineThinkStripSeparator = false
	return rest
}

// drainInlineThinkReasoning 在 Reasoning 态推进：闭合标签完整出现即转 Text，
// 否则除扣住的短尾外，思考内容即时下发。
func drainInlineThinkReasoning(state *convmeta.ClaudeConvertInfo) (string, string) {
	if state.InlineThinkStripSeparator {
		remainder := strings.TrimLeft(state.InlineThinkBuffer, "\r\n")
		state.InlineThinkBuffer = remainder
		if remainder == "" {
			return "", ""
		}
		state.InlineThinkStripSeparator = false
	}
	if index, closeTag := findThinkCloseTag(state.InlineThinkBuffer); index >= 0 {
		buffered := state.InlineThinkBuffer
		state.InlineThinkBuffer = ""
		state.InlineThinkPhase = convmeta.InlineThinkPhaseText
		state.InlineThinkStripSeparator = true
		thinking := strings.TrimRight(buffered[:index], "\r\n")
		text := pushInlineThinkText(state, buffered[index+len(closeTag):])
		return thinking, text
	}
	holdback := reasoningHoldbackLen(state.InlineThinkBuffer)
	splitAt := len(state.InlineThinkBuffer) - holdback
	thinking := state.InlineThinkBuffer[:splitAt]
	state.InlineThinkBuffer = state.InlineThinkBuffer[splitAt:]
	return thinking, ""
}

// pushInlineThink 喂入一个 content 增量，返回（思考增量, 正文增量）。
func pushInlineThink(state *convmeta.ClaudeConvertInfo, delta string) (string, string) {
	switch state.InlineThinkPhase {
	case convmeta.InlineThinkPhaseText:
		return "", pushInlineThinkText(state, delta)
	case convmeta.InlineThinkPhaseReasoning:
		state.InlineThinkBuffer += delta
		return drainInlineThinkReasoning(state)
	default:
		state.InlineThinkBuffer += delta
		switch leadingThinkPrefixDecision(state.InlineThinkBuffer) {
		case thinkPrefixNeedMore:
			return "", ""
		case thinkPrefixReasoning:
			state.InlineThinkPhase = convmeta.InlineThinkPhaseReasoning
			state.InlineThinkStripSeparator = true
			body, _ := stripLeadingThinkOpenTag(state.InlineThinkBuffer)
			state.InlineThinkBuffer = body
			return drainInlineThinkReasoning(state)
		default:
			state.InlineThinkPhase = convmeta.InlineThinkPhaseText
			text := state.InlineThinkBuffer
			state.InlineThinkBuffer = ""
			return "", text
		}
	}
}

// flushInlineThink 在边界处冲刷拆分器残留：未闭合的 think 块按思考内容下发，
// Detecting 态缓冲按正文下发。幂等。
func flushInlineThink(state *convmeta.ClaudeConvertInfo) (string, string) {
	buffered := state.InlineThinkBuffer
	wasReasoning := state.InlineThinkPhase == convmeta.InlineThinkPhaseReasoning
	state.InlineThinkPhase = convmeta.InlineThinkPhaseText
	state.InlineThinkBuffer = ""
	state.InlineThinkStripSeparator = false
	if wasReasoning {
		return buffered, ""
	}
	return "", buffered
}

// ---------------------------------------------------------------------------
// 工具块状态机
// ---------------------------------------------------------------------------

// emitToolCallDelta 按 OpenAI tool_call.index 维护块状态：
// id 与 name 都就绪才发 content_block_start，之前的参数先缓存、启动时补发。
func emitToolCallDelta(state *convmeta.ClaudeConvertInfo, toolCall dto.ToolCallResponse, fallbackIndex int) []*dto.ClaudeResponse {
	toolIndex := fallbackIndex
	if toolCall.Index != nil {
		toolIndex = *toolCall.Index
	}
	if state.ToolBlocks == nil {
		state.ToolBlocks = make(map[int]*convmeta.ToolBlockState)
	}
	block := state.ToolBlocks[toolIndex]
	if block == nil {
		block = &convmeta.ToolBlockState{AnthropicIndex: nextBlockIndex(state)}
		state.ToolBlocks[toolIndex] = block
	}
	if block.Aborted || block.Closed {
		return nil
	}
	if toolCall.ID != "" {
		block.ID = toolCall.ID
	}
	if toolCall.Function.Name != "" {
		block.Name = toolCall.Function.Name
	}

	arguments := toolCall.Function.Arguments
	if arguments != "" {
		for _, char := range arguments {
			if unicode.IsSpace(char) {
				block.BlankArgsLen++
			} else {
				block.BlankArgsLen = 0
			}
		}
		if block.BlankArgsLen >= maxConsecutiveBlankArgs {
			block.Aborted = true
			log.Printf("[relayconvert] 检测到上游无限空白参数（tool: %q, index: %d），中止该 tool call", block.Name, toolIndex)
			return nil
		}
		if !block.Started {
			block.PendingArgs += arguments
		}
	}

	var responses []*dto.ClaudeResponse
	if !block.Started && block.ID != "" && block.Name != "" {
		// 迟到就绪：先开块，再把缓存参数作为第一条 input_json_delta 补发。
		block.Started = true
		responses = append(responses, toolUseStartResponse(block.AnthropicIndex, block.ID, block.Name))
		if block.PendingArgs != "" {
			responses = append(responses, inputJSONDeltaResponse(block.AnthropicIndex, block.PendingArgs))
			block.PendingArgs = ""
		}
		return responses
	}
	if block.Started && arguments != "" {
		responses = append(responses, inputJSONDeltaResponse(block.AnthropicIndex, arguments))
	}
	return responses
}

// lateStartToolBlocks 在 finish 边界对未启动但已有 payload 的工具补发
// content_block_start（缺省 id/name 兜底），随后统一由 stopOpenBlocks 关闭。
func lateStartToolBlocks(state *convmeta.ClaudeConvertInfo) []*dto.ClaudeResponse {
	if len(state.ToolBlocks) == 0 {
		return nil
	}
	type lateStart struct {
		index   int
		id      string
		name    string
		pending string
	}
	starts := make([]lateStart, 0, len(state.ToolBlocks))
	for toolIndex, block := range state.ToolBlocks {
		if block == nil || block.Started || block.Closed || block.Aborted {
			continue
		}
		if block.PendingArgs == "" && block.ID == "" && block.Name == "" {
			continue
		}
		id := block.ID
		if id == "" {
			id = fmt.Sprintf("tool_call_%d", toolIndex)
		}
		name := block.Name
		if name == "" {
			name = "unknown_tool"
		}
		block.Started = true
		starts = append(starts, lateStart{index: block.AnthropicIndex, id: id, name: name, pending: block.PendingArgs})
		block.PendingArgs = ""
	}
	if len(starts) == 0 {
		return nil
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i].index < starts[j].index })
	responses := make([]*dto.ClaudeResponse, 0, len(starts)*2)
	for _, start := range starts {
		responses = append(responses, toolUseStartResponse(start.index, start.id, start.name))
		if start.pending != "" {
			responses = append(responses, inputJSONDeltaResponse(start.index, start.pending))
		}
	}
	return responses
}

// ---------------------------------------------------------------------------
// usage
// ---------------------------------------------------------------------------

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func buildClaudeUsageFromOpenAIUsage(oaiUsage *dto.Usage) *dto.ClaudeUsage {
	if oaiUsage == nil {
		return nil
	}
	if billingUsage := dto.CloneBillingUsage(oaiUsage.BillingUsage); billingUsage != nil && billingUsage.ClaudeUsage != nil {
		if billingUsage.Source == dto.BillingUsageSourceClaudeMessages || billingUsage.Semantic == dto.BillingUsageSemanticAnthropic {
			return billingUsage.ClaudeUsage
		}
	}
	billingUsage := dto.NewOpenAIChatBillingUsage(oaiUsage)
	if existingBillingUsage := dto.CloneBillingUsage(oaiUsage.BillingUsage); existingBillingUsage != nil && existingBillingUsage.OpenAIUsage != nil {
		if existingBillingUsage.Source == dto.BillingUsageSourceOAIChat ||
			existingBillingUsage.Source == dto.BillingUsageSourceOAIResponses ||
			existingBillingUsage.Semantic == dto.BillingUsageSemanticOpenAI {
			billingUsage = existingBillingUsage
		}
	}
	cacheCreation5m, cacheCreation1h := NormalizeCacheCreationSplit(
		oaiUsage.PromptTokensDetails.CachedCreationTokens,
		oaiUsage.ClaudeCacheCreation5mTokens,
		oaiUsage.ClaudeCacheCreation1hTokens,
	)
	// 缓存桶多级回退：显式 0 视为未上报，继续向后找，以兼容把第一环硬编码为桩 0 的中转。
	var inputCachedTokens int
	var inputCacheWriteTokens int
	if oaiUsage.InputTokensDetails != nil {
		inputCachedTokens = oaiUsage.InputTokensDetails.CachedTokens
		inputCacheWriteTokens = oaiUsage.InputTokensDetails.CacheCreationTokensTotal()
	}
	cacheReadTokens := firstPositive(
		oaiUsage.PromptTokensDetails.CachedTokens,
		inputCachedTokens,
		oaiUsage.PromptCacheHitTokens,
	)
	cacheCreationTokens := firstPositive(
		oaiUsage.PromptTokensDetails.CacheCreationTokensTotal(),
		inputCacheWriteTokens,
	)
	// 上游采用 inclusive 语义：prompt_tokens 已包含缓存读写，而 Claude 的
	// input_tokens 不含两者。无条件扣减以保证
	// input_tokens + cache_read + cache_creation == prompt_tokens（负值钳 0）。
	inputTokens := oaiUsage.PromptTokens - cacheReadTokens - cacheCreationTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	usage := &dto.ClaudeUsage{
		InputTokens:              inputTokens,
		OutputTokens:             oaiUsage.CompletionTokens,
		CacheCreationInputTokens: cacheCreationTokens,
		CacheReadInputTokens:     cacheReadTokens,
		BillingUsage:             billingUsage,
	}
	if cacheCreation5m > 0 || cacheCreation1h > 0 {
		usage.CacheCreation = &dto.ClaudeCacheCreationUsage{
			Ephemeral5mInputTokens: cacheCreation5m,
			Ephemeral1hInputTokens: cacheCreation1h,
		}
	}
	return usage
}

// claudeUsageOrZero 返回 Claude 口径的 usage，上游没有上报时返回零值对象，
// 保证 message_delta 的 usage 字段始终存在。
func claudeUsageOrZero(oaiUsage *dto.Usage) *dto.ClaudeUsage {
	if usage := buildClaudeUsageFromOpenAIUsage(oaiUsage); usage != nil {
		return usage
	}
	return &dto.ClaudeUsage{}
}

// stateUsage 返回终态 message_delta 使用的上游 usage：优先取宿主/状态式管线
// 维护的聚合 usage，其次取转换器见过的最后一个 chunk usage。
func stateUsage(state *convmeta.ClaudeConvertInfo) *dto.Usage {
	if state.Usage != nil {
		return state.Usage
	}
	return state.LastChunkUsage
}

func NormalizeCacheCreationSplit(totalTokens int, tokens5m int, tokens1h int) (int, int) {
	remainder := lo.Max([]int{totalTokens - tokens5m - tokens1h, 0})
	return tokens5m + remainder, tokens1h
}

// ---------------------------------------------------------------------------
// 流式转换
// ---------------------------------------------------------------------------

// buildClaudeMessageStart 构造 message_start。input_tokens 优先采用上游首 chunk
// 的 usage，未上报时保留本地估算。
func buildClaudeMessageStart(state *convmeta.ClaudeConvertInfo, info convmeta.Meta) *dto.ClaudeResponse {
	usage := &dto.ClaudeUsage{InputTokens: info.GetEstimatePromptTokens()}
	if chunkUsage := stateUsage(state); chunkUsage != nil && chunkUsage.PromptTokens > 0 {
		if built := buildClaudeUsageFromOpenAIUsage(chunkUsage); built != nil {
			clone := *built
			clone.OutputTokens = 0
			usage = &clone
		}
	}
	msg := &dto.ClaudeMediaMessage{
		Id:    state.MessageId,
		Model: state.Model,
		Type:  "message",
		Role:  "assistant",
		Usage: usage,
	}
	msg.SetContent(make([]any, 0))
	return &dto.ClaudeResponse{
		Type:    "message_start",
		Message: msg,
	}
}

// emitTerminalResponses 按 Anthropic 规范收尾：冲刷增量 → 工具迟到启动 →
// 关闭打开的块 → message_delta → message_stop，并置 Done 保证幂等。
func emitTerminalResponses(state *convmeta.ClaudeConvertInfo) []*dto.ClaudeResponse {
	var responses []*dto.ClaudeResponse
	thinking, text := flushInlineThink(state)
	responses = appendInlineThinkSegments(state, responses, thinking, text)
	responses = append(responses, lateStartToolBlocks(state)...)
	responses = append(responses, stopOpenBlocks(state)...)
	stopReason := stopReasonOpenAI2Claude(state.FinishReason)
	if stopReason == "" {
		stopReason = "end_turn"
	}
	responses = append(responses, &dto.ClaudeResponse{
		Type:  "message_delta",
		Usage: claudeUsageOrZero(stateUsage(state)),
		Delta: &dto.ClaudeMediaMessage{
			StopReason: kitutil.GetPointer[string](stopReason),
		},
	})
	responses = append(responses, &dto.ClaudeResponse{Type: "message_stop"})
	state.Done = true
	return responses
}

func StreamResponseOpenAI2Claude(openAIResponse *dto.ChatCompletionsStreamResponse, info convmeta.Meta) []*dto.ClaudeResponse {
	if info == nil {
		info = &convmeta.Values{}
	}
	state := info.EnsureClaudeConvertInfo()
	if state == nil || openAIResponse == nil {
		return nil
	}

	// 记录 id / model / usage，供 message_start 与 finalize 使用。
	if openAIResponse.Id != "" {
		state.MessageId = openAIResponse.Id
	}
	if openAIResponse.Model != "" {
		state.Model = openAIResponse.Model
	}
	if openAIResponse.Usage != nil {
		// 不覆盖 state.Usage：它由宿主或状态式管线按聚合口径维护。
		state.LastChunkUsage = openAIResponse.Usage
	}

	if state.Done {
		// 重复的终态 chunk（含多个 finish_reason）只更新 usage，不重复发事件。
		return nil
	}

	var claudeResponses []*dto.ClaudeResponse

	// message_start 用状态标志触发：单 chunk 流与 finalize 路径也能发出，
	// 不再依赖 SendResponseCount。
	if !state.HasSentMessageStart {
		claudeResponses = append(claudeResponses, buildClaudeMessageStart(state, info))
		state.HasSentMessageStart = true
	}

	if len(openAIResponse.Choices) > 0 {
		chosenChoice := openAIResponse.Choices[0]
		delta := chosenChoice.Delta

		// reasoning 与 content 顺序处理，各自独立开块/切块（不再 if/else 二选一）。
		if reasoning := delta.ExtractReasoning(); reasoning != "" {
			claudeResponses = append(claudeResponses, ensureNonToolBlock(state, convmeta.LastMessageTypeThinking)...)
			claudeResponses = append(claudeResponses, thinkingDeltaResponse(state.Index, reasoning))
		}
		if content := delta.GetContentString(); content != "" {
			thinking, text := pushInlineThink(state, content)
			claudeResponses = appendInlineThinkSegments(state, claudeResponses, thinking, text)
		}

		if len(delta.ToolCalls) > 0 {
			// 工具边界：先冲刷内联 think 残留，再关闭非工具块，避免块交叠。
			thinking, text := flushInlineThink(state)
			claudeResponses = appendInlineThinkSegments(state, claudeResponses, thinking, text)
			if state.LastMessagesType != convmeta.LastMessageTypeTools {
				claudeResponses = append(claudeResponses, stopOpenBlocks(state)...)
				state.LastMessagesType = convmeta.LastMessageTypeTools
			}
			for i, toolCall := range delta.ToolCalls {
				claudeResponses = append(claudeResponses, emitToolCallDelta(state, toolCall, i)...)
			}
		}

		if chosenChoice.FinishReason != nil && *chosenChoice.FinishReason != "" {
			state.FinishReason = *chosenChoice.FinishReason
			state.FinishSeen = true
		}
	}

	// finish chunk 没有 usage 时不直接收尾：记录 finish_reason，等待后续 usage
	// chunk 或 Finalize 补发终态。拿到 usage 后立即收尾，保证任何形状都有终态事件。
	if state.FinishSeen {
		if stateUsage(state) != nil {
			claudeResponses = append(claudeResponses, emitTerminalResponses(state)...)
		}
	}
	return claudeResponses
}

func FinalizeStreamResponseOpenAI2Claude(info convmeta.Meta) []*dto.ClaudeResponse {
	if info == nil {
		info = &convmeta.Values{}
	}
	state := info.EnsureClaudeConvertInfo()
	if state == nil || state.Done {
		// 幂等：Done 之后反复调用不重复发事件。
		return nil
	}
	var claudeResponses []*dto.ClaudeResponse
	if !state.HasSentMessageStart {
		claudeResponses = append(claudeResponses, buildClaudeMessageStart(state, info))
		state.HasSentMessageStart = true
	}
	return append(claudeResponses, emitTerminalResponses(state)...)
}

// ---------------------------------------------------------------------------
// 非流式转换
// ---------------------------------------------------------------------------

// legacyFunctionCall 是上游 legacy message.function_call 的解析结构。
type legacyFunctionCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// legacyFunctionCallInput 把 legacy function_call 的 arguments 转成 tool_use 的 input：
// 字符串形态按其中 JSON 解析，对象/数组直接作为 input。
func legacyFunctionCallInput(arguments json.RawMessage) any {
	trimmed := bytes.TrimSpace(arguments)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return map[string]interface{}{}
	}
	if trimmed[0] == '"' {
		// 字符串形态：解码出内部的 JSON 文本后再解析，对象/数组都保留。
		var raw string
		if err := json.Unmarshal(trimmed, &raw); err != nil || strings.TrimSpace(raw) == "" {
			return map[string]interface{}{}
		}
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return map[string]interface{}{}
		}
		switch value.(type) {
		case map[string]interface{}, []any:
			return value
		default:
			return map[string]interface{}{}
		}
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil || value == nil {
		return map[string]interface{}{}
	}
	return value
}

// parseLegacyFunctionCall 解析上游 legacy function_call，无有效内容时返回 nil。
func parseLegacyFunctionCall(raw json.RawMessage) *legacyFunctionCall {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	var call legacyFunctionCall
	if err := json.Unmarshal(trimmed, &call); err != nil {
		return nil
	}
	if call.Name == "" && len(bytes.TrimSpace(call.Arguments)) == 0 {
		return nil
	}
	return &call
}

func ResponseOpenAI2Claude(openAIResponse *dto.OpenAITextResponse, info convmeta.Meta) *dto.ClaudeResponse {
	var stopReason string
	contents := make([]dto.ClaudeMediaMessage, 0)
	claudeResponse := &dto.ClaudeResponse{
		Id:    openAIResponse.Id,
		Type:  "message",
		Role:  "assistant",
		Model: openAIResponse.Model,
	}
	hasToolUse := false
	leadingTextHandled := false
	for _, choice := range openAIResponse.Choices {
		if choice.FinishReason != "" {
			stopReason = stopReasonOpenAI2Claude(choice.FinishReason)
		}

		// 先压 thinking 块（上游 reasoning_content / reasoning 非空才压），
		// 保证非流式与流式的 thinking 行为一致。
		if reasoning := choice.Message.ExtractReasoning(); reasoning != "" {
			thinking := reasoning
			contents = append(contents, dto.ClaudeMediaMessage{Type: "thinking", Thinking: &thinking})
		}

		// 只对首段非空文本尝试拆分内联 <think> 块。
		textContent := choice.Message.StringContent()
		if textContent != "" && !leadingTextHandled {
			leadingTextHandled = true
			if thinking, answer, ok := splitLeadingThinkBlock(textContent); ok {
				if thinking != "" {
					thinkingValue := thinking
					contents = append(contents, dto.ClaudeMediaMessage{Type: "thinking", Thinking: &thinkingValue})
				}
				if answer != "" {
					content := dto.ClaudeMediaMessage{Type: "text"}
					content.SetText(answer)
					contents = append(contents, content)
				}
			} else {
				content := dto.ClaudeMediaMessage{Type: "text"}
				content.SetText(textContent)
				contents = append(contents, content)
			}
		} else if textContent != "" {
			content := dto.ClaudeMediaMessage{Type: "text"}
			content.SetText(textContent)
			contents = append(contents, content)
		}

		toolCalls := choice.Message.ParseToolCalls()
		for _, toolUse := range toolCalls {
			claudeContent := dto.ClaudeMediaMessage{}
			claudeContent.Type = "tool_use"
			claudeContent.Id = toolUse.ID
			claudeContent.Name = toolUse.Function.Name
			mapParams := map[string]interface{}{}
			if strings.TrimSpace(toolUse.Function.Arguments) != "" {
				var parsed map[string]interface{}
				if err := kitutil.Unmarshal([]byte(toolUse.Function.Arguments), &parsed); err == nil && parsed != nil {
					mapParams = parsed
				}
			}
			claudeContent.Input = mapParams
			contents = append(contents, claudeContent)
			hasToolUse = true
		}

		// legacy function_call：没有 tool_calls 时才转换，兼容旧式上游。
		if len(toolCalls) == 0 {
			if legacy := parseLegacyFunctionCall(choice.Message.FunctionCall); legacy != nil {
				contents = append(contents, dto.ClaudeMediaMessage{
					Type:  "tool_use",
					Id:    legacy.ID,
					Name:  legacy.Name,
					Input: legacyFunctionCallInput(legacy.Arguments),
				})
				hasToolUse = true
			}
		}
	}

	// Anthropic 要求 stop_reason 是合法枚举；finish_reason 缺失时按是否产出
	// tool_use 块回填，避免被 omitempty 吃掉整个字段。
	if stopReason == "" {
		if hasToolUse {
			stopReason = "tool_use"
		} else {
			stopReason = "end_turn"
		}
	}

	claudeResponse.Content = contents
	claudeResponse.StopReason = stopReason
	claudeResponse.Usage = buildClaudeUsageFromOpenAIUsage(&openAIResponse.Usage)

	return claudeResponse
}

// stopReasonOpenAI2Claude 把 OpenAI 的 finish_reason 映射为 Anthropic 的 stop_reason。
// 空值返回空串由调用方兜底；未知值记日志后按 end_turn 处理。
func stopReasonOpenAI2Claude(reason string) string {
	finishReason := strings.ToLower(strings.TrimSpace(reason))
	switch finishReason {
	case "":
		return ""
	case "function_call":
		// 旧式 function_call 与 tool_calls 同义，reasonmap 只映射后者。
		return "tool_use"
	case "stop", "stop_sequence", "length", "max_tokens", "tool_calls", types.FinishReasonContentFilter:
		return reasonmap.OpenAIFinishReasonToClaudeStopReason(finishReason)
	default:
		log.Printf("[relayconvert] 未知的 finish_reason %q，按 end_turn 处理", reason)
		return "end_turn"
	}
}

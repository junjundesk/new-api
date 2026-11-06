package openai

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

// 辅助函数
func HandleStreamFormat(c *gin.Context, info *relaycommon.RelayInfo, data string, forceFormat bool, thinkToContent bool) error {
	switch info.RelayFormat {
	case types.RelayFormatOpenAI:
		info.SendResponseCount++
		return sendStreamData(c, info, data, forceFormat, thinkToContent)
	case types.RelayFormatClaude:
		// Claude 转换在解析成功后再计数：只有真正进入转换的 chunk 才应被算作
		// 已产生的响应（供 fallback / 重试判断），解析失败的脏数据不能占用计数。
		return handleClaudeFormat(c, data, info)
	case types.RelayFormatGemini:
		info.SendResponseCount++
		return handleGeminiFormat(c, data, info)
	}
	return nil
}

// claudeMessageStartSentKey 记录当前流是否已经下发过 message_start。
// 收尾补发 message_delta/message_stop 前必须确认它已发出，否则会形成非法事件序列。
const claudeMessageStartSentKey = "claude_message_start_sent"

func handleClaudeFormat(c *gin.Context, data string, info *relaycommon.RelayInfo) error {
	var streamResponse dto.ChatCompletionsStreamResponse
	if err := common.Unmarshal(common.StringToByteSlice(data), &streamResponse); err != nil {
		return err
	}

	info.SendResponseCount++
	if streamResponse.Usage != nil {
		info.ClaudeConvertInfo.Usage = streamResponse.Usage
	}
	result, err := relayconvert.ConvertStreamResponse(c, info, types.RelayFormatClaude, &streamResponse)
	if err != nil {
		return err
	}
	claudeResponses, ok := result.Value.([]*dto.ClaudeResponse)
	if !ok {
		return fmt.Errorf("expected Claude stream responses, got %T", result.Value)
	}
	writeClaudeResponses(c, info, claudeResponses)
	return nil
}

// writeClaudeResponses 按顺序写出 Claude SSE 事件，并记录 message_start/message_stop 状态。
func writeClaudeResponses(c *gin.Context, info *relaycommon.RelayInfo, responses []*dto.ClaudeResponse) {
	for _, resp := range responses {
		if resp == nil {
			continue
		}
		switch resp.Type {
		case "message_start":
			c.Set(claudeMessageStartSentKey, true)
		case "message_stop":
			info.EnsureClaudeConvertInfo().Done = true
		}
		_ = helper.ClaudeData(c, *resp)
	}
}

// claudeMessageStartSent 报告当前流是否已经写出过 message_start。
func claudeMessageStartSent(c *gin.Context) bool {
	if c == nil {
		return false
	}
	sent, ok := c.Get(claudeMessageStartSentKey)
	if !ok {
		return false
	}
	flag, ok := sent.(bool)
	return ok && flag
}

// finalizeClaudeStream 在流正常结束后补发缺失的终态事件（message_delta + message_stop）。
// 转换器自身可能在 finish chunk（带 usage）时已经发过终态，此时 Done 为真，直接返回。
func finalizeClaudeStream(c *gin.Context, info *relaycommon.RelayInfo) {
	if info == nil {
		return
	}
	claudeInfo := info.EnsureClaudeConvertInfo()
	if claudeInfo.Done {
		return
	}
	if !claudeMessageStartSent(c) {
		// 没有 message_start 的流再补 message_delta/message_stop 会形成非法事件序列，
		// 此时保持现状，让客户端按空流处理。
		return
	}

	streamState, err := relayconvert.NewResponseStreamState(types.RelayFormatOpenAI, types.RelayFormatClaude, relayconvert.ResponseStreamOptions{
		Model: info.GetUpstreamModelName(),
	})
	if err != nil {
		common.SysLog("error creating Claude stream state for finalize: " + err.Error())
		return
	}

	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	results, err := relayconvert.FinalizeStreamResponse(ctx, info, streamState)
	if err != nil {
		common.SysLog("error finalizing Claude stream response: " + err.Error())
		return
	}
	for _, result := range results {
		if resp, ok := result.Value.(*dto.ClaudeResponse); ok && resp != nil {
			writeClaudeResponses(c, info, []*dto.ClaudeResponse{resp})
		}
	}
}

func handleGeminiFormat(c *gin.Context, data string, info *relaycommon.RelayInfo) error {
	var streamResponse dto.ChatCompletionsStreamResponse
	if err := common.Unmarshal(common.StringToByteSlice(data), &streamResponse); err != nil {
		logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
		return err
	}

	result, err := relayconvert.ConvertStreamResponse(c, info, types.RelayFormatGemini, &streamResponse)
	if err != nil {
		return err
	}
	geminiResponse, ok := result.Value.(*dto.GeminiChatResponse)
	if !ok {
		return fmt.Errorf("expected Gemini stream response, got %T", result.Value)
	}

	// 如果返回 nil，表示没有实际内容，跳过发送
	if geminiResponse == nil {
		return nil
	}

	geminiResponseStr, err := common.Marshal(geminiResponse)
	if err != nil {
		logger.LogError(c, "failed to marshal gemini response: "+err.Error())
		return err
	}

	// send gemini format response
	c.Render(-1, common.CustomEvent{Data: "data: " + string(geminiResponseStr)})
	_ = helper.FlushWriter(c)
	return nil
}

func ProcessStreamResponse(streamResponse dto.ChatCompletionsStreamResponse, responseTextBuilder *strings.Builder, toolCount *int) error {
	for _, choice := range streamResponse.Choices {
		responseTextBuilder.WriteString(choice.Delta.GetContentString())
		responseTextBuilder.WriteString(choice.Delta.GetReasoningContent())
		if choice.Delta.ToolCalls != nil {
			if len(choice.Delta.ToolCalls) > *toolCount {
				*toolCount = len(choice.Delta.ToolCalls)
			}
			for _, tool := range choice.Delta.ToolCalls {
				responseTextBuilder.WriteString(tool.Function.Name)
				responseTextBuilder.WriteString(tool.Function.Arguments)
			}
		}
	}
	return nil
}

func processTokenData(relayMode int, data string, responseTextBuilder *strings.Builder, toolCount *int) error {
	switch relayMode {
	case relayconstant.RelayModeChatCompletions:
		var streamResponse dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			return err
		}
		return ProcessStreamResponse(streamResponse, responseTextBuilder, toolCount)
	case relayconstant.RelayModeCompletions:
		var streamResponse dto.CompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			return err
		}
		processCompletionsStreamResponse(streamResponse, responseTextBuilder)
	}
	return nil
}

func processCompletionsStreamResponse(streamResponse dto.CompletionsStreamResponse, responseTextBuilder *strings.Builder) {
	for _, choice := range streamResponse.Choices {
		responseTextBuilder.WriteString(choice.Text)
	}
}

func handleLastResponse(lastStreamData string, responseId *string, createAt *int64,
	systemFingerprint *string, model *string, usage **dto.Usage,
	containStreamUsage *bool, info *relaycommon.RelayInfo,
	shouldSendLastResp *bool) error {

	var lastStreamResponse dto.ChatCompletionsStreamResponse
	if err := common.Unmarshal(common.StringToByteSlice(lastStreamData), &lastStreamResponse); err != nil {
		return err
	}

	*responseId = lastStreamResponse.Id
	*createAt = lastStreamResponse.Created
	*systemFingerprint = lastStreamResponse.GetSystemFingerprint()
	*model = lastStreamResponse.Model

	if service.ValidUsage(lastStreamResponse.Usage) {
		*containStreamUsage = true
		*usage = lastStreamResponse.Usage
		if !info.ShouldIncludeUsage {
			*shouldSendLastResp = lo.SomeBy(lastStreamResponse.Choices, func(choice dto.ChatCompletionsStreamResponseChoice) bool {
				return choice.Delta.GetContentString() != "" || choice.Delta.GetReasoningContent() != ""
			})
		}
	}

	return nil
}

func HandleFinalResponse(c *gin.Context, info *relaycommon.RelayInfo, lastStreamData string,
	responseId string, createAt int64, model string, systemFingerprint string,
	usage *dto.Usage, containStreamUsage bool) {

	switch info.RelayFormat {
	case types.RelayFormatOpenAI:
		if info.ShouldIncludeUsage && !containStreamUsage {
			response := helper.GenerateFinalUsageResponse(responseId, createAt, model, *usage)
			response.SetSystemFingerprint(systemFingerprint)
			helper.ObjectData(c, response)
		}
		helper.Done(c)

	case types.RelayFormatClaude:
		claudeInfo := info.EnsureClaudeConvertInfo()
		if usage != nil {
			claudeInfo.Usage = usage
		}

		if lastStreamData != "" {
			var streamResponse dto.ChatCompletionsStreamResponse
			if err := common.Unmarshal(common.StringToByteSlice(lastStreamData), &streamResponse); err != nil {
				common.SysLog("error unmarshalling stream response: " + err.Error())
			} else {
				// 单 chunk 流只有这一次转换机会，这里也计入已产生响应的计数。
				info.SendResponseCount++
				result, err := relayconvert.ConvertStreamResponse(c, info, types.RelayFormatClaude, &streamResponse)
				if err != nil {
					common.SysLog("error converting Claude stream response: " + err.Error())
				} else if claudeResponses, ok := result.Value.([]*dto.ClaudeResponse); !ok {
					common.SysLog(fmt.Sprintf("expected Claude stream responses, got %T", result.Value))
				} else {
					writeClaudeResponses(c, info, claudeResponses)
				}
			}
		}

		// finish chunk 不带 usage、或最后一条只是普通内容块时，转换器可能还没有发出终态；
		// 流结束后统一兜底补发 message_delta + message_stop（幂等，已发过则跳过）。
		finalizeClaudeStream(c, info)

	case types.RelayFormatGemini:
		var streamResponse dto.ChatCompletionsStreamResponse
		if err := common.Unmarshal(common.StringToByteSlice(lastStreamData), &streamResponse); err != nil {
			common.SysLog("error unmarshalling stream response: " + err.Error())
			return
		}

		// 这里处理的是 openai 最后一个流响应，其 delta 为空，有 finish_reason 字段
		// 因此相比较于 google 官方的流响应，由 openai 转换而来会多一个 parts 为空，finishReason 为 STOP 的响应
		// 而包含最后一段文本输出的响应（倒数第二个）的 finishReason 为 null
		// 暂不知是否有程序会不兼容。

		result, err := relayconvert.ConvertStreamResponse(c, info, types.RelayFormatGemini, &streamResponse)
		if err != nil {
			common.SysLog("error converting Gemini stream response: " + err.Error())
			return
		}
		geminiResponse, ok := result.Value.(*dto.GeminiChatResponse)
		if !ok {
			common.SysLog(fmt.Sprintf("expected Gemini stream response, got %T", result.Value))
			return
		}

		// openai 流响应开头的空数据
		if geminiResponse == nil {
			return
		}

		geminiResponseStr, err := common.Marshal(geminiResponse)
		if err != nil {
			common.SysLog("error marshalling gemini response: " + err.Error())
			return
		}

		// 发送最终的 Gemini 响应
		c.Render(-1, common.CustomEvent{Data: "data: " + string(geminiResponseStr)})
		_ = helper.FlushWriter(c)
	}
}

func sendResponsesStreamData(c *gin.Context, streamResponse dto.ResponsesStreamResponse, data string) {
	if data == "" {
		return
	}
	_ = helper.ResponseChunkData(c, streamResponse, data)
}

package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func sendStreamData(c *gin.Context, info *relaycommon.RelayInfo, data string, forceFormat bool, thinkToContent bool) error {
	if data == "" {
		return nil
	}

	if !forceFormat && !thinkToContent {
		return helper.StringData(c, data)
	}

	var lastStreamResponse dto.ChatCompletionsStreamResponse
	if err := common.UnmarshalJsonStr(data, &lastStreamResponse); err != nil {
		return err
	}

	if !thinkToContent {
		return helper.ObjectData(c, lastStreamResponse)
	}

	hasThinkingContent := false
	hasContent := false
	var thinkingContent strings.Builder
	for _, choice := range lastStreamResponse.Choices {
		if len(choice.Delta.GetReasoningContent()) > 0 {
			hasThinkingContent = true
			thinkingContent.WriteString(choice.Delta.GetReasoningContent())
		}
		if len(choice.Delta.GetContentString()) > 0 {
			hasContent = true
		}
	}

	// Handle think to content conversion
	if info.ThinkingContentInfo.IsFirstThinkingContent {
		if hasThinkingContent {
			response := lastStreamResponse.Copy()
			for i := range response.Choices {
				// send `think` tag with thinking content
				response.Choices[i].Delta.SetContentString("<think>\n" + thinkingContent.String())
				response.Choices[i].Delta.ReasoningContent = nil
				response.Choices[i].Delta.Reasoning = nil
			}
			info.ThinkingContentInfo.IsFirstThinkingContent = false
			info.ThinkingContentInfo.HasSentThinkingContent = true
			return helper.ObjectData(c, response)
		}
	}

	if lastStreamResponse.Choices == nil || len(lastStreamResponse.Choices) == 0 {
		return helper.ObjectData(c, lastStreamResponse)
	}

	// Process each choice
	for i, choice := range lastStreamResponse.Choices {
		// Handle transition from thinking to content
		// only send `</think>` tag when previous thinking content has been sent
		if hasContent && !info.ThinkingContentInfo.SendLastThinkingContent && info.ThinkingContentInfo.HasSentThinkingContent {
			response := lastStreamResponse.Copy()
			for j := range response.Choices {
				response.Choices[j].Delta.SetContentString("\n</think>\n")
				response.Choices[j].Delta.ReasoningContent = nil
				response.Choices[j].Delta.Reasoning = nil
			}
			info.ThinkingContentInfo.SendLastThinkingContent = true
			helper.ObjectData(c, response)
		}

		// Convert reasoning content to regular content if any
		if len(choice.Delta.GetReasoningContent()) > 0 {
			lastStreamResponse.Choices[i].Delta.SetContentString(choice.Delta.GetReasoningContent())
			lastStreamResponse.Choices[i].Delta.ReasoningContent = nil
			lastStreamResponse.Choices[i].Delta.Reasoning = nil
		} else if !hasThinkingContent && !hasContent {
			// flush thinking content
			lastStreamResponse.Choices[i].Delta.ReasoningContent = nil
			lastStreamResponse.Choices[i].Delta.Reasoning = nil
		}
	}

	return helper.ObjectData(c, lastStreamResponse)
}

func OaiStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	model := info.UpstreamModelName
	var responseId string
	var createAt int64 = 0
	var systemFingerprint string
	var containStreamUsage bool
	var responseTextBuilder strings.Builder
	var toolCount int
	var usage = &dto.Usage{}
	var lastStreamData string
	var secondLastStreamData string // 存储倒数第二个stream data，用于音频模型
	seenStreamToolCalls := make(map[string]struct{})
	var streamFunctionCallNames []string

	// 检查是否为音频模型
	isAudioModel := strings.Contains(strings.ToLower(model), "audio")

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if lastStreamData != "" {
			if err := HandleStreamFormat(c, info, lastStreamData, info.ChannelSetting.ForceFormat, info.ChannelSetting.ThinkingToContent); err != nil {
				common.SysLog("error handling stream format: " + err.Error())
				sr.Error(err)
			}
		}
		if len(data) > 0 {
			// 对音频模型，保存倒数第二个stream data
			if isAudioModel && lastStreamData != "" {
				secondLastStreamData = lastStreamData
			}

			lastStreamData = data
			collectStreamFunctionCallNames(data, seenStreamToolCalls, &streamFunctionCallNames)
			if err := processTokenData(info.RelayMode, data, &responseTextBuilder, &toolCount); err != nil {
				logger.LogError(c, "error processing stream token data: "+err.Error())
				sr.Error(err)
			}
		}
	})

	// 对音频模型，从倒数第二个stream data中提取usage信息
	if isAudioModel && secondLastStreamData != "" {
		var streamResp struct {
			Usage *dto.Usage `json:"usage"`
		}
		err := common.Unmarshal([]byte(secondLastStreamData), &streamResp)
		if err == nil && streamResp.Usage != nil && service.ValidUsage(streamResp.Usage) {
			usage = streamResp.Usage
			containStreamUsage = true

			if common.DebugEnabled {
				logger.LogDebug(c, "Audio model usage extracted from second last SSE: PromptTokens=%d, CompletionTokens=%d, TotalTokens=%d, InputTokens=%d, OutputTokens=%d",
					usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens,
					usage.InputTokens, usage.OutputTokens)
			}
		}
	}

	// 处理最后的响应
	shouldSendLastResp := true
	if err := handleLastResponse(lastStreamData, &responseId, &createAt, &systemFingerprint, &model, &usage,
		&containStreamUsage, info, &shouldSendLastResp); err != nil {
		logger.LogError(c, fmt.Sprintf("error handling last response: %s, lastStreamData: [%s]", err.Error(), lastStreamData))
	}

	if info.RelayFormat == types.RelayFormatOpenAI {
		if shouldSendLastResp {
			_ = sendStreamData(c, info, lastStreamData, info.ChannelSetting.ForceFormat, info.ChannelSetting.ThinkingToContent)
		}
	}

	if !containStreamUsage {
		usage = service.ResponseText2Usage(c, responseTextBuilder.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		usage.CompletionTokens += toolCount * 7
	}

	applyUsagePostProcessing(info, usage, common.StringToByteSlice(lastStreamData))

	for _, name := range streamFunctionCallNames {
		info.CountBillableToolCall(dto.BuildInCallFunctionCall, name)
	}

	// 上游中断/超时/panic 时不能补成功终态（message_stop 或 [DONE]），
	// 否则客户端会把截断的流当成正常完成。
	if status := info.StreamStatus; isAbnormalStreamEnd(status) {
		if c.Writer == nil || !c.Writer.Written() {
			// 还没有写出任何内容，返回可重试错误，让 controller 换渠道重试。
			return nil, streamEndAPIError(status)
		}
		if info.RelayFormat == types.RelayFormatClaude {
			writeClaudeStreamError(c, streamEndErrorMessage(status))
			// 标记终态已处理，避免其它收尾路径再补成功事件。
			info.EnsureClaudeConvertInfo().Done = true
		}
		return usage, nil
	}

	HandleFinalResponse(c, info, lastStreamData, responseId, createAt, model, systemFingerprint, usage, containStreamUsage)

	return usage, nil
}

// isAbnormalStreamEnd 判断流是否因为中断而不是正常结束。
// EOF/[DONE]/HandlerStop 视为正常结束；ScannerErr/Timeout/Panic/PingFail 视为异常。
func isAbnormalStreamEnd(status *relaycommon.StreamStatus) bool {
	if status == nil {
		return false
	}
	switch status.EndReason {
	case relaycommon.StreamEndReasonScannerErr,
		relaycommon.StreamEndReasonTimeout,
		relaycommon.StreamEndReasonPanic,
		relaycommon.StreamEndReasonPingFail:
		return true
	}
	return false
}

// streamEndErrorMessage 构造给客户端的流中断错误文案。
func streamEndErrorMessage(status *relaycommon.StreamStatus) string {
	message := "upstream stream interrupted before completion"
	if status != nil {
		switch status.EndReason {
		case relaycommon.StreamEndReasonTimeout:
			message = "upstream stream timed out"
		case relaycommon.StreamEndReasonPanic:
			message = "upstream stream handler panicked"
		case relaycommon.StreamEndReasonPingFail:
			message = "upstream stream connection lost"
		case relaycommon.StreamEndReasonScannerErr:
			message = "upstream stream scan failed"
		}
		if status.EndError != nil {
			message += ": " + status.EndError.Error()
		}
	}
	return message
}

// streamEndAPIError 把异常结束转换为可重试的上游错误。
func streamEndAPIError(status *relaycommon.StreamStatus) *types.NewAPIError {
	err := fmt.Errorf("%s", streamEndErrorMessage(status))
	statusCode := http.StatusBadGateway
	errorCode := types.ErrorCodeBadResponse
	if status != nil && status.EndReason == relaycommon.StreamEndReasonTimeout {
		statusCode = http.StatusGatewayTimeout
		errorCode = types.ErrorCodeChannelResponseTimeExceeded
	}
	// 不带 ErrOptionWithSkipRetry：控制器可以重试或换渠道。
	return types.NewOpenAIError(err, errorCode, statusCode)
}

// writeClaudeStreamError 按 Anthropic SSE 格式下发流内错误事件。
func writeClaudeStreamError(c *gin.Context, message string) {
	if strings.TrimSpace(message) == "" {
		message = "upstream stream error"
	}
	_ = helper.ClaudeData(c, dto.ClaudeResponse{
		Type:  "error",
		Error: &types.ClaudeError{Type: "stream_error", Message: message},
	})
}

func collectStreamFunctionCallNames(data string, seen map[string]struct{}, names *[]string) {
	var streamResponse dto.ChatCompletionsStreamResponse
	if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
		return
	}
	for _, choice := range streamResponse.Choices {
		for i, tc := range choice.Delta.ToolCalls {
			name := tc.Function.Name
			if name == "" {
				continue
			}
			toolIdx := i
			if tc.Index != nil {
				toolIdx = *tc.Index
			}
			key := fmt.Sprintf("%d-%d", choice.Index, toolIdx)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			*names = append(*names, name)
		}
	}
}

func OpenaiHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	var simpleResponse dto.OpenAITextResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	logger.LogDebug(c, "upstream response body: %s", responseBody)

	// 部分 OpenAI 兼容上游无视 stream:false 直接返回 SSE；客户端没要流式时必须先聚合为
	// 普通 chat.completion JSON，否则会把 SSE 帧原样发给非流式客户端。
	if looksLikeSSE(responseBody) {
		aggregated, aggErr := aggregateChatSSEToJSON(responseBody)
		if aggErr != nil {
			return nil, types.NewOpenAIError(aggErr, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
		}
		responseBody = aggregated
		if resp.Header != nil {
			// 聚合后响应体已经是 JSON，避免把上游的 text/event-stream 透传给客户端。
			resp.Header.Set("Content-Type", "application/json")
			resp.Header.Del("Transfer-Encoding")
		}
	}

	// 兼容两类上游包层：OpenRouter enterprise 的 {"success":true,"data":{...}}
	// 以及通用的 {"data":{"choices":[...]}}。仅在顶层没有 choices 数组时解包。
	unwrappedBody, unwrapErr := unwrapChatCompletionBody(c, responseBody)
	if unwrapErr != nil {
		return nil, unwrapErr
	}
	responseBody = unwrappedBody

	// Unmarshal to simpleResponse
	err = common.Unmarshal(responseBody, &simpleResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if oaiError := simpleResponse.GetOpenAIError(); oaiError != nil && (oaiError.Type != "" || oaiError.Message != "") {
		// 上游可能用 HTTP 200 携带错误体，不能沿用 2xx 状态码：否则 controller 会当成功，
		// 既不重试也不报警。这里按 error.type 映射到 4xx/502。
		return nil, types.WithOpenAIError(*oaiError, chatCompletionErrorStatusCode(resp.StatusCode, oaiError))
	}

	for _, choice := range simpleResponse.Choices {
		if choice.FinishReason == constant.FinishReasonContentFilter {
			common.SetContextKey(c, constant.ContextKeyAdminRejectReason, "openai_finish_reason=content_filter")
			break
		}
	}

	for _, choice := range simpleResponse.Choices {
		for _, tc := range choice.Message.ParseToolCalls() {
			info.CountBillableToolCall(dto.BuildInCallFunctionCall, tc.Function.Name)
		}
	}

	forceFormat := false
	if info.ChannelSetting.ForceFormat {
		forceFormat = true
	}

	usageModified := false
	if simpleResponse.Usage.PromptTokens == 0 {
		completionTokens := simpleResponse.Usage.CompletionTokens
		if completionTokens == 0 {
			for _, choice := range simpleResponse.Choices {
				ctkm := service.CountTextToken(choice.Message.StringContent()+choice.Message.GetReasoningContent(), info.UpstreamModelName)
				completionTokens += ctkm
			}
		}
		simpleResponse.Usage = dto.Usage{
			PromptTokens:     info.GetEstimatePromptTokens(),
			CompletionTokens: completionTokens,
			TotalTokens:      info.GetEstimatePromptTokens() + completionTokens,
		}
		usageModified = true
	}

	applyUsagePostProcessing(info, &simpleResponse.Usage, responseBody)

	switch info.RelayFormat {
	case types.RelayFormatOpenAI:
		if usageModified {
			var bodyMap map[string]interface{}
			err = common.Unmarshal(responseBody, &bodyMap)
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
			bodyMap["usage"] = simpleResponse.Usage
			responseBody, _ = common.Marshal(bodyMap)
		}
		if forceFormat {
			responseBody, err = common.Marshal(simpleResponse)
			if err != nil {
				return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
			}
		} else {
			break
		}
	case types.RelayFormatClaude:
		convertResult, err := relayconvert.ConvertResponse(c, info, types.RelayFormatClaude, &simpleResponse)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		claudeRespStr, err := common.Marshal(convertResult.Value)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		responseBody = claudeRespStr
	case types.RelayFormatGemini:
		convertResult, err := relayconvert.ConvertResponse(c, info, types.RelayFormatGemini, &simpleResponse)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		geminiRespStr, err := common.Marshal(convertResult.Value)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		responseBody = geminiRespStr
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)

	return &simpleResponse.Usage, nil
}

// chatCompletionEnvelope 描述上游可能加在 chat.completion 外层的包层。
type chatCompletionEnvelope struct {
	Success *bool           `json:"success"`
	Data    json.RawMessage `json:"data"`
	Choices json.RawMessage `json:"choices"`
}

// isJSONArray 判断原始 JSON 是否为数组（含空数组）。
func isJSONArray(raw json.RawMessage) bool {
	return strings.HasPrefix(strings.TrimSpace(string(raw)), "[")
}

// unwrapChatCompletionBody 解包上游包层：
//   - 兼容 OpenRouter enterprise 的 {"success":true,"data":{...}}；
//   - 泛化为任意 {"data":{"choices":[...]}} 且顶层没有 choices 数组的响应；
//   - 保留 success=false 的报错分支。
func unwrapChatCompletionBody(c *gin.Context, responseBody []byte) ([]byte, *types.NewAPIError) {
	var envelope chatCompletionEnvelope
	if err := common.Unmarshal(responseBody, &envelope); err != nil {
		// 不是 JSON（例如 SSE 或纯文本），交给后续解析处理。
		return responseBody, nil
	}
	if envelope.Success != nil && !*envelope.Success {
		logger.LogError(c, fmt.Sprintf("upstream response success=false, data: %s", common.LocalLogPreview(string(envelope.Data))))
		return nil, types.NewOpenAIError(fmt.Errorf("upstream response success=false"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if isJSONArray(envelope.Choices) || len(envelope.Data) == 0 {
		return responseBody, nil
	}

	var dataPayload struct {
		Choices json.RawMessage `json:"choices"`
	}
	if err := common.Unmarshal(envelope.Data, &dataPayload); err != nil || !isJSONArray(dataPayload.Choices) {
		return responseBody, nil
	}
	return envelope.Data, nil
}

// chatCompletionErrorStatusCode 计算 HTTP 200 响应体里 error 应使用的状态码。
// 上游明确返回了非 2xx 时保持原状态码，否则按 error.type 映射，避免 2xx 被当成成功。
func chatCompletionErrorStatusCode(statusCode int, oaiError *types.OpenAIError) int {
	if statusCode < 200 || statusCode >= 300 {
		return statusCode
	}
	if oaiError == nil {
		return http.StatusBadGateway
	}
	errorType := strings.ToLower(oaiError.Type)
	switch {
	case strings.Contains(errorType, "invalid_request"):
		return http.StatusBadRequest
	case strings.Contains(errorType, "authentication"):
		return http.StatusUnauthorized
	case strings.Contains(errorType, "permission"), strings.Contains(errorType, "insufficient_quota"):
		return http.StatusForbidden
	case strings.Contains(errorType, "not_found"):
		return http.StatusNotFound
	case strings.Contains(errorType, "rate_limit"), strings.Contains(errorType, "too_many"):
		return http.StatusTooManyRequests
	default:
		return http.StatusBadGateway
	}
}

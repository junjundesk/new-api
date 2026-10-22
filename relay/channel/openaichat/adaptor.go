/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package openaichat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// Adaptor 固定以 OpenAI Chat Completions 格式请求上游。
// 客户端请求格式（chat / responses / claude）与线上格式一致时原样透传，
// 不一致时先转换为 chat 再发送；响应按客户端格式回转。
type Adaptor struct {
	openai.Adaptor
}

// GetRequestURL responses 客户端的请求已转换为 chat，端点必须改写为
// /v1/chat/completions，而不是客户端原始的 /v1/responses。
func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode == relayconstant.RelayModeResponses {
		return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, "/v1/chat/completions", info.ChannelType), nil
	}
	return a.Adaptor.GetRequestURL(info)
}

// ConvertOpenAIResponsesRequest responses 客户端请求 → chat 上游请求。
func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// 上游固定 chat completions，没有服务端会话状态：忽略有状态字段并按当前
	// input 继续，而不是直接 400；被忽略的内容记录到日志便于排查。
	if dropped := ignoreStatefulResponsesFields(&request); len(dropped) > 0 {
		logger.LogWarn(c, fmt.Sprintf("fixed chat channel %d ignores stateful responses fields: %s", info.ChannelId, strings.Join(dropped, ", ")))
	}
	if droppedTools := droppedBuiltInToolTypes(request.Tools); len(droppedTools) > 0 {
		logger.LogWarn(c, fmt.Sprintf("fixed chat channel %d drops built-in tools unsupported by chat completions: %s", info.ChannelId, strings.Join(droppedTools, ", ")))
	}

	result, err := service.ConvertRequest(c, info, types.RelayFormatOpenAI, &request)
	if err != nil {
		return nil, err
	}
	chatRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", result.Value)
	}
	if len(chatRequest.Messages) == 0 {
		return nil, fmt.Errorf("responses request has no input after ignoring stateful fields")
	}
	return a.Adaptor.ConvertOpenAIRequest(c, info, chatRequest)
}

// ignoreStatefulResponsesFields 清空固定 chat 渠道无法表达的有状态字段，并返回被丢弃的字段名。
func ignoreStatefulResponsesFields(request *dto.OpenAIResponsesRequest) []string {
	if request == nil {
		return nil
	}
	dropped := make([]string, 0, 4)
	if hasRawJSONValue(request.Conversation) {
		dropped = append(dropped, "conversation")
		request.Conversation = nil
	}
	if request.PreviousResponseID != "" {
		dropped = append(dropped, "previous_response_id")
		request.PreviousResponseID = ""
	}
	if hasRawJSONValue(request.Prompt) {
		dropped = append(dropped, "prompt")
		request.Prompt = nil
	}
	if hasRawJSONValue(request.ContextManagement) {
		dropped = append(dropped, "context_management")
		request.ContextManagement = nil
	}
	return dropped
}

// hasRawJSONValue 判断 RawMessage 字段是否真的有值（显式 null 视为没有）。
func hasRawJSONValue(raw json.RawMessage) bool {
	return len(raw) > 0 && common.GetJsonType(raw) != "null"
}

// droppedBuiltInToolTypes 返回请求里 chat completions 无法表达、会被转换器丢弃的工具类型。
func droppedBuiltInToolTypes(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	dropped := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		if toolType == "function" || toolType == dto.CustomType {
			continue
		}
		dropped = append(dropped, toolType)
	}
	return dropped
}

// DoResponse 上游永远返回 chat 格式：responses 客户端需要把 chat 响应
// 回转成 responses 格式；chat / claude 客户端沿用 openai adaptor 的处理。
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == relayconstant.RelayModeResponses {
		if info.IsStream {
			return openai.OaiChatToResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiChatToResponsesHandler(c, info, resp)
	}
	return a.Adaptor.DoResponse(c, resp, info)
}

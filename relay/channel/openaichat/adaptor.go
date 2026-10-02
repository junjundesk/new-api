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
	"fmt"
	"net/http"

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
	result, err := service.ConvertRequest(c, info, types.RelayFormatOpenAI, &request)
	if err != nil {
		return nil, err
	}
	chatRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", result.Value)
	}
	return a.Adaptor.ConvertOpenAIRequest(c, info, chatRequest)
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

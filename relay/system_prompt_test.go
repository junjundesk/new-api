/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package relay

import (
	"net/http/httptest"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newSystemPromptTestContext() *gin.Context {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	return context
}

func TestApplySystemPromptIfNeededUsesChannelPromptAsDefault(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "system", Content: "user rules"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{SystemPrompt: "channel rules"},
		},
	}

	applySystemPromptIfNeeded(newSystemPromptTestContext(), info, request)

	require.Equal(t, "channel rules", request.Messages[0].StringContent())
}

func TestApplySystemPromptIfNeededConcatenatesUserPromptWhenEnabled(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "system", Content: "user rules"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				SystemPrompt:         "channel rules",
				SystemPromptOverride: true,
			},
		},
	}

	applySystemPromptIfNeeded(newSystemPromptTestContext(), info, request)

	require.Equal(t, "channel rules\nuser rules", request.Messages[0].StringContent())
}

func TestApplySystemPromptToResponsesRequestUsesSamePriority(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{}
	encoded, err := rootcommon.Marshal("user rules")
	require.NoError(t, err)
	request.Instructions = encoded
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{SystemPrompt: "channel rules"},
		},
	}

	err = applySystemPromptToResponsesRequest(newSystemPromptTestContext(), info, request)
	require.NoError(t, err)

	var instructions string
	err = rootcommon.Unmarshal(request.Instructions, &instructions)
	require.NoError(t, err)
	require.Equal(t, "channel rules", instructions)
}

func TestApplySystemPromptToResponsesRequestConcatenatesWhenEnabled(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{}
	encoded, err := rootcommon.Marshal("user rules")
	require.NoError(t, err)
	request.Instructions = encoded
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				SystemPrompt:         "channel rules",
				SystemPromptOverride: true,
			},
		},
	}

	err = applySystemPromptToResponsesRequest(newSystemPromptTestContext(), info, request)
	require.NoError(t, err)

	var instructions string
	err = rootcommon.Unmarshal(request.Instructions, &instructions)
	require.NoError(t, err)
	require.Equal(t, "channel rules\nuser rules", instructions)
}

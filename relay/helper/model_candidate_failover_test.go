package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	kitcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModelCandidateFailoverRetriesSameChannelWithNextCandidate walks the same
// path the relay loop takes on a candidate failure: the channel stays selected
// (pinned), its context is set up again, and the mapping resolves the next
// upstream model for the new attempt.
func TestModelCandidateFailoverRetriesSameChannelWithNextCandidate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	candidates := []string{"candidate-a", "candidate-b", "candidate-c"}
	channelName := "mapped-channel"
	mappingRaw, err := kitcommon.Marshal(map[string]any{"alias-model": candidates})
	require.NoError(t, err)
	mapping := string(mappingRaw)

	priority := int64(0)
	weight := uint(100)
	channel := &model.Channel{
		Id:           8201,
		Type:         constant.ChannelTypeOpenAI,
		Key:          "test-key",
		Status:       kitcommon.ChannelStatusEnabled,
		Name:         channelName,
		Models:       "alias-model",
		Group:        "default",
		Priority:     &priority,
		Weight:       &weight,
		ModelMapping: &mapping,
	}

	info := &relaycommon.RelayInfo{
		OriginModelName: "alias-model",
		Request:         &dto.GeneralOpenAIRequest{Model: "alias-model"},
	}

	// 首次尝试：选中渠道，映射取第一个候选（与 handler 一致：先 InitChannelMeta 再映射）
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName))
	info.InitChannelMeta(c)
	request := &dto.GeneralOpenAIRequest{Model: info.OriginModelName}
	require.NoError(t, ModelMappedHelper(c, info, request))
	assert.Equal(t, "candidate-a", info.UpstreamModelName)

	// 上游报告 candidate-a 不可用：推进候选并原地重试
	apiErr := newUpstreamModelNotFoundError("candidate-a")
	require.True(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))
	require.True(t, AdvanceModelCandidate(info))

	// 第二次尝试：同一个渠道重新建立上下文后应使用下一个候选
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName))
	info.InitChannelMeta(c)
	request = &dto.GeneralOpenAIRequest{Model: info.OriginModelName}
	require.NoError(t, ModelMappedHelper(c, info, request))
	assert.Equal(t, "candidate-b", info.UpstreamModelName)
	assert.Equal(t, "candidate-b", request.Model)

	// candidate-b 失败后仍有最后一个候选可用
	apiErr = newUpstreamModelNotFoundError("candidate-b")
	require.True(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))
	require.True(t, AdvanceModelCandidate(info))

	// 最后一个候选也用尽后不再切换，交回渠道级重试
	require.False(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))
	require.False(t, AdvanceModelCandidate(info))

	require.Nil(t, middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName))
	info.InitChannelMeta(c)
	request = &dto.GeneralOpenAIRequest{Model: info.OriginModelName}
	require.NoError(t, ModelMappedHelper(c, info, request))
	assert.Equal(t, "candidate-c", info.UpstreamModelName)
}

// TestModelCandidateFailoverResetsOnAnotherChannel verifies that a channel-level
// retry landing on a different channel restarts from the first candidate.
func TestModelCandidateFailoverResetsOnAnotherChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	mappingRaw, err := kitcommon.Marshal(map[string]any{"alias-model": []string{"candidate-a", "candidate-b"}})
	require.NoError(t, err)
	mapping := string(mappingRaw)

	first := &model.Channel{Id: 8301, Type: constant.ChannelTypeOpenAI, Name: "first", ModelMapping: &mapping}
	second := &model.Channel{Id: 8302, Type: constant.ChannelTypeOpenAI, Name: "second", ModelMapping: &mapping}

	info := &relaycommon.RelayInfo{
		OriginModelName: "alias-model",
		Request:         &dto.GeneralOpenAIRequest{Model: "alias-model"},
	}

	require.Nil(t, middleware.SetupContextForSelectedChannel(c, first, info.OriginModelName))
	info.InitChannelMeta(c)
	require.NoError(t, ModelMappedHelper(c, info, &dto.GeneralOpenAIRequest{Model: info.OriginModelName}))
	require.True(t, AdvanceModelCandidate(info))
	assert.Equal(t, 1, info.ModelCandidate.Index)

	require.Nil(t, middleware.SetupContextForSelectedChannel(c, second, info.OriginModelName))
	info.InitChannelMeta(c)
	require.NoError(t, ModelMappedHelper(c, info, &dto.GeneralOpenAIRequest{Model: info.OriginModelName}))
	assert.Equal(t, 0, info.ModelCandidate.Index)
	assert.Equal(t, "candidate-a", info.UpstreamModelName)
}

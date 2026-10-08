package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMappingTestContext(t *testing.T, mapping string) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if mapping != "" {
		c.Set("model_mapping", mapping)
	}
	return c
}

func newMappingTestRequest(modelName string) *dto.GeneralOpenAIRequest {
	return &dto.GeneralOpenAIRequest{Model: modelName}
}

func TestModelMappedHelperLegacyChaining(t *testing.T) {
	c := newMappingTestContext(t, `{"client-model":"middle-model","middle-model":"upstream-model"}`)
	request := newMappingTestRequest("client-model")
	info := &common.RelayInfo{
		ChannelMeta:     &common.ChannelMeta{UpstreamModelName: "client-model"},
		OriginModelName: "client-model",
	}

	require.NoError(t, ModelMappedHelper(c, info, request))
	assert.True(t, info.IsModelMapped)
	assert.Equal(t, "upstream-model", info.UpstreamModelName)
	assert.Equal(t, "upstream-model", request.Model)
}

func TestModelMappedHelperSelfMappingIsNotMapped(t *testing.T) {
	c := newMappingTestContext(t, `{"client-model":"client-model"}`)
	request := newMappingTestRequest("client-model")
	info := &common.RelayInfo{
		ChannelMeta:     &common.ChannelMeta{UpstreamModelName: "client-model"},
		OriginModelName: "client-model",
	}

	require.NoError(t, ModelMappedHelper(c, info, request))
	assert.False(t, info.IsModelMapped)
	assert.Equal(t, "client-model", request.Model)
}

func TestModelMappedHelperDetectsCycle(t *testing.T) {
	c := newMappingTestContext(t, `{"a":"b","b":"a"}`)
	request := newMappingTestRequest("a")
	info := &common.RelayInfo{
		ChannelMeta:     &common.ChannelMeta{UpstreamModelName: "a"},
		OriginModelName: "a",
	}

	err := ModelMappedHelper(c, info, request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model_mapping_contains_cycle")
}

func TestModelMappedHelperUsesFirstCandidate(t *testing.T) {
	c := newMappingTestContext(t, `{"claude-3-5-sonnet":["claude-3-5-sonnet-20241022","claude-3-5-sonnet-latest"]}`)
	request := newMappingTestRequest("claude-3-5-sonnet")
	info := &common.RelayInfo{
		ChannelMeta:     &common.ChannelMeta{ChannelId: 7, UpstreamModelName: "claude-3-5-sonnet"},
		OriginModelName: "claude-3-5-sonnet",
	}

	require.NoError(t, ModelMappedHelper(c, info, request))
	assert.True(t, info.IsModelMapped)
	assert.Equal(t, "claude-3-5-sonnet-20241022", info.UpstreamModelName)
	assert.Equal(t, "claude-3-5-sonnet-20241022", request.Model)
	require.NotNil(t, info.ModelCandidate)
	assert.Equal(t, 0, info.ModelCandidate.Index)
}

func TestModelMappedHelperCandidateSelectionAndAdvance(t *testing.T) {
	c := newMappingTestContext(t, `{"alias":["candidate-a","candidate-b","candidate-c"]}`)
	info := &common.RelayInfo{
		ChannelMeta:     &common.ChannelMeta{ChannelId: 7, UpstreamModelName: "alias"},
		OriginModelName: "alias",
	}

	require.NoError(t, ModelMappedHelper(c, info, newMappingTestRequest("alias")))
	assert.Equal(t, "candidate-a", CurrentModelCandidate(info))
	require.True(t, HasModelCandidates(info))
	assert.False(t, ModelCandidateExhausted(info))

	require.True(t, AdvanceModelCandidate(info))
	assert.Equal(t, "candidate-b", CurrentModelCandidate(info))
	require.NoError(t, ModelMappedHelper(c, info, newMappingTestRequest("alias")))
	assert.Equal(t, "candidate-b", info.UpstreamModelName)

	require.True(t, AdvanceModelCandidate(info))
	assert.Equal(t, "candidate-c", CurrentModelCandidate(info))
	assert.True(t, ModelCandidateExhausted(info))
	assert.False(t, AdvanceModelCandidate(info))
}

func TestModelMappedHelperResetsCandidatesWhenChannelChanges(t *testing.T) {
	c := newMappingTestContext(t, `{"alias":["candidate-a","candidate-b"]}`)
	info := &common.RelayInfo{
		ChannelMeta:     &common.ChannelMeta{ChannelId: 7, UpstreamModelName: "alias"},
		OriginModelName: "alias",
	}

	require.NoError(t, ModelMappedHelper(c, info, newMappingTestRequest("alias")))
	require.True(t, AdvanceModelCandidate(info))
	require.Equal(t, 1, info.ModelCandidate.Index)

	// 换到另一个渠道后应从第一个候选重新开始
	info.ChannelMeta = &common.ChannelMeta{ChannelId: 8, UpstreamModelName: "alias"}
	require.NoError(t, ModelMappedHelper(c, info, newMappingTestRequest("alias")))
	assert.Equal(t, 0, info.ModelCandidate.Index)
	assert.Equal(t, "candidate-a", info.UpstreamModelName)
}

func TestModelMappedHelperSingleCandidateDoesNotEnableFailover(t *testing.T) {
	c := newMappingTestContext(t, `{"alias":"upstream-model"}`)
	info := &common.RelayInfo{
		ChannelMeta:     &common.ChannelMeta{ChannelId: 7, UpstreamModelName: "alias"},
		OriginModelName: "alias",
	}

	require.NoError(t, ModelMappedHelper(c, info, newMappingTestRequest("alias")))
	assert.False(t, HasModelCandidates(info))
	assert.False(t, AdvanceModelCandidate(info))
	assert.Equal(t, "", CurrentModelCandidate(info))
}

func TestModelMappedHelperReportsMalformedPayload(t *testing.T) {
	c := newMappingTestContext(t, `{"alias":`)
	info := &common.RelayInfo{
		ChannelMeta:     &common.ChannelMeta{UpstreamModelName: "alias"},
		OriginModelName: "alias",
	}

	err := ModelMappedHelper(c, info, newMappingTestRequest("alias"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal_model_mapping_failed")
}

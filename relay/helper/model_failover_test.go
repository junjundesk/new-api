package helper

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// candidateRelayInfo builds a RelayInfo whose channel mapping already resolved
// a two-candidate model.
func candidateRelayInfo(t *testing.T) *common.RelayInfo {
	t.Helper()
	return &common.RelayInfo{
		OriginModelName: "alias",
		ChannelMeta: &common.ChannelMeta{
			ChannelId:         7,
			UpstreamModelName: "candidate-a",
		},
		ModelCandidate: &common.ModelCandidateState{
			ChannelId:  7,
			Index:      0,
			Candidates: []string{"candidate-a", "candidate-b"},
		},
	}
}

func newFailoverTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}

func upstreamError(statusCode int, errorCode types.ErrorCode, message string) *types.NewAPIError {
	return types.NewErrorWithStatusCode(errors.New(message), errorCode, statusCode)
}

// newUpstreamModelNotFoundError builds the upstream response the candidate
// failover is designed for: the model itself is reported as unavailable.
func newUpstreamModelNotFoundError(modelName string) *types.NewAPIError {
	return upstreamError(
		http.StatusNotFound,
		types.ErrorCodeBadResponseStatusCode,
		fmt.Sprintf("The model `%s` does not exist", modelName),
	)
}

func TestShouldFailoverToNextModelCandidate(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorCode  types.ErrorCode
		message    string
		expected   bool
	}{
		{
			name:       "model not found on 404",
			statusCode: http.StatusNotFound,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "The model `candidate-a` does not exist",
			expected:   true,
		},
		{
			name:       "plain 404 without model wording",
			statusCode: http.StatusNotFound,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "not found",
			expected:   true,
		},
		{
			name:       "rate limited",
			statusCode: http.StatusTooManyRequests,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "rate limit exceeded",
			expected:   true,
		},
		{
			name:       "upstream server error",
			statusCode: http.StatusInternalServerError,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "internal error",
			expected:   true,
		},
		{
			name:       "invalid model on 400",
			statusCode: http.StatusBadRequest,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "invalid model: candidate-a",
			expected:   true,
		},
		{
			name:       "model unsupported on 422",
			statusCode: http.StatusUnprocessableEntity,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "this model is unsupported for this endpoint",
			expected:   true,
		},
		{
			name:       "malformed request on 400 without model wording",
			statusCode: http.StatusBadRequest,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "messages is required",
			expected:   false,
		},
		{
			name:       "context length exceeded is not a model availability problem",
			statusCode: http.StatusBadRequest,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "model context length exceeded",
			expected:   false,
		},
		{
			name:       "gateway timeout is in always skip retry",
			statusCode: http.StatusGatewayTimeout,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "timeout",
			expected:   false,
		},
		{
			name:       "unauthorized disables the channel instead of failing over",
			statusCode: http.StatusUnauthorized,
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			message:    "invalid api key",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newFailoverTestContext()
			info := candidateRelayInfo(t)
			apiErr := upstreamError(tt.statusCode, tt.errorCode, tt.message)

			assert.Equal(t, tt.expected, ShouldFailoverToNextModelCandidate(c, info, apiErr))
		})
	}
}

func TestShouldFailoverSkipsInternalChannelAndSkipRetryErrors(t *testing.T) {
	c := newFailoverTestContext()

	t.Run("channel error", func(t *testing.T) {
		info := candidateRelayInfo(t)
		apiErr := types.NewError(errors.New("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
		assert.False(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))
	})

	t.Run("skip retry marked error", func(t *testing.T) {
		info := candidateRelayInfo(t)
		apiErr := types.NewErrorWithStatusCode(
			fmt.Errorf("model not found"),
			types.ErrorCodeBadResponseStatusCode,
			http.StatusNotFound,
			types.ErrOptionWithSkipRetry(),
		)
		assert.False(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))
	})
}

func TestShouldFailoverRequiresRemainingCandidates(t *testing.T) {
	c := newFailoverTestContext()
	apiErr := upstreamError(http.StatusNotFound, types.ErrorCodeBadResponseStatusCode, "model not found")

	t.Run("single candidate mapping never fails over", func(t *testing.T) {
		info := &common.RelayInfo{
			OriginModelName: "alias",
			ChannelMeta:     &common.ChannelMeta{ChannelId: 7, UpstreamModelName: "upstream"},
			ModelCandidate: &common.ModelCandidateState{
				ChannelId:  7,
				Candidates: []string{"upstream"},
			},
		}
		assert.False(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))
	})

	t.Run("no mapping at all never fails over", func(t *testing.T) {
		info := &common.RelayInfo{
			OriginModelName: "alias",
			ChannelMeta:     &common.ChannelMeta{ChannelId: 7, UpstreamModelName: "alias"},
		}
		assert.False(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))
	})

	t.Run("exhausted candidate list never fails over", func(t *testing.T) {
		info := candidateRelayInfo(t)
		info.ModelCandidate.Index = 1
		assert.False(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))
	})
}

func TestShouldFailoverRequiresUnwrittenResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := candidateRelayInfo(t)
	apiErr := upstreamError(http.StatusNotFound, types.ErrorCodeBadResponseStatusCode, "model not found")

	require.True(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))

	// 已经向客户端写出内容后不允许再换模型重发
	c.Writer.WriteHeader(http.StatusOK)
	_, err := c.Writer.WriteString("partial")
	require.NoError(t, err)
	assert.False(t, ShouldFailoverToNextModelCandidate(c, info, apiErr))
}

func TestShouldFailoverIgnoresNilArguments(t *testing.T) {
	c := newFailoverTestContext()
	info := candidateRelayInfo(t)

	assert.False(t, ShouldFailoverToNextModelCandidate(c, info, nil))
	assert.False(t, ShouldFailoverToNextModelCandidate(c, nil, upstreamError(http.StatusNotFound, types.ErrorCodeBadResponseStatusCode, "model not found")))
}

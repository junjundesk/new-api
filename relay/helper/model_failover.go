package helper

import (
	"strings"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// modelUnavailableKeywords describe upstream errors that report the requested
// model itself as unusable rather than the request being malformed. They only
// count on 4xx responses that are not already covered by the configurable
// retry status codes.
var modelUnavailableKeywords = []string{
	"not found",
	"not exist",
	"not_exist",
	"does not exist",
	"unsupported",
	"invalid model",
	"unavailable",
	"no such model",
	"model not",
}

// ShouldFailoverToNextModelCandidate reports whether this attempt should retry
// the same channel with the next upstream model candidate.
//
// It deliberately requires an explicit signal from the upstream: the error must
// not be marked as skip-retry, must not be an internal channel error, must not
// be a status code that disables the channel, must have written nothing to the
// client yet, and must either be a status code the admin configured for retry
// or look like a "model unavailable" response.
func ShouldFailoverToNextModelCandidate(c *gin.Context, info *common.RelayInfo, apiErr *types.NewAPIError) bool {
	if apiErr == nil || info == nil {
		return false
	}
	if !HasModelCandidates(info) || ModelCandidateExhausted(info) {
		return false
	}
	if types.IsSkipRetryError(apiErr) || types.IsChannelError(apiErr) {
		return false
	}
	// 渠道本身不可用（例如认证失败）时换模型没有意义，交给常规的渠道级重试
	if operation_setting.ShouldDisableByStatusCode(apiErr.StatusCode) {
		return false
	}
	// 客户端已经收到字节，换模型重发会重复输出
	if c != nil && c.Writer != nil && c.Writer.Written() {
		return false
	}
	return statusCodeQualifiesForModelFailover(apiErr) || messageQualifiesForModelFailover(apiErr)
}

func statusCodeQualifiesForModelFailover(apiErr *types.NewAPIError) bool {
	if apiErr.StatusCode == 404 {
		return true
	}
	return operation_setting.ShouldRetryByStatusCode(apiErr.StatusCode)
}

func messageQualifiesForModelFailover(apiErr *types.NewAPIError) bool {
	if apiErr.StatusCode < 400 || apiErr.StatusCode >= 500 {
		return false
	}
	message := strings.ToLower(apiErr.ErrorWithStatusCode())
	if !strings.Contains(message, "model") {
		return false
	}
	for _, keyword := range modelUnavailableKeywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

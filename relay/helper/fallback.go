package helper

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// BuildFallbackState converts the token context configuration into the
// request-scoped state used by the relay loop.
func BuildFallbackState(c *gin.Context, info *relaycommon.RelayInfo) *relaycommon.FallbackState {
	if c == nil || info == nil {
		return nil
	}
	configured, ok := common.GetContextKeyType[[]setting.TokenFallbackModel](c, constant.ContextKeyTokenFallbackModels)
	if !ok || len(configured) == 0 {
		return nil
	}
	state := &relaycommon.FallbackState{
		Configured:    make([]relaycommon.FallbackTarget, 0, len(configured)),
		OriginalModel: info.OriginModelName,
		OriginalGroup: info.UsingGroup,
	}
	for _, item := range configured {
		state.Configured = append(state.Configured, relaycommon.FallbackTarget{
			SourceModel: item.SourceModel,
			Group:       item.Group,
			Model:       item.Model,
		})
	}
	return state
}

// MatchingFallbackTargets keeps the configured order and filters out entries
// scoped to another model.
func MatchingFallbackTargets(state *relaycommon.FallbackState) []relaycommon.FallbackTarget {
	if state == nil || len(state.Configured) == 0 {
		return nil
	}
	targets := make([]relaycommon.FallbackTarget, 0, len(state.Configured))
	for _, target := range state.Configured {
		if target.SourceModel == "" || target.SourceModel == state.OriginalModel {
			targets = append(targets, target)
		}
	}
	return targets
}

// ShouldTryFallback reports whether the failed attempt may be replaced by a
// key-level fallback. It deliberately does not reuse shouldRetry: an empty
// group is a skip-retry error to the channel loop but is a primary fallback
// scenario here.
func ShouldTryFallback(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) bool {
	if c == nil || info == nil || info.Fallback == nil || apiErr == nil {
		return false
	}
	if len(MatchingFallbackTargets(info.Fallback)) == 0 {
		return false
	}
	// Any output already produced — sent downstream or merely received from the
	// upstream — must not be replaced: the client would see concatenated
	// responses and the discarded attempt would still be billable.
	if info.IsChannelTest || c.Writer.Written() || info.SendResponseCount > 0 || info.ReceivedResponseCount > 0 {
		return false
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return false
	}
	switch info.RelayFormat {
	case types.RelayFormatOpenAI, types.RelayFormatClaude, types.RelayFormatGemini,
		types.RelayFormatOpenAIResponses, types.RelayFormatEmbedding:
	default:
		return false
	}
	return isFallbackEligibleError(apiErr)
}

func isFallbackEligibleError(apiErr *types.NewAPIError) bool {
	switch apiErr.GetErrorCode() {
	case types.ErrorCodeGetChannelFailed, types.ErrorCodeBadResponseStatusCode,
		types.ErrorCodeBadResponse, types.ErrorCodeBadResponseBody,
		types.ErrorCodeEmptyResponse, types.ErrorCodeDoRequestFailed,
		types.ErrorCodeModelNotFound, types.ErrorCodeChannelResponseTimeExceeded,
		types.ErrorCodeChannelAwsClientError:
		// Status-code exclusions below still apply.
	case types.ErrorCodeInvalidRequest, types.ErrorCodeSensitiveWordsDetected,
		types.ErrorCodeAccessDenied, types.ErrorCodeInsufficientUserQuota,
		types.ErrorCodePreConsumeTokenQuotaFailed, types.ErrorCodePromptBlocked:
		return false
	default:
		// Unknown errors are eligible only when their status clearly describes
		// an upstream outage rather than a client request problem.
		if apiErr.StatusCode == 0 {
			return false
		}
	}
	code := apiErr.StatusCode
	if code >= http.StatusOK && code < http.StatusMultipleChoices {
		return false
	}
	switch code {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return false
	}
	return code == 0 || code == http.StatusNotFound || code == http.StatusRequestTimeout ||
		code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

// CheckFallbackTarget revalidates permissions at request time: token settings
// may have changed after the key was saved.
func CheckFallbackTarget(c *gin.Context, info *relaycommon.RelayInfo, target relaycommon.FallbackTarget) error {
	// A fallback target must resolve to one concrete group. auto would pick a
	// group at random and a chain would fan out over several groups, so both are
	// rejected here as well as at save time (the stored value may predate the
	// validation, e.g. after a direct database edit).
	if target.Group == "auto" || strings.HasPrefix(target.Group, "chain:") {
		return fmt.Errorf("group %s is not available", target.Group)
	}
	if !service.GroupInUserUsableGroups(info.UserGroup, target.Group) {
		return fmt.Errorf("group %s is not available", target.Group)
	}
	if !slices.Contains(service.GetGroupsEnabledModels([]string{target.Group}), target.Model) {
		return fmt.Errorf("model %s is not available in group %s", target.Model, target.Group)
	}
	if common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		limits, _ := common.GetContextKeyType[map[string]bool](c, constant.ContextKeyTokenModelLimit)
		if !limits[target.Model] && !limits[ratio_setting.FormatMatchingModelName(target.Model)] {
			return fmt.Errorf("model %s is not allowed by this key", target.Model)
		}
	}
	return nil
}

// rewriteCachedBodyModel replaces the model field of the cached request body.
// Pass-through channels replay that body verbatim instead of marshaling the
// parsed request, so without the rewrite a fallback target would still be sent
// to the model the client originally asked for.
func rewriteCachedBodyModel(c *gin.Context, modelName string) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return
	}
	body, err := storage.Bytes()
	if err != nil || !gjson.ValidBytes(body) || !gjson.GetBytes(body, "model").Exists() {
		return
	}
	updated, err := sjson.SetBytes(body, "model", modelName)
	if err != nil {
		return
	}
	replacement, err := common.CreateBodyStorage(updated)
	if err != nil {
		return
	}
	_ = storage.Close()
	c.Set(common.KeyBodyStorage, replacement)
	c.Request.ContentLength = int64(len(updated))
}

// PrepareFallbackTarget rewrites the request state before the fallback attempt
// selects its channel. ChannelMeta must stay non-nil: getChannel only performs
// a fresh selection when ChannelMeta is non-nil.
func PrepareFallbackTarget(c *gin.Context, info *relaycommon.RelayInfo, target relaycommon.FallbackTarget) {
	// UpstreamModelName and IsModelMapped are promoted from ChannelMeta. A first
	// attempt that failed before channel selection leaves ChannelMeta nil, so it
	// must be created before those fields are assigned.
	if info.ChannelMeta == nil {
		info.ChannelMeta = &relaycommon.ChannelMeta{}
	}
	info.OriginModelName = target.Model
	info.UsingGroup = target.Group
	info.UpstreamModelName = target.Model
	info.IsModelMapped = false
	info.ModelCandidate = nil
	info.PriceData = hosttypes.PriceData{}
	info.TieredBillingSnapshot = nil
	info.BillingRequestInput = nil
	info.RequestConversionChain = []types.RelayFormat{info.RelayFormat}
	info.FinalRequestRelayFormat = ""
	if info.Request != nil {
		info.Request.SetModelName(target.Model)
	}
	rewriteCachedBodyModel(c, target.Model)
	c.Set("original_model", target.Model)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, target.Group)
	common.SetContextKey(c, constant.ContextKeyAutoGroup, target.Group)
	common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, 0)
	common.SetContextKey(c, constant.ContextKeyAutoGroupRetryIndex, 0)
}

package controller

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/QuantumNous/new-api/relay/helper"

	"github.com/gin-gonic/gin"
)

// runFallbackAttempts tries the key-level fallback targets in order. Each
// target is attempted once; success returns (nil, true), exhaustion returns
// the last upstream error and false after restoring the original model state.
func runFallbackAttempts(c *gin.Context, relayFormat types.RelayFormat, info *relaycommon.RelayInfo,
	tokens int, meta *types.TokenCountMeta) (*types.NewAPIError, bool) {
	state := info.Fallback
	targets := helper.MatchingFallbackTargets(state)
	if len(targets) == 0 {
		return nil, false
	}

	var lastErr *types.NewAPIError
	for i, target := range targets {
		attempt := relaycommon.FallbackAttempt{
			Index: len(state.Attempts) + 1,
			Group: target.Group,
			Model: target.Model,
		}

		if err := helper.CheckFallbackTarget(c, info, target); err != nil {
			attempt.Error = common.LocalLogPreview(err.Error())
			state.Attempts = append(state.Attempts, attempt)
			logger.LogWarn(c, fmt.Sprintf("fallback target %s/%s skipped: %s", target.Group, target.Model, err.Error()))
			continue
		}

		helper.PrepareFallbackTarget(c, info, target)
		retryParam := &service.RetryParam{
			Ctx:         c,
			TokenGroup:  target.Group,
			ModelName:   target.Model,
			RequestPath: c.Request.URL.Path,
			Retry:       common.GetPointer(0),
		}
		result := runRelayAttempt(c, relayFormat, info, retryParam, relayAttemptOptions{
			reprice: true,
			tokens:  tokens,
			meta:    meta,
		})
		if result.channel != nil {
			attempt.ChannelId = result.channel.Id
		}

		if result.err == nil {
			state.Attempts = append(state.Attempts, attempt)
			state.Used = &state.Attempts[len(state.Attempts)-1]
			logger.LogInfo(c, fmt.Sprintf("fallback succeeded: %s -> %s (%s/%s)", state.OriginalModel, target.Model, target.Group, target.Model))
			return nil, true
		}

		attempt.StatusCode = result.err.StatusCode
		attempt.Error = common.LocalLogPreview(result.err.Error())
		lastErr = result.err
		state.Attempts = append(state.Attempts, attempt)

		// Once a failed fallback has emitted output, trying another target would
		// concatenate responses or bill for an invisible upstream response.
		if c.Writer.Written() || info.SendResponseCount > 0 || (c.Request != nil && c.Request.Context().Err() != nil) {
			break
		}

		if result.channel != nil && result.channelFailure {
			processChannelError(c, *types.NewChannelError(
				result.channel.Id,
				result.channel.Type,
				result.channel.Name,
				result.channel.ChannelInfo.IsMultiKey,
				common.GetContextKeyString(c, constant.ContextKeyChannelKey),
				result.channel.GetAutoBan(),
			), result.err, i < len(targets)-1)
			continue
		}
		logger.LogWarn(c, fmt.Sprintf("fallback attempt %s/%s failed before dispatch: %s", target.Group, target.Model, result.err.Error()))
	}

	// Restore the request-facing model so the final error response and logs
	// describe what the client asked for, not the last fallback target.
	if state.OriginalModel != "" {
		info.OriginModelName = state.OriginalModel
		info.UsingGroup = state.OriginalGroup
		c.Set("original_model", state.OriginalModel)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, state.OriginalGroup)
	}
	return lastErr, false
}

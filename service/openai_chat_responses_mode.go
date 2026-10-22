package service

import (
	"regexp"
	"sync"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// Chat→Responses upgrade policy is host routing logic (it decides *whether*
// to convert, reading host settings), so it lives here, not in relayconvert.

var chatResponsesRegexCache sync.Map // map[string]*regexp.Regexp

func matchAnyModelPattern(patterns []string, model string) bool {
	if len(patterns) == 0 || model == "" {
		return false
	}
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		re, ok := chatResponsesRegexCache.Load(pattern)
		if !ok {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				// Treat invalid patterns as non-matching to avoid breaking runtime traffic.
				continue
			}
			re = compiled
			chatResponsesRegexCache.Store(pattern, re)
		}
		if re.(*regexp.Regexp).MatchString(model) {
			return true
		}
	}
	return false
}

// FixedUpstreamRelayFormat 返回渠道固定使用的上游线上格式。
// 固定格式渠道忽略客户端格式：与固定格式不一致的请求一律先转换再发上游。
func FixedUpstreamRelayFormat(channelType int) (types.RelayFormat, bool) {
	switch channelType {
	case constant.ChannelTypeOpenAIChat:
		return types.RelayFormatOpenAI, true
	case constant.ChannelTypeOpenAIResponses:
		return types.RelayFormatOpenAIResponses, true
	default:
		return "", false
	}
}

// FixedFormatRequiresConversion 判断客户端格式与渠道固定格式是否不一致。
// responses/compact 属于 responses 家族，与固定 responses 渠道视为一致，
// 否则 compact 会被误判为需要转换，从而丢掉原生透传能力。
func FixedFormatRequiresConversion(channelType int, clientFormat types.RelayFormat) bool {
	fixedFormat, ok := FixedUpstreamRelayFormat(channelType)
	if !ok {
		return false
	}
	if clientFormat == types.RelayFormatOpenAIResponsesCompaction {
		clientFormat = types.RelayFormatOpenAIResponses
	}
	return fixedFormat != clientFormat
}

func ShouldChatCompletionsUseResponsesPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelID int, channelType int, model string) bool {
	// 固定格式渠道优先于全局策略：固定 responses 渠道的 chat / claude 客户端
	// 请求一律先转换为 responses 再发送上游（responses 客户端原生透传）；
	// 固定 chat 渠道永远不升级为 responses，否则会与渠道固定格式冲突。
	switch channelType {
	case constant.ChannelTypeOpenAIResponses:
		return true
	case constant.ChannelTypeOpenAIChat:
		return false
	}
	if !policy.IsChannelEnabled(channelID, channelType) {
		return false
	}
	return matchAnyModelPattern(policy.ModelPatterns, model)
}

func ShouldChatCompletionsUseResponsesGlobal(channelID int, channelType int, model string) bool {
	return ShouldChatCompletionsUseResponsesPolicy(
		model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy,
		channelID,
		channelType,
		model,
	)
}

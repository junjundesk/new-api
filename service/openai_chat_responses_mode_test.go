package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
)

func TestShouldChatCompletionsUseResponsesPolicyPrefersFixedChannelFormat(t *testing.T) {
	enabledAll := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   true,
		ModelPatterns: []string{".*"},
	}
	enabledChannelIDs := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		ChannelIDs:    []int{42},
		ModelPatterns: []string{".*"},
	}
	disabled := model_setting.ChatCompletionsToResponsesPolicy{}

	tests := []struct {
		name        string
		policy      model_setting.ChatCompletionsToResponsesPolicy
		channelID   int
		channelType int
		want        bool
	}{
		{name: "fixed responses channel always upgrades", policy: disabled, channelID: 7, channelType: constant.ChannelTypeOpenAIResponses, want: true},
		{name: "fixed chat channel ignores all-channels policy", policy: enabledAll, channelID: 7, channelType: constant.ChannelTypeOpenAIChat, want: false},
		{name: "fixed chat channel ignores channel-id policy", policy: enabledChannelIDs, channelID: 42, channelType: constant.ChannelTypeOpenAIChat, want: false},
		{name: "plain openai channel follows enabled policy", policy: enabledAll, channelID: 7, channelType: constant.ChannelTypeOpenAI, want: true},
		{name: "plain openai channel follows disabled policy", policy: disabled, channelID: 7, channelType: constant.ChannelTypeOpenAI, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldChatCompletionsUseResponsesPolicy(tt.policy, tt.channelID, tt.channelType, "gpt-test"))
		})
	}
}

func TestFixedFormatRequiresConversion(t *testing.T) {
	tests := []struct {
		name         string
		channelType  int
		clientFormat types.RelayFormat
		want         bool
	}{
		{name: "chat client on fixed chat channel", channelType: constant.ChannelTypeOpenAIChat, clientFormat: types.RelayFormatOpenAI, want: false},
		{name: "responses client on fixed chat channel", channelType: constant.ChannelTypeOpenAIChat, clientFormat: types.RelayFormatOpenAIResponses, want: true},
		{name: "compaction client on fixed chat channel", channelType: constant.ChannelTypeOpenAIChat, clientFormat: types.RelayFormatOpenAIResponsesCompaction, want: true},
		{name: "claude client on fixed chat channel", channelType: constant.ChannelTypeOpenAIChat, clientFormat: types.RelayFormatClaude, want: true},
		{name: "gemini client on fixed chat channel", channelType: constant.ChannelTypeOpenAIChat, clientFormat: types.RelayFormatGemini, want: true},
		{name: "responses client on fixed responses channel", channelType: constant.ChannelTypeOpenAIResponses, clientFormat: types.RelayFormatOpenAIResponses, want: false},
		{name: "compaction client on fixed responses channel", channelType: constant.ChannelTypeOpenAIResponses, clientFormat: types.RelayFormatOpenAIResponsesCompaction, want: false},
		{name: "chat client on fixed responses channel", channelType: constant.ChannelTypeOpenAIResponses, clientFormat: types.RelayFormatOpenAI, want: true},
		{name: "claude client on fixed responses channel", channelType: constant.ChannelTypeOpenAIResponses, clientFormat: types.RelayFormatClaude, want: true},
		{name: "gemini client on fixed responses channel", channelType: constant.ChannelTypeOpenAIResponses, clientFormat: types.RelayFormatGemini, want: true},
		{name: "chat client on plain openai channel", channelType: constant.ChannelTypeOpenAI, clientFormat: types.RelayFormatOpenAI, want: false},
		{name: "responses client on plain openai channel", channelType: constant.ChannelTypeOpenAI, clientFormat: types.RelayFormatOpenAIResponses, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FixedFormatRequiresConversion(tt.channelType, tt.clientFormat))
		})
	}
}

package assistant

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/assistant_setting"
)

// ResolveConfig turns a profile into a concrete upstream configuration. For a
// channel-backed profile it reads the channel (including disabled channels) and
// its key, so the assistant works even when the channel is not in the relay
// selection pool. The key is only returned in memory and never exposed to the
// browser.
func ResolveConfig(profile assistant_setting.Profile) (Config, error) {
	setting := assistant_setting.Get()
	dailyLimit := setting.DailyUserMessageLimit
	if dailyLimit < 0 {
		dailyLimit = 0
	}
	if dailyLimit > 10000 {
		dailyLimit = 10000
	}
	cfg := Config{
		SystemPrompt:          setting.SystemPrompt,
		MaxToolRounds:         clampInt(setting.MaxToolRounds, 5, 1, 10),
		RequestTimeout:        time.Duration(clampInt(setting.RequestTimeoutSeconds, 60, 5, 300)) * time.Second,
		MaxOutputTokens:       clampInt(setting.MaxOutputTokens, 1500, 64, 8192),
		DailyUserMessageLimit: dailyLimit,
		AllowTokenActions:     setting.AllowTokenActions,
	}

	if profile.Source == assistant_setting.SourceChannel {
		if profile.ChannelID <= 0 {
			return Config{}, fmt.Errorf("assistant channel is not selected")
		}
		channel, err := model.GetChannelById(profile.ChannelID, true)
		if err != nil {
			return Config{}, fmt.Errorf("assistant channel not found")
		}
		// Azure channels need api-key headers and deployment paths; use a
		// direct OpenAI-compatible endpoint for those deployments instead.
		if channel.Type == constant.ChannelTypeAzure {
			return Config{}, fmt.Errorf("Azure channels are not supported for the assistant; configure a direct OpenAI-compatible endpoint")
		}
		modelName := strings.TrimSpace(profile.ChannelModel)
		if modelName == "" {
			return Config{}, fmt.Errorf("assistant channel model is not selected")
		}
		if mapping := channel.GetModelMapping(); mapping != "" {
			if parsed, parseErr := model.ParseChannelModelMapping(mapping); parseErr == nil {
				if candidates, ok := parsed[modelName]; ok && len(candidates) > 0 {
					modelName = strings.TrimSpace(candidates[0])
				}
			}
		}
		baseURL := strings.TrimSpace(channel.GetBaseURL())
		if baseURL == "" {
			return Config{}, fmt.Errorf("assistant channel has no base URL")
		}
		baseURL = strings.ReplaceAll(baseURL, "{model}", modelName)
		normalizedBaseURL, err := assistant_setting.NormalizeChannelBaseURL(baseURL)
		if err != nil {
			return Config{}, err
		}
		apiKey := strings.TrimSpace(profile.APIKey)
		if apiKey == "" {
			key, _, apiErr := channel.GetNextEnabledKey()
			if apiErr != nil || strings.TrimSpace(key) == "" {
				return Config{}, fmt.Errorf("assistant channel has no available key")
			}
			apiKey = strings.TrimSpace(key)
		}
		cfg.BaseURL = normalizedBaseURL
		cfg.APIKey = apiKey
		cfg.Model = modelName
		return cfg, nil
	}

	baseURL := strings.TrimSpace(profile.BaseURL)
	if baseURL == "" {
		return Config{}, fmt.Errorf("assistant base URL is not configured")
	}
	normalizedBaseURL, err := assistant_setting.NormalizeBaseURL(baseURL)
	if err != nil {
		return Config{}, err
	}
	apiKey := strings.TrimSpace(profile.APIKey)
	if apiKey == "" {
		return Config{}, fmt.Errorf("assistant API key is not configured")
	}
	modelName := strings.TrimSpace(profile.Model)
	if modelName == "" {
		return Config{}, fmt.Errorf("assistant model is not configured")
	}
	cfg.BaseURL = normalizedBaseURL
	cfg.APIKey = apiKey
	cfg.Model = modelName
	return cfg, nil
}

func clampInt(value, fallback, minValue, maxValue int) int {
	if value <= 0 && fallback > 0 {
		value = fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

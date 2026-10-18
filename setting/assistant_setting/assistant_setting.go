package assistant_setting

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	ProfileUser  = "user"
	ProfileAdmin = "admin"

	SourceDirect  = "direct"
	SourceChannel = "channel"

	OptionKeyUserEnabled           = "assistant_setting.user_enabled"
	OptionKeyUserSource            = "assistant_setting.user_source"
	OptionKeyUserBaseURL           = "assistant_setting.user_base_url"
	OptionKeyUserAPIKey            = "assistant_setting.user_api_key"
	OptionKeyUserModel             = "assistant_setting.user_model"
	OptionKeyUserChannelID         = "assistant_setting.user_channel_id"
	OptionKeyUserChannelModel      = "assistant_setting.user_channel_model"
	OptionKeyAdminEnabled          = "assistant_setting.admin_enabled"
	OptionKeyAdminSource           = "assistant_setting.admin_source"
	OptionKeyAdminBaseURL          = "assistant_setting.admin_base_url"
	OptionKeyAdminAPIKey           = "assistant_setting.admin_api_key"
	OptionKeyAdminModel            = "assistant_setting.admin_model"
	OptionKeyAdminChannelID        = "assistant_setting.admin_channel_id"
	OptionKeyAdminChannelModel     = "assistant_setting.admin_channel_model"
	OptionKeySystemPrompt          = "assistant_setting.system_prompt"
	OptionKeyMaxToolRounds         = "assistant_setting.max_tool_rounds"
	OptionKeyRequestTimeout        = "assistant_setting.request_timeout_seconds"
	OptionKeyMaxOutputTokens       = "assistant_setting.max_output_tokens"
	OptionKeyDailyMessageLimit     = "assistant_setting.daily_user_message_limit"
	OptionKeyAllowTokenActions     = "assistant_setting.allow_token_actions"
	OptionKeyUserAPIKeyConfigured  = "assistant_setting.user_api_key_configured"
	OptionKeyAdminAPIKeyConfigured = "assistant_setting.admin_api_key_configured"
)

// AssistantSetting stores two independent upstream profiles. Each profile can
// use a direct OpenAI-compatible endpoint or bind a channel (including a
// disabled channel) and one of that channel's models.
type AssistantSetting struct {
	UserEnabled      bool   `json:"user_enabled"`
	UserSource       string `json:"user_source"`
	UserBaseURL      string `json:"user_base_url"`
	UserAPIKey       string `json:"user_api_key"`
	UserModel        string `json:"user_model"`
	UserChannelID    int    `json:"user_channel_id"`
	UserChannelModel string `json:"user_channel_model"`

	AdminEnabled      bool   `json:"admin_enabled"`
	AdminSource       string `json:"admin_source"`
	AdminBaseURL      string `json:"admin_base_url"`
	AdminAPIKey       string `json:"admin_api_key"`
	AdminModel        string `json:"admin_model"`
	AdminChannelID    int    `json:"admin_channel_id"`
	AdminChannelModel string `json:"admin_channel_model"`

	SystemPrompt          string `json:"system_prompt"`
	MaxToolRounds         int    `json:"max_tool_rounds"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	MaxOutputTokens       int    `json:"max_output_tokens"`
	DailyUserMessageLimit int    `json:"daily_user_message_limit"`
	AllowTokenActions     bool   `json:"allow_token_actions"`
}

// Profile is a request-scoped subset used by the assistant runtime.
type Profile struct {
	Name         string
	Enabled      bool
	Source       string
	BaseURL      string
	APIKey       string
	Model        string
	ChannelID    int
	ChannelModel string
}

var assistantSetting = AssistantSetting{
	UserEnabled:           false,
	UserSource:            SourceDirect,
	UserBaseURL:           "https://api.openai.com/v1",
	UserAPIKey:            "",
	UserModel:             "gpt-4.1-mini",
	UserChannelID:         0,
	UserChannelModel:      "",
	AdminEnabled:          false,
	AdminSource:           SourceDirect,
	AdminBaseURL:          "https://api.openai.com/v1",
	AdminAPIKey:           "",
	AdminModel:            "gpt-4.1-mini",
	AdminChannelID:        0,
	AdminChannelModel:     "",
	SystemPrompt:          "",
	MaxToolRounds:         5,
	RequestTimeoutSeconds: 60,
	MaxOutputTokens:       1500,
	DailyUserMessageLimit: 50,
	AllowTokenActions:     true,
}

func init() {
	config.GlobalConfig.Register("assistant_setting", &assistantSetting)
}

func Get() *AssistantSetting {
	return &assistantSetting
}

func GetProfile(name string) Profile {
	if name == ProfileAdmin {
		return Profile{
			Name:         ProfileAdmin,
			Enabled:      assistantSetting.AdminEnabled,
			Source:       normalizeSource(assistantSetting.AdminSource),
			BaseURL:      strings.TrimSpace(assistantSetting.AdminBaseURL),
			APIKey:       strings.TrimSpace(assistantSetting.AdminAPIKey),
			Model:        strings.TrimSpace(assistantSetting.AdminModel),
			ChannelID:    assistantSetting.AdminChannelID,
			ChannelModel: strings.TrimSpace(assistantSetting.AdminChannelModel),
		}
	}
	return Profile{
		Name:         ProfileUser,
		Enabled:      assistantSetting.UserEnabled,
		Source:       normalizeSource(assistantSetting.UserSource),
		BaseURL:      strings.TrimSpace(assistantSetting.UserBaseURL),
		APIKey:       strings.TrimSpace(assistantSetting.UserAPIKey),
		Model:        strings.TrimSpace(assistantSetting.UserModel),
		ChannelID:    assistantSetting.UserChannelID,
		ChannelModel: strings.TrimSpace(assistantSetting.UserChannelModel),
	}
}

// IsConfigured reports whether the profile has the minimum fields needed to
// try a request. Channel-backed profiles do not need a profile-level API key:
// the channel key is used unless the profile overrides it.
func IsConfigured(name string) bool {
	profile := GetProfile(name)
	if profile.Source == SourceChannel {
		return profile.ChannelID > 0 && profile.ChannelModel != ""
	}
	return profile.BaseURL != "" && profile.APIKey != "" && profile.Model != ""
}

func IsAvailable(name string) bool {
	profile := GetProfile(name)
	return profile.Enabled && IsConfigured(name)
}

func ModelName(name string) string {
	profile := GetProfile(name)
	if profile.Source == SourceChannel {
		return profile.ChannelModel
	}
	return profile.Model
}

// ProfileForOptionKey reports which profile an option key belongs to.
func ProfileForOptionKey(key string) (string, bool) {
	if strings.HasPrefix(key, "assistant_setting.user_") {
		return ProfileUser, true
	}
	if strings.HasPrefix(key, "assistant_setting.admin_") {
		return ProfileAdmin, true
	}
	return "", false
}

func IsOptionKey(key string) bool {
	return strings.HasPrefix(key, "assistant_setting.")
}

func normalizeSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == SourceChannel {
		return SourceChannel
	}
	return SourceDirect
}

// NormalizeBaseURL validates a direct OpenAI-compatible API root. The client
// appends "/responses" to the returned value.
func NormalizeBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("base URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", errors.New("base URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("base URL must start with http:// or https://")
	}
	if parsed.Host == "" {
		return "", errors.New("base URL must include a host")
	}
	if parsed.User != nil {
		return "", errors.New("base URL must not include credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("base URL must not include a query string or fragment")
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	if strings.HasSuffix(strings.ToLower(path), "/responses") {
		return "", errors.New("base URL must be the API root, not the /responses endpoint")
	}
	if path == "" {
		path = "/v1"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return strings.TrimSuffix(parsed.String(), "/"), nil
}

// NormalizeChannelBaseURL converts a channel base URL into the API root used by
// the assistant client. OpenAI channels store "https://api.openai.com" while
// other OpenAI-compatible gateways may store "/api" or "/v1"; the result always
// ends with "/v1" so the client can append "/responses".
func NormalizeChannelBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("channel base URL is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", errors.New("channel base URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("channel base URL must start with http:// or https://")
	}
	if parsed.Host == "" {
		return "", errors.New("channel base URL must include a host")
	}
	if parsed.User != nil {
		return "", errors.New("channel base URL must not include credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("channel base URL must not include a query string or fragment")
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	if strings.HasSuffix(strings.ToLower(path), "/responses") {
		path = strings.TrimSuffix(path, "/responses")
	}
	if path == "" {
		path = "/v1"
	} else if !strings.HasSuffix(strings.ToLower(path), "/v1") {
		path += "/v1"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return strings.TrimSuffix(parsed.String(), "/"), nil
}

// NormalizeOptionValue validates one assistant option. skip means "accept the
// request without writing"; an empty API key uses this so the admin can keep
// the existing secret.
func NormalizeOptionValue(key, value string) (normalized string, skip bool, err error) {
	if profileName, ok := ProfileForOptionKey(key); ok {
		field := strings.TrimPrefix(key, "assistant_setting."+profileName+"_")
		return normalizeProfileOption(profileName, field, value)
	}
	switch key {
	case OptionKeySystemPrompt:
		if len(value) > 8000 {
			return "", false, errors.New("system prompt is too long (max 8000 characters)")
		}
		return value, false, nil
	case OptionKeyMaxToolRounds:
		return normalizeIntOption(value, 1, 10)
	case OptionKeyRequestTimeout:
		return normalizeIntOption(value, 5, 300)
	case OptionKeyMaxOutputTokens:
		return normalizeIntOption(value, 64, 8192)
	case OptionKeyDailyMessageLimit:
		return normalizeIntOption(value, 0, 10000)
	case OptionKeyAllowTokenActions:
		parsed, parseErr := strconv.ParseBool(strings.TrimSpace(value))
		if parseErr != nil {
			return "", false, fmt.Errorf("%s must be true or false", key)
		}
		return strconv.FormatBool(parsed), false, nil
	default:
		return "", false, fmt.Errorf("unknown assistant setting: %s", key)
	}
}

func normalizeProfileOption(profileName, field, value string) (string, bool, error) {
	switch field {
	case "enabled":
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return "", false, fmt.Errorf("%s enabled must be true or false", profileName)
		}
		return strconv.FormatBool(parsed), false, nil
	case "source":
		source := strings.ToLower(strings.TrimSpace(value))
		if source != SourceDirect && source != SourceChannel {
			return "", false, fmt.Errorf("%s source must be direct or channel", profileName)
		}
		return source, false, nil
	case "base_url":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "", false, nil
		}
		normalized, err := NormalizeBaseURL(trimmed)
		return normalized, false, err
	case "api_key":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "", true, nil
		}
		if len(trimmed) > 4096 {
			return "", false, errors.New("API key is too long")
		}
		return trimmed, false, nil
	case "model":
		trimmed := strings.TrimSpace(value)
		if len(trimmed) > 128 {
			return "", false, errors.New("model name is too long")
		}
		return trimmed, false, nil
	case "channel_id":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "0", false, nil
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil || parsed < 0 {
			return "", false, errors.New("channel id must be a non-negative integer")
		}
		return strconv.Itoa(parsed), false, nil
	case "channel_model":
		trimmed := strings.TrimSpace(value)
		if len(trimmed) > 128 {
			return "", false, errors.New("channel model name is too long")
		}
		return trimmed, false, nil
	default:
		return "", false, fmt.Errorf("unknown assistant profile setting: %s", field)
	}
}

func normalizeIntOption(value string, minValue, maxValue int) (string, bool, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return "", false, fmt.Errorf("value must be an integer between %d and %d", minValue, maxValue)
	}
	if parsed < minValue || parsed > maxValue {
		return "", false, fmt.Errorf("value must be between %d and %d", minValue, maxValue)
	}
	return strconv.Itoa(parsed), false, nil
}

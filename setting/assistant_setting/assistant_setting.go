package assistant_setting

import (
	"errors"
	"fmt"
	"math"
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

	ProtocolResponses = "responses"
	ProtocolChat      = "chat"

	OptionKeyEnabled               = "assistant_setting.enabled"
	OptionKeyUserEnabled           = "assistant_setting.user_enabled"
	OptionKeyUserSource            = "assistant_setting.user_source"
	OptionKeyUserProtocol          = "assistant_setting.user_protocol"
	OptionKeyUserBaseURL           = "assistant_setting.user_base_url"
	OptionKeyUserAPIKey            = "assistant_setting.user_api_key"
	OptionKeyUserModel             = "assistant_setting.user_model"
	OptionKeyUserChannelID         = "assistant_setting.user_channel_id"
	OptionKeyUserChannelModel      = "assistant_setting.user_channel_model"
	OptionKeyUserContextWindow     = "assistant_setting.user_context_window"
	OptionKeyUserMaxOutputTokens   = "assistant_setting.user_max_output_tokens"
	OptionKeyAdminEnabled          = "assistant_setting.admin_enabled"
	OptionKeyAdminSource           = "assistant_setting.admin_source"
	OptionKeyAdminProtocol         = "assistant_setting.admin_protocol"
	OptionKeyAdminBaseURL          = "assistant_setting.admin_base_url"
	OptionKeyAdminAPIKey           = "assistant_setting.admin_api_key"
	OptionKeyAdminModel            = "assistant_setting.admin_model"
	OptionKeyAdminChannelID        = "assistant_setting.admin_channel_id"
	OptionKeyAdminChannelModel     = "assistant_setting.admin_channel_model"
	OptionKeyAdminContextWindow    = "assistant_setting.admin_context_window"
	OptionKeyAdminMaxOutputTokens  = "assistant_setting.admin_max_output_tokens"
	OptionKeySystemPrompt          = "assistant_setting.system_prompt"
	OptionKeyRequestTimeout        = "assistant_setting.request_timeout_seconds"
	OptionKeyConfirmWrites         = "assistant_setting.confirm_writes"
	OptionKeyAdminDBToolsEnabled   = "assistant_setting.admin_db_tools_enabled"
	OptionKeyUserDailyTokenLimit   = "assistant_setting.user_daily_token_limit"
	OptionKeyUserAPIKeyConfigured  = "assistant_setting.user_api_key_configured"
	OptionKeyAdminAPIKeyConfigured = "assistant_setting.admin_api_key_configured"
)

// AssistantSetting stores two independent upstream profiles. Each profile can
// use a direct OpenAI-compatible endpoint or bind a channel (including a
// disabled channel) and one of that channel's models.
type AssistantSetting struct {
	Enabled bool `json:"enabled"`

	UserEnabled         bool   `json:"user_enabled"`
	UserSource          string `json:"user_source"`
	UserProtocol        string `json:"user_protocol"`
	UserBaseURL         string `json:"user_base_url"`
	UserAPIKey          string `json:"user_api_key"`
	UserModel           string `json:"user_model"`
	UserChannelID       int    `json:"user_channel_id"`
	UserChannelModel    string `json:"user_channel_model"`
	UserContextWindow   int    `json:"user_context_window"`
	UserMaxOutputTokens int    `json:"user_max_output_tokens"`

	AdminEnabled         bool   `json:"admin_enabled"`
	AdminSource          string `json:"admin_source"`
	AdminProtocol        string `json:"admin_protocol"`
	AdminBaseURL         string `json:"admin_base_url"`
	AdminAPIKey          string `json:"admin_api_key"`
	AdminModel           string `json:"admin_model"`
	AdminChannelID       int    `json:"admin_channel_id"`
	AdminChannelModel    string `json:"admin_channel_model"`
	AdminContextWindow   int    `json:"admin_context_window"`
	AdminMaxOutputTokens int    `json:"admin_max_output_tokens"`

	SystemPrompt          string `json:"system_prompt"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	ConfirmWrites         bool   `json:"confirm_writes"`
	AdminDBToolsEnabled   bool   `json:"admin_db_tools_enabled"`
	UserDailyTokenLimit   int    `json:"user_daily_token_limit"`
}

// Profile is a request-scoped subset used by the assistant runtime.
type Profile struct {
	Name            string
	Enabled         bool
	Source          string
	Protocol        string
	BaseURL         string
	APIKey          string
	Model           string
	ChannelID       int
	ChannelModel    string
	ContextWindow   int
	MaxOutputTokens int
}

var assistantSetting = AssistantSetting{
	Enabled:               true,
	UserEnabled:           false,
	UserSource:            SourceDirect,
	UserProtocol:          ProtocolResponses,
	UserBaseURL:           "https://api.openai.com/v1",
	UserAPIKey:            "",
	UserModel:             "gpt-4.1-mini",
	UserChannelID:         0,
	UserChannelModel:      "",
	UserContextWindow:     1000000,
	UserMaxOutputTokens:   128000,
	AdminEnabled:          false,
	AdminSource:           SourceDirect,
	AdminProtocol:         ProtocolResponses,
	AdminBaseURL:          "https://api.openai.com/v1",
	AdminAPIKey:           "",
	AdminModel:            "gpt-4.1-mini",
	AdminChannelID:        0,
	AdminChannelModel:     "",
	AdminContextWindow:    1000000,
	AdminMaxOutputTokens:  128000,
	SystemPrompt:          "",
	RequestTimeoutSeconds: 60,
	ConfirmWrites:         true,
	AdminDBToolsEnabled:   true,
	UserDailyTokenLimit:   100000000,
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
			Name:            ProfileAdmin,
			Enabled:         assistantSetting.AdminEnabled,
			Source:          normalizeSource(assistantSetting.AdminSource),
			Protocol:        normalizeProtocol(assistantSetting.AdminProtocol),
			BaseURL:         strings.TrimSpace(assistantSetting.AdminBaseURL),
			APIKey:          strings.TrimSpace(assistantSetting.AdminAPIKey),
			Model:           strings.TrimSpace(assistantSetting.AdminModel),
			ChannelID:       assistantSetting.AdminChannelID,
			ChannelModel:    strings.TrimSpace(assistantSetting.AdminChannelModel),
			ContextWindow:   assistantSetting.AdminContextWindow,
			MaxOutputTokens: assistantSetting.AdminMaxOutputTokens,
		}
	}
	return Profile{
		Name:            ProfileUser,
		Enabled:         assistantSetting.UserEnabled,
		Source:          normalizeSource(assistantSetting.UserSource),
		Protocol:        normalizeProtocol(assistantSetting.UserProtocol),
		BaseURL:         strings.TrimSpace(assistantSetting.UserBaseURL),
		APIKey:          strings.TrimSpace(assistantSetting.UserAPIKey),
		Model:           strings.TrimSpace(assistantSetting.UserModel),
		ChannelID:       assistantSetting.UserChannelID,
		ChannelModel:    strings.TrimSpace(assistantSetting.UserChannelModel),
		ContextWindow:   assistantSetting.UserContextWindow,
		MaxOutputTokens: assistantSetting.UserMaxOutputTokens,
	}
}

// IsConfigured reports whether the profile has the minimum fields needed to
// try a request. Channel-backed profiles do not need a profile-level API key:
// the channel key is used unless the profile overrides it.
func IsConfigured(name string) bool {
	profile := GetProfile(name)
	if profile.Protocol != ProtocolResponses && profile.Protocol != ProtocolChat {
		return false
	}
	if profile.ContextWindow <= 0 || profile.MaxOutputTokens <= 0 || profile.MaxOutputTokens >= profile.ContextWindow {
		return false
	}
	if profile.Source == SourceChannel {
		return profile.ChannelID > 0 && profile.ChannelModel != ""
	}
	return profile.BaseURL != "" && profile.APIKey != "" && profile.Model != ""
}

func IsAvailable(name string) bool {
	if !assistantSetting.Enabled {
		return false
	}
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
	switch key {
	case OptionKeyEnabled, OptionKeySystemPrompt, OptionKeyRequestTimeout, OptionKeyConfirmWrites,
		OptionKeyAdminDBToolsEnabled, OptionKeyUserDailyTokenLimit, OptionKeyUserAPIKeyConfigured, OptionKeyAdminAPIKeyConfigured:
		return "", false
	}
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

func normalizeProtocol(protocol string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	switch protocol {
	case ProtocolChat:
		return ProtocolChat
	case "", ProtocolResponses:
		return ProtocolResponses
	default:
		return protocol
	}
}

// NormalizeBaseURL validates a direct OpenAI-compatible API root. The client
// appends "/responses" or "/chat/completions" to the returned value.
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
	if strings.HasSuffix(strings.ToLower(path), "/responses") || strings.HasSuffix(strings.ToLower(path), "/chat/completions") {
		return "", errors.New("base URL must be the API root, not a completions endpoint")
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
// ends with "/v1" so the client can append its protocol endpoint.
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
	if strings.HasSuffix(strings.ToLower(path), "/chat/completions") {
		path = strings.TrimSuffix(path, "/chat/completions")
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
	case OptionKeyEnabled:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return "", false, errors.New("assistant enabled must be true or false")
		}
		return strconv.FormatBool(parsed), false, nil
	case OptionKeySystemPrompt:
		if len(value) > 8000 {
			return "", false, errors.New("system prompt is too long (max 8000 characters)")
		}
		return value, false, nil
	case OptionKeyRequestTimeout:
		return normalizeIntOption(value, 5, 300)
	case OptionKeyConfirmWrites:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return "", false, errors.New("confirm_writes must be true or false")
		}
		return strconv.FormatBool(parsed), false, nil
	case OptionKeyAdminDBToolsEnabled:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return "", false, errors.New("admin_db_tools_enabled must be true or false")
		}
		return strconv.FormatBool(parsed), false, nil
	case OptionKeyUserDailyTokenLimit:
		return normalizeTokenOption(value, 0, 1000000000000)
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
	case "protocol":
		protocol := strings.ToLower(strings.TrimSpace(value))
		if protocol != ProtocolResponses && protocol != ProtocolChat {
			return "", false, fmt.Errorf("%s protocol must be responses or chat", profileName)
		}
		return protocol, false, nil
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
	case "context_window":
		return normalizeTokenOption(value, 1024, 10000000)
	case "max_output_tokens":
		return normalizeTokenOption(value, 1, 1000000)
	default:
		return "", false, fmt.Errorf("unknown assistant profile setting: %s", field)
	}
}

// ParseTokenAmount parses a human-readable token count such as "128k", "1M",
// "1亿", or "1000000". Negative, non-numeric, and absurdly large values fail.
func ParseTokenAmount(raw string) (int, error) {
	text := strings.ToLower(strings.TrimSpace(raw))
	multiplier := 1.0
	switch {
	case strings.HasSuffix(text, "亿"):
		multiplier = 100000000
		text = strings.TrimSuffix(text, "亿")
	case strings.HasSuffix(text, "万"):
		multiplier = 10000
		text = strings.TrimSuffix(text, "万")
	case strings.HasSuffix(text, "b"):
		multiplier = 1000000000
		text = strings.TrimSuffix(text, "b")
	case strings.HasSuffix(text, "m"):
		multiplier = 1000000
		text = strings.TrimSuffix(text, "m")
	case strings.HasSuffix(text, "k"):
		multiplier = 1000
		text = strings.TrimSuffix(text, "k")
	}
	base, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(base) || math.IsInf(base, 0) {
		return 0, errors.New("token amount must be a number such as 100000, 100k, 1M, or 1亿")
	}
	value := base * multiplier
	if value < 0 || value > 1e15 {
		return 0, errors.New("token amount is out of range")
	}
	return int(value), nil
}

func normalizeTokenOption(value string, minValue, maxValue int) (string, bool, error) {
	parsed, err := ParseTokenAmount(value)
	if err != nil {
		return "", false, err
	}
	if parsed < minValue || parsed > maxValue {
		return "", false, fmt.Errorf("value must be between %d and %d", minValue, maxValue)
	}
	return strconv.Itoa(parsed), false, nil
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

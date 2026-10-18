package assistant_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "adds v1 to bare host", input: "https://api.openai.com", want: "https://api.openai.com/v1"},
		{name: "keeps v1 and trims slash", input: "https://api.openai.com/v1/", want: "https://api.openai.com/v1"},
		{name: "keeps nested path", input: "https://gateway.example.com/openai/v1", want: "https://gateway.example.com/openai/v1"},
		{name: "allows http for self hosted", input: "http://127.0.0.1:8080", want: "http://127.0.0.1:8080/v1"},
		{name: "rejects responses endpoint", input: "https://api.openai.com/v1/responses", wantErr: true},
		{name: "rejects chat endpoint", input: "https://api.openai.com/v1/chat/completions", wantErr: true},
		{name: "rejects non http scheme", input: "ftp://api.openai.com/v1", wantErr: true},
		{name: "rejects credentials", input: "https://user:pass@api.openai.com/v1", wantErr: true},
		{name: "rejects query", input: "https://api.openai.com/v1?debug=1", wantErr: true},
		{name: "rejects fragment", input: "https://api.openai.com/v1#frag", wantErr: true},
		{name: "rejects empty", input: "   ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeBaseURL(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeChannelBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "openai bare host", input: "https://api.openai.com", want: "https://api.openai.com/v1"},
		{name: "openrouter api path", input: "https://openrouter.ai/api", want: "https://openrouter.ai/api/v1"},
		{name: "already v1", input: "https://api.openai.com/v1/", want: "https://api.openai.com/v1"},
		{name: "strips responses suffix", input: "https://api.openai.com/v1/responses", want: "https://api.openai.com/v1"},
		{name: "strips chat suffix", input: "https://api.openai.com/v1/chat/completions", want: "https://api.openai.com/v1"},
		{name: "rejects empty", input: "", wantErr: true},
		{name: "rejects non http", input: "ftp://host/v1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeChannelBaseURL(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeOptionValue(t *testing.T) {
	t.Run("empty api key keeps existing secret", func(t *testing.T) {
		normalized, skip, err := NormalizeOptionValue(OptionKeyUserAPIKey, "  ")
		require.NoError(t, err)
		assert.True(t, skip)
		assert.Empty(t, normalized)
	})

	t.Run("trims api key", func(t *testing.T) {
		normalized, skip, err := NormalizeOptionValue(OptionKeyAdminAPIKey, " sk-test ")
		require.NoError(t, err)
		assert.False(t, skip)
		assert.Equal(t, "sk-test", normalized)
	})

	t.Run("normalizes profile base url", func(t *testing.T) {
		normalized, skip, err := NormalizeOptionValue(OptionKeyUserBaseURL, "https://api.openai.com")
		require.NoError(t, err)
		assert.False(t, skip)
		assert.Equal(t, "https://api.openai.com/v1", normalized)
	})

	t.Run("allows empty model while switching source", func(t *testing.T) {
		normalized, skip, err := NormalizeOptionValue(OptionKeyUserModel, "  ")
		require.NoError(t, err)
		assert.False(t, skip)
		assert.Empty(t, normalized)
	})

	t.Run("validates source", func(t *testing.T) {
		_, _, err := NormalizeOptionValue(OptionKeyUserSource, "upstream")
		require.Error(t, err)
	})

	t.Run("validates protocol", func(t *testing.T) {
		_, _, err := NormalizeOptionValue(OptionKeyAdminProtocol, "grpc")
		require.Error(t, err)
		normalized, _, err := NormalizeOptionValue(OptionKeyAdminProtocol, "chat")
		require.NoError(t, err)
		assert.Equal(t, "chat", normalized)
	})

	t.Run("validates context window and output", func(t *testing.T) {
		_, _, err := NormalizeOptionValue(OptionKeyUserContextWindow, "100")
		require.Error(t, err)
		normalized, _, err := NormalizeOptionValue(OptionKeyUserContextWindow, "256000")
		require.NoError(t, err)
		assert.Equal(t, "256000", normalized)
		_, _, err = NormalizeOptionValue(OptionKeyUserMaxOutputTokens, "0")
		require.Error(t, err)
		normalized, _, err = NormalizeOptionValue(OptionKeyUserMaxOutputTokens, "128000")
		require.NoError(t, err)
		assert.Equal(t, "128000", normalized)
	})

	t.Run("validates channel id", func(t *testing.T) {
		_, _, err := NormalizeOptionValue(OptionKeyAdminChannelID, "-1")
		require.Error(t, err)
		normalized, _, err := NormalizeOptionValue(OptionKeyAdminChannelID, "12")
		require.NoError(t, err)
		assert.Equal(t, "12", normalized)
	})

	t.Run("validates request timeout", func(t *testing.T) {
		_, _, err := NormalizeOptionValue(OptionKeyRequestTimeout, "1")
		require.Error(t, err)
	})
}

func TestProfiles(t *testing.T) {
	setting := Get()
	original := *setting
	t.Cleanup(func() { *setting = original })

	setting.UserEnabled = true
	setting.UserSource = SourceDirect
	setting.UserProtocol = ProtocolResponses
	setting.UserBaseURL = "https://api.openai.com/v1"
	setting.UserAPIKey = "sk-user"
	setting.UserModel = "gpt-user"
	setting.UserContextWindow = 1000000
	setting.UserMaxOutputTokens = 128000
	assert.True(t, IsConfigured(ProfileUser))
	assert.True(t, IsAvailable(ProfileUser))
	assert.Equal(t, "gpt-user", ModelName(ProfileUser))

	setting.UserMaxOutputTokens = 2000000
	assert.False(t, IsConfigured(ProfileUser))
	setting.UserMaxOutputTokens = 128000

	setting.UserSource = SourceChannel
	setting.UserChannelID = 12
	setting.UserChannelModel = "gpt-channel"
	assert.True(t, IsConfigured(ProfileUser))
	assert.Equal(t, "gpt-channel", ModelName(ProfileUser))

	setting.UserChannelID = 0
	assert.False(t, IsConfigured(ProfileUser))
	assert.False(t, IsAvailable(ProfileUser))

	setting.AdminEnabled = true
	setting.AdminSource = SourceDirect
	setting.AdminProtocol = ProtocolChat
	setting.AdminBaseURL = "https://api.openai.com/v1"
	setting.AdminAPIKey = "sk-admin"
	setting.AdminModel = "gpt-admin"
	setting.AdminContextWindow = 256000
	setting.AdminMaxOutputTokens = 128000
	assert.True(t, IsAvailable(ProfileAdmin))
	assert.Equal(t, "gpt-admin", ModelName(ProfileAdmin))
	profile, ok := ProfileForOptionKey(OptionKeyAdminChannelID)
	assert.True(t, ok)
	assert.Equal(t, ProfileAdmin, profile)
}

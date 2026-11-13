package claude

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupRequestHeaderClaudeCodeHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		channelType   int
		isChannelTest bool
		compat        bool
		clientHeaders map[string]string
		wantXApp      string
		wantBeta      string
		wantUserAgent string
	}{
		{
			name:        "keeps original claude code client headers",
			channelType: constant.ChannelTypeAnthropic,
			clientHeaders: map[string]string{
				"x-app":          "cli",
				"anthropic-beta": "claude-code-20250219,context-1m-2025-08-07",
			},
			wantXApp:      "cli",
			wantBeta:      "claude-code-20250219,context-1m-2025-08-07",
			wantUserAgent: defaultUserAgent,
		},
		{
			name:          "channel test fills claude code headers",
			channelType:   constant.ChannelTypeAnthropic,
			isChannelTest: true,
			wantXApp:      "cli",
			wantBeta:      claudeCodeBeta,
			wantUserAgent: defaultUserAgent,
		},
		{
			name:        "claude code compat mode forces upstream fingerprint",
			channelType: constant.ChannelTypeAnthropic,
			compat:      true,
			clientHeaders: map[string]string{
				"user-agent":     "pi/1.5.0 (win32; node/v25.2.1; x64)",
				"anthropic-beta": "interleaved-thinking-2025-05-14",
			},
			wantXApp:      "cli",
			wantBeta:      "interleaved-thinking-2025-05-14," + claudeCodeBeta,
			wantUserAgent: defaultUserAgent,
		},
		{
			name:          "other channel types get no claude code defaults",
			channelType:   constant.ChannelTypeOpenAI,
			isChannelTest: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			ctx.Request.Header.Set("Content-Type", "application/json")
			for key, value := range test.clientHeaders {
				ctx.Request.Header.Set(key, value)
			}

			info := &relaycommon.RelayInfo{
				IsChannelTest: test.isChannelTest,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:    test.channelType,
					ApiKey:         "test-key",
					ChannelSetting: dto.ChannelSettings{ClaudeCodeCompat: test.compat},
				},
			}

			headers := http.Header{}
			adaptor := &Adaptor{}
			require.NoError(t, adaptor.SetupRequestHeader(ctx, &headers, info))

			assert.Equal(t, test.wantXApp, headers.Get("x-app"))
			assert.Equal(t, test.wantBeta, headers.Get("anthropic-beta"))
			assert.Equal(t, test.wantUserAgent, headers.Get("User-Agent"))
		})
	}
}

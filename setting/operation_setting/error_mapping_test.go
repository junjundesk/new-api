package operation_setting

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateErrorMappingRulesJSON(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "empty array",
			input:   "[]",
			wantErr: false,
		},
		{
			name: "valid status only and keyword rules",
			input: `[
				{"enabled":true,"status_code":502,"target_message":"上游不可用 请联系管理员"},
				{"enabled":true,"status_code":502,"match_message":"无效响应","target_status":503,"target_message":"服务暂时不可用"},
				{"enabled":false,"status_code":0,"match_message":"dial tcp","target_message":"上游连接失败"}
			]`,
			wantErr: false,
		},
		{
			name:    "non-array JSON",
			input:   `{"502":"msg"}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `[bad`,
			wantErr: true,
		},
		{
			name:    "missing both status code and match message",
			input:   `[{"enabled":true,"status_code":0,"match_message":"   ","target_message":"hidden"}]`,
			wantErr: true,
		},
		{
			name:    "missing target message",
			input:   `[{"enabled":true,"status_code":502,"target_message":"   "}]`,
			wantErr: true,
		},
		{
			name:    "status code 200 is rejected",
			input:   `[{"enabled":true,"status_code":200,"target_message":"hidden"}]`,
			wantErr: true,
		},
		{
			name:    "out of range status code",
			input:   `[{"enabled":true,"status_code":999,"target_message":"hidden"}]`,
			wantErr: true,
		},
		{
			name:    "out of range target status",
			input:   `[{"enabled":true,"status_code":502,"target_status":99,"target_message":"hidden"}]`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateErrorMappingRulesJSON(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMatchErrorMappingPriorityAndBehavior(t *testing.T) {
	restore := SetErrorMappingForTest(true, []ErrorMappingRule{
		{
			Enabled:       true,
			StatusCode:    http.StatusBadGateway,
			TargetMessage: "上游不可用 请联系管理员",
		},
		{
			Enabled:       true,
			StatusCode:    http.StatusBadGateway,
			MatchMessage:  "无效响应",
			TargetStatus:  http.StatusServiceUnavailable,
			TargetMessage: "上游网关异常 请联系管理员",
		},
		{
			Enabled:       false,
			StatusCode:    http.StatusTooManyRequests,
			TargetMessage: "disabled rule",
		},
		{
			Enabled:       true,
			StatusCode:    0,
			MatchMessage:  "DIAL TCP",
			TargetStatus:  http.StatusBadGateway,
			TargetMessage: "上游网络连接异常",
		},
	})
	t.Cleanup(restore)

	t.Run("exact status + keyword wins over status-only catch-all", func(t *testing.T) {
		status, msg, matched := MatchErrorMapping(502, "上游服务返回了无效响应或当前不可用。请重试。")
		require.True(t, matched)
		assert.Equal(t, http.StatusServiceUnavailable, status)
		assert.Equal(t, "上游网关异常 请联系管理员", msg)
	})

	t.Run("status-only catch-all keeps original status when target_status is 0", func(t *testing.T) {
		status, msg, matched := MatchErrorMapping(502, "bad gateway from nginx")
		require.True(t, matched)
		assert.Equal(t, http.StatusBadGateway, status)
		assert.Equal(t, "上游不可用 请联系管理员", msg)
	})

	t.Run("keyword-only rule matches case-insensitively across any error status", func(t *testing.T) {
		status, msg, matched := MatchErrorMapping(500, "Post https://upstream: dial tcp 10.0.0.1:443: i/o timeout")
		require.True(t, matched)
		assert.Equal(t, http.StatusBadGateway, status)
		assert.Equal(t, "上游网络连接异常", msg)
	})

	t.Run("disabled rule is skipped", func(t *testing.T) {
		status, msg, matched := MatchErrorMapping(429, "rate limit exceeded")
		assert.False(t, matched)
		assert.Equal(t, 429, status)
		assert.Equal(t, "rate limit exceeded", msg)
	})

	t.Run("never rewrites HTTP 200", func(t *testing.T) {
		status, msg, matched := MatchErrorMapping(200, "dial tcp")
		assert.False(t, matched)
		assert.Equal(t, 200, status)
		assert.Equal(t, "dial tcp", msg)
	})

	t.Run("global disabled switch turns off all mappings", func(t *testing.T) {
		restoreDisabled := SetErrorMappingForTest(false, []ErrorMappingRule{
			{
				Enabled:       true,
				StatusCode:    http.StatusBadGateway,
				TargetMessage: "上游不可用 请联系管理员",
			},
		})
		defer restoreDisabled()

		status, msg, matched := MatchErrorMapping(502, "上游服务返回了无效响应")
		assert.False(t, matched)
		assert.Equal(t, 502, status)
		assert.Equal(t, "上游服务返回了无效响应", msg)
	})
}

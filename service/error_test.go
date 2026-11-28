package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerTruncatesInvalidJSONBodyInLog(t *testing.T) {
	withDebugEnabled(t, false)

	body := strings.Repeat("b", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	// 非 JSON 错误体的原文现在会截断后带进客户端可见 message（便于定位上游问题），
	// 日志侧仍按 LocalLogContentLimit 截断。
	require.Equal(t, fmt.Sprintf("bad response status code 500: %s... [truncated]", strings.Repeat("b", errMessageBodyLimit)), newAPIError.Error())
	require.Contains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), fmt.Sprintf("original_length=%d", len(body)))
	require.NotContains(t, logBuffer.String(), strings.Repeat("b", common.LocalLogContentLimit+1))
}

func TestRelayErrorHandlerKeepsStructuredErrorMessage(t *testing.T) {
	message := strings.Repeat("c", common.LocalLogContentLimit+256)
	body := `{"message":"` + message + `"}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsOpenAIErrorMessage(t *testing.T) {
	message := strings.Repeat("d", common.LocalLogContentLimit+256)
	body := `{"error":{"message":"` + message + `","type":"server_error","code":"server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsInvalidJSONBodyInDebugLog(t *testing.T) {
	withDebugEnabled(t, true)

	body := strings.Repeat("e", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.NotContains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), body)
}

func TestApplyErrorMappingRewritesOpenAIAndClaudeErrors(t *testing.T) {
	restore := operation_setting.SetErrorMappingForTest(true, []operation_setting.ErrorMappingRule{
		{
			Enabled:       true,
			StatusCode:    http.StatusBadGateway,
			TargetMessage: "上游不可用 请联系管理员",
		},
		{
			Enabled:       true,
			StatusCode:    http.StatusServiceUnavailable,
			MatchMessage:  "overloaded",
			TargetStatus:  http.StatusTooManyRequests,
			TargetMessage: "当前服务繁忙 请稍后重试",
		},
	})
	t.Cleanup(restore)

	t.Run("openai protocol 502 keeps 502 and hides real reason", func(t *testing.T) {
		apiErr := types.WithOpenAIError(types.OpenAIError{
			Message: "上游服务返回了无效响应或当前不可用。请重试。",
			Type:    "upstream_error",
			Param:   "upstream_param",
			Code:    "502",
		}, http.StatusBadGateway)

		require.True(t, ApplyErrorMapping(apiErr))
		apiErr.SetMessage(common.MessageWithRequestId(apiErr.Error(), "req-502"))

		assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
		oai := apiErr.ToOpenAIError()
		assert.Equal(t, "上游不可用 请联系管理员 (request id: req-502)", oai.Message)
		assert.Empty(t, oai.Param)
		assert.NotContains(t, oai.Message, "无效响应")
	})

	t.Run("claude protocol remaps status and message", func(t *testing.T) {
		apiErr := types.WithClaudeError(types.ClaudeError{
			Type:    "overloaded_error",
			Message: "Anthropic server overloaded",
		}, http.StatusServiceUnavailable)

		require.True(t, ApplyErrorMapping(apiErr))
		assert.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
		claudeErr := apiErr.ToClaudeError()
		assert.Equal(t, "当前服务繁忙 请稍后重试", claudeErr.Message)
		assert.NotContains(t, claudeErr.Message, "Anthropic")
	})

	t.Run("task error is rewritten", func(t *testing.T) {
		taskErr := &taskdto.TaskError{
			Code:       "bad_response_status_code",
			Message:    "上游服务返回了无效响应或当前不可用。请重试。",
			StatusCode: http.StatusBadGateway,
		}
		require.True(t, ApplyTaskErrorMapping(taskErr))
		assert.Equal(t, http.StatusBadGateway, taskErr.StatusCode)
		assert.Equal(t, "上游不可用 请联系管理员", taskErr.Message)
	})
}

func withDebugEnabled(t *testing.T, enabled bool) {
	t.Helper()

	oldDebug := common.DebugEnabled
	common.DebugEnabled = enabled
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})
}

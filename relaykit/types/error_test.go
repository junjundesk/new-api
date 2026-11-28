package types

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIErrorSetMessageSyncsRelayError(t *testing.T) {
	t.Parallel()

	t.Run("openai error", func(t *testing.T) {
		t.Parallel()
		err := WithOpenAIError(OpenAIError{
			Message: "upstream bad gateway",
			Type:    "upstream_error",
			Code:    "502",
		}, http.StatusBadGateway)

		err.SetMessage("上游不可用 请联系管理员 (request id: req-1)")

		assert.Equal(t, "上游不可用 请联系管理员 (request id: req-1)", err.Error())
		assert.Equal(t, "上游不可用 请联系管理员 (request id: req-1)", err.ToOpenAIError().Message)
		assert.Equal(t, "上游不可用 请联系管理员 (request id: req-1)", err.ToClaudeError().Message)
	})

	t.Run("claude error", func(t *testing.T) {
		t.Parallel()
		err := WithClaudeError(ClaudeError{
			Type:    "overloaded_error",
			Message: "overloaded",
		}, http.StatusBadGateway)

		err.SetMessage("上游不可用 请联系管理员 (request id: req-2)")

		assert.Equal(t, "上游不可用 请联系管理员 (request id: req-2)", err.Error())
		assert.Equal(t, "上游不可用 请联系管理员 (request id: req-2)", err.ToClaudeError().Message)
		assert.Equal(t, "上游不可用 请联系管理员 (request id: req-2)", err.ToOpenAIError().Message)
	})
}

func TestNewAPIErrorReplaceError(t *testing.T) {
	t.Parallel()

	t.Run("openai error clears metadata and param and updates status", func(t *testing.T) {
		t.Parallel()
		err := WithOpenAIError(OpenAIError{
			Message:  "upstream raw detail",
			Type:     "upstream_error",
			Param:    "internal_field",
			Code:     "bad_gateway",
			Metadata: json.RawMessage(`{"provider":"secret-vendor"}`),
		}, http.StatusBadGateway)

		err.ReplaceError(http.StatusServiceUnavailable, "上游不可用 请联系管理员")

		require.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
		assert.Equal(t, "上游不可用 请联系管理员", err.Error())
		assert.Nil(t, err.Metadata)

		oai := err.ToOpenAIError()
		assert.Equal(t, "上游不可用 请联系管理员", oai.Message)
		assert.Empty(t, oai.Param)
		assert.Nil(t, oai.Metadata)

		claude := err.ToClaudeError()
		assert.Equal(t, "上游不可用 请联系管理员", claude.Message)
	})

	t.Run("keep original status when target status is zero", func(t *testing.T) {
		t.Parallel()
		err := NewErrorWithStatusCode(errors.New("raw error"), ErrorCodeDoRequestFailed, http.StatusBadGateway)

		err.ReplaceError(0, "上游不可用 请联系管理员")

		assert.Equal(t, http.StatusBadGateway, err.StatusCode)
		assert.Equal(t, "上游不可用 请联系管理员", err.ToOpenAIError().Message)
		assert.Equal(t, "上游不可用 请联系管理员", err.ToClaudeError().Message)
	})
}

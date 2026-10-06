package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newTokenMappingContext(t *testing.T, path string, body string, mappings []setting.TokenModelMapping) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	if mappings != nil {
		common.SetContextKey(c, constant.ContextKeyTokenModelMappings, mappings)
	}
	return c
}

func readCachedBody(t *testing.T, c *gin.Context) []byte {
	t.Helper()
	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	body, err := storage.Bytes()
	require.NoError(t, err)
	return body
}

func TestApplyTokenModelMapping(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		mappings   []setting.TokenModelMapping
		model      string
		wantModel  string
		effortPath string
		wantEffort string
	}{
		{
			name:      "no mappings keeps request untouched",
			path:      "/v1/chat/completions",
			body:      `{"model":"gpt-6-luna"}`,
			model:     "gpt-6-luna",
			wantModel: "gpt-6-luna",
		},
		{
			name:      "exact match rewrites model and keeps client effort",
			path:      "/v1/chat/completions",
			body:      `{"model":"gpt-6-luna","reasoning_effort":"high"}`,
			mappings:  []setting.TokenModelMapping{{SourceModel: "gpt-6-luna", TargetModel: "gpt-6-sol"}},
			model:     "gpt-6-luna",
			wantModel: "gpt-6-sol", effortPath: "reasoning_effort", wantEffort: "high",
		},
		{
			name:      "mapping runs one hop only",
			path:      "/v1/chat/completions",
			body:      `{"model":"a"}`,
			mappings:  []setting.TokenModelMapping{{SourceModel: "a", TargetModel: "b"}, {SourceModel: "b", TargetModel: "c"}},
			model:     "a",
			wantModel: "b",
		},
		{
			name:      "no prefix or wildcard matching",
			path:      "/v1/chat/completions",
			body:      `{"model":"gpt-6-luna-high"}`,
			mappings:  []setting.TokenModelMapping{{SourceModel: "gpt-6-luna", TargetModel: "gpt-6-sol"}},
			model:     "gpt-6-luna-high",
			wantModel: "gpt-6-luna-high",
		},
		{
			name:      "chat completions effort override",
			path:      "/v1/chat/completions",
			body:      `{"model":"gpt-6-luna","reasoning_effort":"high"}`,
			mappings:  []setting.TokenModelMapping{{SourceModel: "gpt-6-luna", TargetModel: "gpt-6-sol", ReasoningEffort: "low"}},
			model:     "gpt-6-luna",
			wantModel: "gpt-6-sol", effortPath: "reasoning_effort", wantEffort: "low",
		},
		{
			name:      "responses effort override",
			path:      "/v1/responses",
			body:      `{"model":"gpt-6-luna"}`,
			mappings:  []setting.TokenModelMapping{{SourceModel: "gpt-6-luna", TargetModel: "gpt-6-luna", ReasoningEffort: "xhigh"}},
			model:     "gpt-6-luna",
			wantModel: "gpt-6-luna", effortPath: "reasoning.effort", wantEffort: "xhigh",
		},
		{
			name:      "claude messages effort override",
			path:      "/v1/messages",
			body:      `{"model":"claude-a"}`,
			mappings:  []setting.TokenModelMapping{{SourceModel: "claude-a", TargetModel: "claude-b", ReasoningEffort: "max"}},
			model:     "claude-a",
			wantModel: "claude-b", effortPath: "output_config.effort", wantEffort: "max",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTokenMappingContext(t, tt.path, tt.body, tt.mappings)
			modelRequest := &ModelRequest{Model: tt.model}
			require.NoError(t, applyTokenModelMapping(c, modelRequest))
			assert.Equal(t, tt.wantModel, modelRequest.Model)

			body := readCachedBody(t, c)
			assert.Equal(t, tt.wantModel, gjson.GetBytes(body, "model").String())
			if tt.effortPath != "" {
				assert.Equal(t, tt.wantEffort, gjson.GetBytes(body, tt.effortPath).String())
			}
		})
	}
}

package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/assistant_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAssistantModelsUsesUnsavedDirectProfile(t *testing.T) {
	setupAssistantOptionsTest(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "Bearer sk-live", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"gpt-b"},{"id":"gpt-a"}]}`)
	}))
	defer upstream.Close()

	body, err := common.Marshal(map[string]any{
		"profile":  assistant_setting.ProfileUser,
		"source":   assistant_setting.SourceDirect,
		"base_url": upstream.URL,
		"api_key":  "sk-live",
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/assistant/admin/models", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	ListAssistantModels(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	payload := struct {
		Success bool `json:"success"`
		Data    struct {
			Models []string `json:"models"`
		} `json:"data"`
	}{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.Equal(t, []string{"gpt-a", "gpt-b"}, payload.Data.Models)
}

func TestListAssistantModelsRejectsInvalidProfile(t *testing.T) {
	setupAssistantOptionsTest(t)
	body, err := common.Marshal(map[string]any{"profile": "guest"})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/assistant/admin/models", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	ListAssistantModels(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	payload := map[string]any{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, false, payload["success"])
}

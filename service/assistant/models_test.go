package assistant

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/assistant_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModelListFormats(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "data entries", body: `{"data":[{"id":"b"},{"id":"a"},{"id":"b"}]}`, want: []string{"a", "b"}},
		{name: "model entries", body: `{"models":[{"id":"m2"},{"id":"m1"}]}`, want: []string{"m1", "m2"}},
		{name: "string models", body: `{"models":["z","x"]}`, want: []string{"x", "z"}},
		{name: "array entries", body: `[{"id":"k"},{"id":"j"}]`, want: []string{"j", "k"}},
		{name: "string array", body: `["n","m"]`, want: []string{"m", "n"}},
		{name: "empty", body: `{}`, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseModelList([]byte(tt.body)))
		})
	}
}

func TestListModelsFetchesDirectProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "Bearer sk-models", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"gpt-b"},{"id":"gpt-a"},{"id":"gpt-b"}]}`)
	}))
	defer server.Close()

	models, err := ListModels(context.Background(), assistant_setting.Profile{
		Source:  assistant_setting.SourceDirect,
		BaseURL: server.URL,
		APIKey:  "sk-models",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-a", "gpt-b"}, models)
}

func TestListModelsRedactsAPIKeyFromUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "invalid key sk-secret")
	}))
	defer server.Close()

	_, err := ListModels(context.Background(), assistant_setting.Profile{
		Source:  assistant_setting.SourceDirect,
		BaseURL: server.URL,
		APIKey:  "sk-secret",
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "sk-secret")
	assert.Contains(t, err.Error(), "[REDACTED]")
}

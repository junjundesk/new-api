package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiskBackedRouteFieldsPreserveValidationAndReplay(t *testing.T) {
	prior := common.GetDiskCacheConfig()
	common.SetDiskCacheConfig(common.DiskCacheConfig{Enabled: true, ThresholdMB: 0, MaxSizeMB: 16, Path: t.TempDir()})
	t.Cleanup(func() { common.SetDiskCacheConfig(prior) })
	for _, tc := range []struct {
		name, body, model, group, wantErr string
	}{
		{"first exact top-level key", `{"messages":[{"model":"nested"}],"Model":"case-sensitive","model":"first","model":"second","group":"a\u0031"}`, "first", "a1", ""},
		{"null and missing", `{"model":null}`, "", "", ""},
		{"invalid type", `{"model":"ok","group":false}`, "", "", "field group must be a string"},
		{"trailing document rejected", `{"model":"ok"} {}`, "", "", "invalid JSON request body"},
		{"late syntax error", `{"model":"ok","messages":[}`, "", "", "invalid JSON request body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tc.body))
			storage, err := common.GetBodyStorage(c)
			require.NoError(t, err)
			require.True(t, storage.IsDisk())
			t.Cleanup(func() { common.CleanupBodyStorage(c) })
			fields, err := getModelFromJSONBody(c)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			payload, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.body, string(payload))
			replay, err := storage.NewReader()
			require.NoError(t, err)
			payload, err = io.ReadAll(replay)
			require.NoError(t, err)
			require.NoError(t, replay.Close())
			assert.Equal(t, tc.body, string(payload))
			require.NoError(t, storage.Close())
			assert.Equal(t, tc.model, fields.Model)
			assert.Equal(t, tc.group, fields.Group)
		})
	}
}

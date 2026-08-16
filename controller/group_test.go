package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGroupsIncludesRatios(t *testing.T) {
	originalRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1.5}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
	})

	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/group/", nil)

	GetGroups(context)

	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool               `json:"success"`
		Data    []string           `json:"data"`
		Ratios  map[string]float64 `json:"ratios"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.ElementsMatch(t, []string{"default", "vip"}, payload.Data)
	assert.InDelta(t, 1, payload.Ratios["default"], 1e-9)
	assert.InDelta(t, 1.5, payload.Ratios["vip"], 1e-9)
}

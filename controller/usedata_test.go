package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type quotaDatesResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    []model.QuotaData `json:"data"`
}

type userBreakdownResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Rows      []model.UserQuotaBreakdownRow `json:"rows"`
		Truncated bool                          `json:"truncated"`
	} `json:"data"`
}

func setupQuotaDataControllerTestDB(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaData{}))
	for _, row := range []model.QuotaData{
		{UserID: 1, Username: "alice", UseGroup: "vip", ModelName: "gpt-a", CreatedAt: 3600, Count: 2, Quota: 100, TokenUsed: 40},
		{UserID: 10, Username: "admin", UseGroup: "default", ModelName: "gpt-a", CreatedAt: 3600, Count: 1, Quota: 70, TokenUsed: 30},
		{UserID: 2, Username: "bob", UseGroup: "default", ModelName: "gpt-b", CreatedAt: 3600, Count: 1, Quota: 25, TokenUsed: 10},
	} {
		row := row
		require.NoError(t, db.Create(&row).Error)
	}
}

func TestGetAllQuotaDatesExcludesUsersFromStats(t *testing.T) {
	setupQuotaDataControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data?start_timestamp=0&end_timestamp=7200&exclude_user_ids=10", nil)

	GetAllQuotaDates(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload quotaDatesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	require.Len(t, payload.Data, 2)

	quotaByModel := make(map[string]int, len(payload.Data))
	for _, row := range payload.Data {
		quotaByModel[row.ModelName] = row.Quota
	}
	require.Equal(t, 100, quotaByModel["gpt-a"])
	require.Equal(t, 25, quotaByModel["gpt-b"])
}

func TestGetAllQuotaDatesRejectsMalformedExclusions(t *testing.T) {
	setupQuotaDataControllerTestDB(t)

	for _, raw := range []string{"abc", "1,-2", "0"} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data?start_timestamp=0&end_timestamp=7200&exclude_user_ids="+raw, nil)

		GetAllQuotaDates(ctx)

		var payload quotaDatesResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		require.False(t, payload.Success)
		require.Equal(t, "invalid exclude_user_ids", payload.Message)
	}
}

func TestGetUserQuotaBreakdownExcludesUsers(t *testing.T) {
	setupQuotaDataControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", common.RoleAdminUser)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/user-breakdown?exclude_user_ids=10", nil)

	GetUserQuotaBreakdown(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload userBreakdownResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	require.Len(t, payload.Data.Rows, 2)
	require.Equal(t, "alice", payload.Data.Rows[0].Username)
	require.Equal(t, "bob", payload.Data.Rows[1].Username)
}

package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 管理端活动码接口：请求体只能被读取一次，创建与编辑都必须成功。
// 该用例保护的是真实回归：曾经 buildActivityCode 内部二次 ShouldBindJSON，
// 导致编辑（先绑定一次再进 buildActivityCode）永远拿到 EOF 并返回无效参数。
func setupActivityControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ActivityCode{}, &model.ActivityGrant{}, &model.Log{}))
	model.DB, model.LOG_DB = db, db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})

	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"activity-test-group":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
	return db
}

func newActivityAdminRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminContext := func(c *gin.Context) {
		c.Set("id", 7)
		c.Set("role", common.RoleRootUser)
	}
	router.POST("/api/activity/", adminContext, AddActivityCode)
	router.PUT("/api/activity/", adminContext, UpdateActivityCode)
	return router
}

type activityAPIResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

func activityRequestPayload() map[string]interface{} {
	return map[string]interface{}{
		"code":           "hdmfree",
		"name":           "Free activity",
		"description":    "",
		"group_name":     "activity-test-group",
		"duration_hours": 24,
		"rpm_limit":      5,
		"price_quota":    0,
		"ratio_mode":     "override",
		"ratio_override": 0,
		"start_time":     0,
		"end_time":       0,
		"total_limit":    0,
		"per_user_limit": 1,
		"public_visible": 1,
	}
}

func callActivityAdmin(t *testing.T, router *gin.Engine, method string, payload map[string]interface{}) activityAPIResponse {
	t.Helper()
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, "/api/activity/", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	response := activityAPIResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestActivityCodeCreateThenUpdateSucceeds(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	created := callActivityAdmin(t, router, http.MethodPost, activityRequestPayload())
	require.True(t, created.Success, "create must succeed: "+created.Message)
	require.NotNil(t, created.Data)
	codeId, ok := created.Data["id"].(float64)
	require.True(t, ok, "created response must include the activity code id")
	activityCodeId := int(codeId)

	updatePayload := activityRequestPayload()
	updatePayload["id"] = activityCodeId
	updatePayload["rpm_limit"] = 9
	updatePayload["name"] = "Free activity updated"

	updated := callActivityAdmin(t, router, http.MethodPut, updatePayload)
	require.True(t, updated.Success, "update must succeed: "+updated.Message)

	stored, err := model.GetActivityCodeById(activityCodeId)
	require.NoError(t, err)
	assert.Equal(t, 9, stored.RpmLimit, "updated RPM must be persisted")
	assert.Equal(t, "Free activity updated", stored.Name)
	assert.Equal(t, "hdmfree", stored.Code, "activity code string must stay unchanged")
}

func TestActivityCodeUpdateRejectsMalformedPayload(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/activity/", bytes.NewReader([]byte("{not json")))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	response := activityAPIResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success, "malformed payload must be rejected")
}

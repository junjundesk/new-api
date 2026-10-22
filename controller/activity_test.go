package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
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

func TestActivityCodeSupportsChineseCode(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	payload := activityRequestPayload()
	payload["code"] = "夏日免费"
	payload["name"] = "夏日免费活动"

	created := callActivityAdmin(t, router, http.MethodPost, payload)
	require.True(t, created.Success, "creating a Chinese activity code must succeed: "+created.Message)
	codeIdValue, ok := created.Data["id"].(float64)
	require.True(t, ok)
	activityCodeId := int(codeIdValue)

	updatePayload := activityRequestPayload()
	updatePayload["id"] = activityCodeId
	updatePayload["code"] = "夏日免费"
	updatePayload["name"] = "夏日免费活动-已改名"
	updated := callActivityAdmin(t, router, http.MethodPut, updatePayload)
	require.True(t, updated.Success, "updating a Chinese activity code must succeed: "+updated.Message)

	stored, err := model.GetActivityCodeById(activityCodeId)
	require.NoError(t, err)
	assert.Equal(t, "夏日免费", stored.Code)
	assert.Equal(t, "夏日免费活动-已改名", stored.Name)
}

func TestActivityCodeRejectsIllegalCharacters(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	payload := activityRequestPayload()
	payload["code"] = "夏日 免费"
	rejected := callActivityAdmin(t, router, http.MethodPost, payload)
	assert.False(t, rejected.Success, "codes containing spaces must be rejected")
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
func activityModelsRequestPayload() map[string]interface{} {
	payload := activityRequestPayload()
	payload["code"] = "hdmmodels"
	payload["name"] = "Models activity"
	payload["ratio_scope"] = "models"
	payload["ratio_mode"] = "inherit"
	payload["ratio_override"] = 0
	payload["model_ratio_overrides"] = map[string]interface{}{"gpt-5": 0, "o3": 0.5}
	return payload
}

func tooManyModelRatios() map[string]interface{} {
	ratios := make(map[string]interface{}, model.ActivityCodeMaxModelRatioOverrides+1)
	for i := 0; i <= model.ActivityCodeMaxModelRatioOverrides; i++ {
		ratios[fmt.Sprintf("model-%d", i)] = 0.5
	}
	return ratios
}

// 指定模型类型：模型倍率落库并回显；切回整个分组时模型倍率被清空。
func TestActivityCodeModelsScopePersistsModelRatios(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	created := callActivityAdmin(t, router, http.MethodPost, activityModelsRequestPayload())
	require.True(t, created.Success, "creating a models-scope activity code must succeed: "+created.Message)
	codeIdValue, ok := created.Data["id"].(float64)
	require.True(t, ok)
	activityCodeId := int(codeIdValue)

	stored, err := model.GetActivityCodeById(activityCodeId)
	require.NoError(t, err)
	assert.Equal(t, model.ActivityRatioScopeModels, stored.RatioScope)
	assert.Equal(t, model.ActivityRatioModeInherit, stored.RatioMode, "指定模型类型强制继承分组级倍率")
	assert.Equal(t, float64(0), stored.RatioOverride)
	assert.Equal(t, map[string]float64{"gpt-5": 0, "o3": 0.5}, map[string]float64(stored.ModelRatioOverrides))
	assert.Equal(t, "models", created.Data["ratio_scope"])
	assert.Equal(t, map[string]interface{}{"gpt-5": float64(0), "o3": 0.5}, created.Data["model_ratio_overrides"])

	updatePayload := activityRequestPayload()
	updatePayload["id"] = activityCodeId
	updatePayload["ratio_scope"] = "group"
	updatePayload["ratio_mode"] = "override"
	updatePayload["ratio_override"] = 0.5
	updated := callActivityAdmin(t, router, http.MethodPut, updatePayload)
	require.True(t, updated.Success, "switching back to the whole group must succeed: "+updated.Message)

	stored, err = model.GetActivityCodeById(activityCodeId)
	require.NoError(t, err)
	assert.Equal(t, model.ActivityRatioScopeGroup, stored.RatioScope)
	assert.Equal(t, 0.5, stored.RatioOverride)
	assert.Empty(t, stored.ModelRatioOverrides, "切回整个分组时模型倍率被清空")
}

func TestActivityCodeModelsScopeRejectsInvalidPayloads(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	tests := []struct {
		name   string
		mutate func(payload map[string]interface{})
	}{
		{name: "empty model list", mutate: func(payload map[string]interface{}) {
			payload["model_ratio_overrides"] = map[string]interface{}{}
		}},
		{name: "missing model list", mutate: func(payload map[string]interface{}) {
			delete(payload, "model_ratio_overrides")
		}},
		{name: "blank model name", mutate: func(payload map[string]interface{}) {
			payload["model_ratio_overrides"] = map[string]interface{}{"   ": 0.5}
		}},
		{name: "over-long model name", mutate: func(payload map[string]interface{}) {
			payload["model_ratio_overrides"] = map[string]interface{}{strings.Repeat("m", model.ActivityModelNameMaxLength+1): 0.5}
		}},
		{name: "null ratio", mutate: func(payload map[string]interface{}) {
			payload["model_ratio_overrides"] = map[string]interface{}{"gpt-5": nil}
		}},
		{name: "negative ratio", mutate: func(payload map[string]interface{}) {
			payload["model_ratio_overrides"] = map[string]interface{}{"gpt-5": -1}
		}},
		{name: "ratio above the maximum", mutate: func(payload map[string]interface{}) {
			payload["model_ratio_overrides"] = map[string]interface{}{"gpt-5": dto.MaxChannelModelGroupRatio + 1}
		}},
		{name: "too many entries", mutate: func(payload map[string]interface{}) {
			payload["model_ratio_overrides"] = tooManyModelRatios()
		}},
		{name: "names colliding after trimming", mutate: func(payload map[string]interface{}) {
			payload["model_ratio_overrides"] = map[string]interface{}{"gpt-5": 0.5, " gpt-5 ": 0}
		}},
		{name: "invalid ratio scope", mutate: func(payload map[string]interface{}) {
			payload["ratio_scope"] = "group-and-models"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := activityModelsRequestPayload()
			tt.mutate(payload)
			response := callActivityAdmin(t, router, http.MethodPost, payload)
			assert.False(t, response.Success, "非法配置必须被拒绝")
		})
	}
}

// 分组内在架模型随时会变，活动码允许先配置尚未上架的模型名，并去掉首尾空白。
func TestActivityCodeModelsScopeAcceptsUnknownModelAndTrimsNames(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	payload := activityModelsRequestPayload()
	payload["model_ratio_overrides"] = map[string]interface{}{"  not-enabled-yet  ": 0.25}
	created := callActivityAdmin(t, router, http.MethodPost, payload)
	require.True(t, created.Success, created.Message)

	codeIdValue, ok := created.Data["id"].(float64)
	require.True(t, ok)
	stored, err := model.GetActivityCodeById(int(codeIdValue))
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"not-enabled-yet": 0.25}, map[string]float64(stored.ModelRatioOverrides))
}

func newActivityGrantRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminContext := func(c *gin.Context) {
		c.Set("id", 7)
		c.Set("role", common.RoleRootUser)
	}
	router.POST("/api/activity/:id/grants", adminContext, GrantActivityCode)
	return router
}

func TestGrantActivityCodeGrantsEveryUserWithoutCharging(t *testing.T) {
	db := setupActivityControllerTest(t)
	router := newActivityGrantRouter()

	code := &model.ActivityCode{
		Code:            "hcmgrant",
		Name:            "Grant activity",
		GroupName:       "activity-test-group",
		DurationSeconds: 3600,
		Status:          model.ActivityCodeStatusEnabled,
		RatioMode:       model.ActivityRatioModeInherit,
		PerUserLimit:    1,
	}
	require.NoError(t, db.Create(code).Error)

	first := &model.User{Username: "grant-one", Password: "password", Status: common.UserStatusEnabled, Quota: 100, AffCode: "gr01"}
	second := &model.User{Username: "grant-two", Password: "password", Status: common.UserStatusEnabled, Quota: 100, AffCode: "gr02"}
	require.NoError(t, db.Create(first).Error)
	require.NoError(t, db.Create(second).Error)

	payload, err := common.Marshal(map[string]interface{}{
		"user_ids": []int{first.Id, second.Id, first.Id},
		"remark":   "campaign",
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/activity/%d/grants", code.Id), bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	response := struct {
		Success bool `json:"success"`
		Data    struct {
			Granted int `json:"granted"`
			Failed  int `json:"failed"`
			Total   int `json:"total"`
		} `json:"data"`
	}{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, 2, response.Data.Granted, "重复用户 ID 去重后按两人赠送")
	assert.Equal(t, 0, response.Data.Failed)
	assert.Equal(t, 2, response.Data.Total)

	var grants []model.ActivityGrant
	require.NoError(t, db.Where("code_id = ?", code.Id).Find(&grants).Error)
	require.Len(t, grants, 2)
	for _, grant := range grants {
		assert.Equal(t, model.ActivityGrantSourceAdmin, grant.Source)
		assert.Equal(t, 0, grant.PriceQuota, "赠送不扣余额")
		assert.Equal(t, "campaign", grant.Remark)
	}
	var stored model.ActivityCode
	require.NoError(t, db.First(&stored, "id = ?", code.Id).Error)
	assert.Equal(t, 0, stored.UsedCount, "赠送不占兑换次数")

	for _, id := range []int{first.Id, second.Id} {
		var user model.User
		require.NoError(t, db.First(&user, "id = ?", id).Error)
		assert.Equal(t, 100, user.Quota, "赠送后余额不变")
	}
}

func TestGrantActivityCodeRejectsEmptyUserList(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityGrantRouter()

	payload, err := common.Marshal(map[string]interface{}{"user_ids": []int{}})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/activity/1/grants", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	response := activityAPIResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
}

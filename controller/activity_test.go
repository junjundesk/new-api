package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ActivityCode{}, &model.ActivityGrant{}, &model.UserBonusGrant{}, &model.Log{}))
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

// 用户端活动中心：售罄与活动到期不再隐藏卡片，只标记不可兑换。
func newActivityUserRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	userContext := func(c *gin.Context) {
		c.Set("id", 11)
	}
	router.GET("/api/activity/offers", userContext, GetActivityOffers)
	router.POST("/api/activity/verify", userContext, VerifyActivityCode)
	return router
}

type activityOfferView struct {
	Code          string `json:"code"`
	Redeemable    bool   `json:"redeemable"`
	BlockedReason string `json:"blocked_reason"`
	SortWeight    int    `json:"sort_weight"`
	ImageUrl      string `json:"image_url"`
	ActiveEndTime int64  `json:"active_end_time"`
}

type activityOffersResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Enabled bool                `json:"enabled"`
		Offers  []activityOfferView `json:"offers"`
	} `json:"data"`
}

func callActivityOffers(t *testing.T, router *gin.Engine) activityOffersResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/activity/offers", nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	response := activityOffersResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response
}

func TestActivityOffersKeepSoldOutAndEndedCards(t *testing.T) {
	db := setupActivityControllerTest(t)
	router := newActivityUserRouter()
	now := common.GetTimestamp()

	base := model.ActivityCode{
		Name:          "offer",
		GroupName:     "activity-test-group",
		Status:        model.ActivityCodeStatusEnabled,
		PublicVisible: 1,
	}
	codes := []model.ActivityCode{}
	for _, mutate := range []func(code *model.ActivityCode){
		func(code *model.ActivityCode) {
			code.Code, code.Name, code.SortWeight, code.ActiveEndTime = "hdmendedact", "已结束", 10, now-1
		},
		func(code *model.ActivityCode) {
			code.Code, code.Name, code.SortWeight, code.TotalLimit, code.UsedCount = "hdmsold", "已抢光", 5, 2, 2
		},
		func(code *model.ActivityCode) {
			code.Code, code.Name, code.ImageUrl = "hdmopen", "可兑换", "https://cdn.example.com/card.png"
		},
		func(code *model.ActivityCode) {
			code.Code, code.Name, code.PublicVisible = "hdmhidden", "未公开", 0
		},
		func(code *model.ActivityCode) {
			code.Code, code.Name, code.StartTime = "hdmfuture", "未开始", now+3600
		},
		func(code *model.ActivityCode) {
			code.Code, code.Name, code.EndTime = "hdmclosed", "兑换已截止", now-1
		},
	} {
		code := base
		mutate(&code)
		require.NoError(t, db.Create(&code).Error)
		codes = append(codes, code)
	}

	response := callActivityOffers(t, router)
	ordered := make([]string, 0, len(response.Data.Offers))
	for _, offer := range response.Data.Offers {
		ordered = append(ordered, offer.Code)
	}
	assert.Equal(t, []string{"hdmendedact", "hdmsold", "hdmopen"}, ordered,
		"售罄与活动到期仍然展示；未公开、未开始、兑换截止已过的活动继续隐藏；排序按 sort_weight 倒序")

	assert.False(t, response.Data.Offers[0].Redeemable)
	assert.Equal(t, "activity_ended", response.Data.Offers[0].BlockedReason)
	assert.Equal(t, 10, response.Data.Offers[0].SortWeight)
	assert.Equal(t, codes[0].ActiveEndTime, response.Data.Offers[0].ActiveEndTime)

	assert.False(t, response.Data.Offers[1].Redeemable)
	assert.Equal(t, "sold_out", response.Data.Offers[1].BlockedReason)

	assert.True(t, response.Data.Offers[2].Redeemable)
	assert.Empty(t, response.Data.Offers[2].BlockedReason)
	assert.Equal(t, "https://cdn.example.com/card.png", response.Data.Offers[2].ImageUrl)
}

func TestActivityPreviewReportsEndedActivityBeforeSoldOut(t *testing.T) {
	db := setupActivityControllerTest(t)
	router := newActivityUserRouter()
	// 预览会按 blocked_reason 翻译提示文案，测试里初始化一次本地化资源。
	require.NoError(t, i18n.Init())
	now := common.GetTimestamp()

	user := &model.User{Id: 11, Username: "activity-preview", Password: "password", Status: common.UserStatusEnabled, Quota: 100}
	require.NoError(t, db.Create(user).Error)
	code := &model.ActivityCode{
		Code:          "hdmpreview",
		Name:          "预览",
		GroupName:     "activity-test-group",
		Status:        model.ActivityCodeStatusEnabled,
		PublicVisible: 1,
		TotalLimit:    1,
		UsedCount:     1,
		ActiveEndTime: now - 1,
		SortWeight:    7,
		ImageUrl:      "/uploads/activity.png",
	}
	require.NoError(t, db.Create(code).Error)

	body, err := common.Marshal(map[string]string{"code": "hdmpreview"})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/activity/verify", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	response := activityAPIResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)
	assert.Equal(t, false, response.Data["redeemable"])
	assert.Equal(t, "activity_ended", response.Data["blocked_reason"], "活动到期优先于售罄")
	assert.Equal(t, float64(7), response.Data["sort_weight"])
	assert.Equal(t, "/uploads/activity.png", response.Data["image_url"])
	assert.Equal(t, float64(code.ActiveEndTime), response.Data["active_end_time"])
}

func TestActivityCodeRejectsInvalidSortWeightAndImageUrl(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()
	now := common.GetTimestamp()

	tests := []struct {
		name   string
		mutate func(payload map[string]interface{})
	}{
		{name: "negative sort weight", mutate: func(payload map[string]interface{}) {
			payload["sort_weight"] = -1
		}},
		{name: "sort weight above the maximum", mutate: func(payload map[string]interface{}) {
			payload["sort_weight"] = model.ActivityCodeMaxSortWeight + 1
		}},
		{name: "image url without scheme or site path", mutate: func(payload map[string]interface{}) {
			payload["image_url"] = "cdn.example.com/card.png"
		}},
		{name: "image url containing a space", mutate: func(payload map[string]interface{}) {
			payload["image_url"] = "https://cdn.example.com/card .png"
		}},
		{name: "image url containing a control character", mutate: func(payload map[string]interface{}) {
			payload["image_url"] = "https://cdn.example.com/card.png\nX-Injected: 1"
		}},
		{name: "image url longer than the column", mutate: func(payload map[string]interface{}) {
			payload["image_url"] = "https://cdn.example.com/" + strings.Repeat("a", model.ActivityCodeImageUrlMaxLength)
		}},
		{name: "negative active end time", mutate: func(payload map[string]interface{}) {
			payload["active_end_time"] = -1
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := activityRequestPayload()
			tt.mutate(payload)
			response := callActivityAdmin(t, router, http.MethodPost, payload)
			assert.False(t, response.Success, "非法配置必须被拒绝")
		})
	}

	payload := activityRequestPayload()
	payload["code"] = "hdmvalid"
	payload["sort_weight"] = 1234
	payload["image_url"] = "  https://cdn.example.com/card.png  "
	payload["active_end_time"] = now + 3600
	created := callActivityAdmin(t, router, http.MethodPost, payload)
	require.True(t, created.Success, created.Message)
	codeIdValue, ok := created.Data["id"].(float64)
	require.True(t, ok)

	stored, err := model.GetActivityCodeById(int(codeIdValue))
	require.NoError(t, err)
	assert.Equal(t, 1234, stored.SortWeight)
	assert.Equal(t, "https://cdn.example.com/card.png", stored.ImageUrl, "保存前去掉首尾空白")
	assert.Equal(t, now+3600, stored.ActiveEndTime)
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

// 活动到期后赠送整批拒绝：否则会写出一条立刻失效的授权，用户端只见 0 天可用。
func TestGrantActivityCodeRejectsEndedActivity(t *testing.T) {
	db := setupActivityControllerTest(t)
	router := newActivityGrantRouter()
	now := common.GetTimestamp()

	code := &model.ActivityCode{
		Code:            "hcmended",
		Name:            "已结束赠送",
		GroupName:       "activity-test-group",
		DurationSeconds: 3600,
		Status:          model.ActivityCodeStatusEnabled,
		RatioMode:       model.ActivityRatioModeInherit,
		PerUserLimit:    1,
		ActiveEndTime:   now - 1,
	}
	require.NoError(t, db.Create(code).Error)
	user := &model.User{Username: "grant-ended", Password: "password", Status: common.UserStatusEnabled, Quota: 100, AffCode: "gr99"}
	require.NoError(t, db.Create(user).Error)

	payload, err := common.Marshal(map[string]interface{}{"user_ids": []int{user.Id}})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/activity/%d/grants", code.Id), bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	response := activityAPIResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success, "活动到期后赠送必须被拒绝")

	var count int64
	require.NoError(t, db.Model(&model.ActivityGrant{}).Count(&count).Error)
	assert.Zero(t, count, "拒绝赠送时不写授权")
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

func activityBenefitRequestPayload() map[string]interface{} {
	payload := activityRequestPayload()
	payload["type"] = "benefit"
	payload["code"] = "hdmbenefit"
	return payload
}

func benefitItems(items ...map[string]interface{}) []map[string]interface{} {
	return items
}

func TestActivityCodeBenefitCreateAndUpdate(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	payload := activityBenefitRequestPayload()
	payload["benefits"] = benefitItems(
		map[string]interface{}{"key": "", "group_name": "activity-test-group", "scope": "group", "rpm_limit": 10, "token_total": 10000000},
		map[string]interface{}{"key": "", "group_name": "default", "scope": "model", "model_name": "gpt-5", "rpm_limit": 0, "token_total": 0},
	)
	created := callActivityAdmin(t, router, http.MethodPost, payload)
	require.True(t, created.Success, created.Message)
	codeIdValue, ok := created.Data["id"].(float64)
	require.True(t, ok)
	codeId := int(codeIdValue)

	stored, err := model.GetActivityCodeById(codeId)
	require.NoError(t, err)
	assert.Equal(t, model.ActivityCodeTypeBenefit, stored.Type)
	require.Len(t, stored.Benefits, 2)
	assert.NotEmpty(t, stored.Benefits[0].Key)
	assert.NotEmpty(t, stored.Benefits[1].Key)
	assert.Equal(t, "activity-test-group", stored.GroupName)
	assert.Equal(t, int64(10000000), stored.Benefits[0].TokenTotal)

	update := activityBenefitRequestPayload()
	update["id"] = codeId
	update["benefits"] = benefitItems(
		map[string]interface{}{"key": stored.Benefits[0].Key, "group_name": "activity-test-group", "scope": "group", "rpm_limit": 20, "token_total": 500},
		map[string]interface{}{"key": "", "group_name": "default", "scope": "group", "rpm_limit": 0, "token_total": 0},
	)
	updated := callActivityAdmin(t, router, http.MethodPut, update)
	require.True(t, updated.Success, updated.Message)

	stored, err = model.GetActivityCodeById(codeId)
	require.NoError(t, err)
	require.Len(t, stored.Benefits, 2)
	assert.Equal(t, update["benefits"].([]map[string]interface{})[0]["key"], stored.Benefits[0].Key, "已有权益 key 保留")
	assert.Equal(t, 20, stored.Benefits[0].RpmLimit)
	assert.Equal(t, int64(500), stored.Benefits[0].TokenTotal)
	assert.NotEqual(t, stored.Benefits[0].Key, stored.Benefits[1].Key)
}

func TestActivityCodeRejectsBenefitTypeChange(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	payload := activityBenefitRequestPayload()
	payload["benefits"] = benefitItems(
		map[string]interface{}{"group_name": "activity-test-group", "scope": "group"},
	)
	created := callActivityAdmin(t, router, http.MethodPost, payload)
	require.True(t, created.Success, created.Message)
	codeIdValue := created.Data["id"].(float64)

	update := activityBenefitRequestPayload()
	update["id"] = int(codeIdValue)
	update["type"] = "ratio"
	update["benefits"] = benefitItems()
	rejected := callActivityAdmin(t, router, http.MethodPut, update)
	assert.False(t, rejected.Success, "活动码类型创建后不可修改")
}

func TestActivityCodeBenefitRejectsInvalidPayloads(t *testing.T) {
	setupActivityControllerTest(t)
	router := newActivityAdminRouter()

	validBenefit := func() map[string]interface{} {
		return map[string]interface{}{"group_name": "activity-test-group", "scope": "group"}
	}
	tooMany := make([]map[string]interface{}, model.ActivityCodeMaxBenefits+1)
	for i := range tooMany {
		tooMany[i] = map[string]interface{}{"group_name": "activity-test-group", "scope": "group"}
	}

	tests := []struct {
		name   string
		mutate func(payload map[string]interface{})
	}{
		{name: "empty benefits", mutate: func(payload map[string]interface{}) {
			payload["benefits"] = benefitItems()
		}},
		{name: "duplicate group scope model", mutate: func(payload map[string]interface{}) {
			payload["benefits"] = benefitItems(validBenefit(), validBenefit())
		}},
		{name: "invalid scope", mutate: func(payload map[string]interface{}) {
			item := validBenefit()
			item["scope"] = "both"
			payload["benefits"] = benefitItems(item)
		}},
		{name: "model scope without model", mutate: func(payload map[string]interface{}) {
			item := validBenefit()
			item["scope"] = "model"
			payload["benefits"] = benefitItems(item)
		}},
		{name: "group not configured", mutate: func(payload map[string]interface{}) {
			item := validBenefit()
			item["group_name"] = "missing-benefit-group"
			payload["benefits"] = benefitItems(item)
		}},
		{name: "rpm above max", mutate: func(payload map[string]interface{}) {
			item := validBenefit()
			item["rpm_limit"] = model.ActivityCodeMaxRpm + 1
			payload["benefits"] = benefitItems(item)
		}},
		{name: "negative token total", mutate: func(payload map[string]interface{}) {
			item := validBenefit()
			item["token_total"] = -1
			payload["benefits"] = benefitItems(item)
		}},
		{name: "token total above max", mutate: func(payload map[string]interface{}) {
			item := validBenefit()
			item["token_total"] = model.ActivityCodeMaxTokenTotal + 1
			payload["benefits"] = benefitItems(item)
		}},
		{name: "too many benefits", mutate: func(payload map[string]interface{}) {
			payload["benefits"] = tooMany
		}},
		{name: "invalid key", mutate: func(payload map[string]interface{}) {
			item := validBenefit()
			item["key"] = "BAD KEY"
			payload["benefits"] = benefitItems(item)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := activityBenefitRequestPayload()
			payload["code"] = "hdmbenefit-" + strings.ReplaceAll(tt.name, " ", "-")
			payload["benefits"] = benefitItems(validBenefit())
			tt.mutate(payload)
			response := callActivityAdmin(t, router, http.MethodPost, payload)
			assert.False(t, response.Success, "非法权益配置必须被拒绝")
		})
	}
}

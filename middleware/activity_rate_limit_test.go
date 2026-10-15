package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupActivityRateLimitTest 提供一个内存数据库，并关闭 Redis（走内存限流器路径）。
func setupActivityRateLimitTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ActivityCode{}, &model.ActivityGrant{}))
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedis
	})
	return db
}

func createActivityRateLimitGrant(t *testing.T, db *gorm.DB, userId int, group string, rpmLimit int, expireTime int64) {
	t.Helper()
	grant := &model.ActivityGrant{
		UserId: userId, CodeId: userId, Code: "act-rpm", CodeName: "rpm code", GroupName: group,
		RatioMode: model.ActivityRatioModeOverride, RpmLimit: rpmLimit,
		StartTime: common.GetTimestamp(), ExpireTime: expireTime, Status: model.ActivityGrantStatusActive,
	}
	require.NoError(t, db.Create(grant).Error)
}

// callActivityRateLimitRequests 依次发起请求并返回状态码，同时校验 429 的 Retry-After 提示。
func callActivityRateLimitRequests(t *testing.T, userId int, group string, requests int) []int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/activity-rate-limit", func(c *gin.Context) {
		c.Set("id", userId)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
	}, ActivityGrantRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	statuses := make([]int, 0, requests)
	for i := 0; i < requests; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/activity-rate-limit", nil))
		statuses = append(statuses, recorder.Code)
		if recorder.Code == http.StatusTooManyRequests {
			assert.Equal(t, "60", recorder.Header().Get("Retry-After"), "超限时必须提示一分钟窗口")
		}
	}
	return statuses
}

func TestActivityGrantRateLimitEnforcesPerGrantRpm(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9101, "act-rpm-group", 2, common.GetTimestamp()+3600)

	statuses := callActivityRateLimitRequests(t, 9101, "act-rpm-group", 3)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests}, statuses)
}

func TestActivityGrantRateLimitSkipsGroupsWithoutGrant(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9102, "act-other-group", 1, common.GetTimestamp()+3600)

	statuses := callActivityRateLimitRequests(t, 9102, "plain-group", 3)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK}, statuses, "没有活动授权的分组不受活动 RPM 限制")
}

func TestActivityGrantRateLimitUnlimitedWhenRpmIsZero(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9103, "act-unlimited-group", 0, common.GetTimestamp()+3600)

	statuses := callActivityRateLimitRequests(t, 9103, "act-unlimited-group", 5)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK, http.StatusOK, http.StatusOK}, statuses)
}

func TestActivityGrantRateLimitIgnoresExpiredGrant(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9104, "act-expired-group", 1, common.GetTimestamp()-10)

	statuses := callActivityRateLimitRequests(t, 9104, "act-expired-group", 3)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK}, statuses, "已过期的活动授权不再限流")
}

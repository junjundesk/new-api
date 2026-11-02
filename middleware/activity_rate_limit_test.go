package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupActivityRateLimitTest 提供一个内存数据库，并关闭 Redis（走内存限流器路径）。
func setupActivityRateLimitTest(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, i18n.Init())
	previousDB := model.DB
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.ActivityCode{}, &model.ActivityGrant{}))
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedis
	})
	return db
}

func createActivityRateLimitGrant(t *testing.T, db *gorm.DB, userId int, group, limitMode string, rpmLimit, concurrencyLimit int, expireTime int64) {
	t.Helper()
	grant := &model.ActivityGrant{
		UserId: userId, CodeId: userId, Code: "act-rpm", CodeName: "rpm code", GroupName: group,
		RatioMode: model.ActivityRatioModeOverride,
		LimitMode: limitMode, RpmLimit: rpmLimit, ConcurrencyLimit: concurrencyLimit,
		StartTime: common.GetTimestamp(), ExpireTime: expireTime, Status: model.ActivityGrantStatusActive,
	}
	require.NoError(t, db.Create(grant).Error)
}

// callActivityRateLimitRequests 模拟真实链路：中间件在 Distribute 之前建立请求级状态，
// 选渠道定组后做限流检查，请求结束统一释放。返回每次请求的状态码与 Retry-After。
func callActivityRateLimitRequests(t *testing.T, userId int, group string, requests int) ([]int, []string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/activity-rate-limit", func(c *gin.Context) {
		c.Set("id", userId)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
	}, ActivityGrantRateLimit(), func(c *gin.Context) {
		if limitErr := service.ActivityLimitGuard(c, group); limitErr != nil {
			abortWithActivityLimit(c, limitErr)
			return
		}
		c.Status(http.StatusOK)
	})

	statuses := make([]int, 0, requests)
	retryAfters := make([]string, 0, requests)
	for i := 0; i < requests; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/activity-rate-limit", nil))
		statuses = append(statuses, recorder.Code)
		retryAfters = append(retryAfters, recorder.Header().Get("Retry-After"))
	}
	return statuses, retryAfters
}

func TestActivityGrantRateLimitEnforcesPerGrantRpm(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9101, "act-rpm-group", model.ActivityLimitModeRPM, 2, 0, common.GetTimestamp()+3600)

	statuses, retryAfters := callActivityRateLimitRequests(t, 9101, "act-rpm-group", 3)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests}, statuses)

	retryAfter, err := strconv.ParseInt(retryAfters[2], 10, 64)
	require.NoError(t, err, "RPM 超限必须给出 Retry-After")
	assert.GreaterOrEqual(t, retryAfter, int64(1))
	assert.LessOrEqual(t, retryAfter, int64(60), "Retry-After 不超过一分钟窗口")
}

func TestActivityGrantRateLimitSkipsGroupsWithoutGrant(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9102, "act-other-group", model.ActivityLimitModeConcurrency, 0, 1, common.GetTimestamp()+3600)

	statuses, _ := callActivityRateLimitRequests(t, 9102, "plain-group", 3)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK}, statuses, "没有活动授权的分组不受活动限流")
}

func TestActivityGrantRateLimitUnlimitedWhenRpmIsZero(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9103, "act-unlimited-group", model.ActivityLimitModeRPM, 0, 0, common.GetTimestamp()+3600)

	statuses, _ := callActivityRateLimitRequests(t, 9103, "act-unlimited-group", 5)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK, http.StatusOK, http.StatusOK}, statuses)
}

func TestActivityGrantRateLimitIgnoresExpiredGrant(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9104, "act-expired-group", model.ActivityLimitModeRPM, 1, 0, common.GetTimestamp()-10)

	statuses, _ := callActivityRateLimitRequests(t, 9104, "act-expired-group", 3)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK}, statuses, "已过期的活动授权不再限流")
}

// TestActivityGrantConcurrencyReleasedAfterRequest 验证请求结束即释放并发槽：
// 顺序请求不会被前一个请求残留的槽挡住，并发满时 429 不带 Retry-After。
func TestActivityGrantConcurrencyReleasedAfterRequest(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9105, "act-conc-group", model.ActivityLimitModeConcurrency, 0, 1, common.GetTimestamp()+3600)

	// 限流=1：顺序请求（前一个结束后释放）都成功。
	statuses, retryAfters := callActivityRateLimitRequests(t, 9105, "act-conc-group", 3)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK}, statuses, "请求结束必须释放并发槽")
	for _, retryAfter := range retryAfters {
		assert.Empty(t, retryAfter, "并发超限不得伪造 Retry-After")
	}
}

// TestDistributeAllowsTaskFetchWithoutChannel 验证不选渠道的任务查询路由：
// SetupContextForSelectedChannel 对 nil channel 的返回错误不能把合法查询打成错误，
// 但无选渠道的请求仍要经过活动限流检查。
func TestDistributeAllowsTaskFetchWithoutChannel(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9201, "fetch-group", model.ActivityLimitModeRPM, 1, 0, common.GetTimestamp()+3600)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/mj/task/:id/fetch", func(c *gin.Context) {
		c.Set("id", 9201)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "fetch-group")
	}, ActivityGrantRateLimit(), Distribute(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/mj/task/123/fetch", nil))
	assert.Equal(t, http.StatusOK, recorder.Code, "任务查询是合法的无选渠道路径")

	// 无选渠道的请求同样受活动限流（RPM=1 的窗口已被上一个请求用掉）。
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/mj/task/123/fetch", nil))
	assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
}

// TestActivityGrantConcurrencyReleasedOnPanicOrAbort 验证异常路径同样释放并发槽：
// handler panic（Recovery 捕获）或提前 Abort 后，请求结束的统一释放仍要归还槽位。
func TestActivityGrantConcurrencyReleasedOnPanicOrAbort(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9202, "panic-group", model.ActivityLimitModeConcurrency, 0, 1, common.GetTimestamp()+3600)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	boom := true
	abortNext := false
	router.GET("/boom", func(c *gin.Context) {
		c.Set("id", 9202)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "panic-group")
	}, ActivityGrantRateLimit(), func(c *gin.Context) {
		if limitErr := service.ActivityLimitGuard(c, "panic-group"); limitErr != nil {
			abortWithActivityLimit(c, limitErr)
			return
		}
		if boom {
			boom = false
			abortNext = true // 下一个请求走 Abort 分支
			panic("handler crashed")
		}
		if abortNext {
			abortNext = false
			c.JSON(http.StatusBadRequest, gin.H{"error": "client mistake"})
			c.Abort()
			return
		}
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/boom", nil))
	assert.Equal(t, http.StatusInternalServerError, recorder.Code, "panic 由 Recovery 捕获")

	// panic 后并发槽必须已释放：下一个请求能占到同一个槽。
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/boom", nil))
	assert.Equal(t, http.StatusBadRequest, recorder.Code, "走 Abort 分支")

	// Abort 后同样释放。
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/boom", nil))
	assert.Equal(t, http.StatusOK, recorder.Code, "panic/Abort 路径都必须释放并发槽")
}

// TestActivityGrantConcurrencyRejectsInFlightRequest 验证同一用户的并发请求
// 共享 user+group 额度：第一个请求在途时第二个请求被 429 拒绝。
func TestActivityGrantConcurrencyRejectsInFlightRequest(t *testing.T) {
	db := setupActivityRateLimitTest(t)
	createActivityRateLimitGrant(t, db, 9106, "act-conc-shared", model.ActivityLimitModeConcurrency, 0, 1, common.GetTimestamp()+3600)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	blocked := make(chan struct{})
	release := make(chan struct{})
	router.GET("/hold", func(c *gin.Context) {
		c.Set("id", 9106)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "act-conc-shared")
	}, ActivityGrantRateLimit(), func(c *gin.Context) {
		if limitErr := service.ActivityLimitGuard(c, "act-conc-shared"); limitErr != nil {
			abortWithActivityLimit(c, limitErr)
			return
		}
		blocked <- struct{}{} // 槽已持有：通知测试发起第二个请求
		<-release
		c.Status(http.StatusOK)
	})

	done := make(chan int, 1)
	go func() {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/hold", nil))
		done <- recorder.Code
	}()
	<-blocked // 第一个请求已占槽且仍在途

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/hold", nil))
	assert.Equal(t, http.StatusTooManyRequests, recorder.Code, "同用户并发请求共享额度")
	assert.Empty(t, recorder.Header().Get("Retry-After"))

	close(release)
	assert.Equal(t, http.StatusOK, <-done, "第一个请求正常结束并释放并发槽")
}

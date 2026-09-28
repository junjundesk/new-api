package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 查看渠道密钥不做 Passkey/2FA 安全验证：持 root 身份即可直接取回密钥，
// 请求不带 X-Security-Proof 也必须成功。
func TestChannelKeyRouteRevealsKeyWithoutSecurityProof(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupChannelKeyRouteTestDB(t)

	user := &model.User{
		Username:    "root-key-route",
		Password:    "password-placeholder",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     "root-key-route-aff",
	}
	require.NoError(t, model.DB.Create(user).Error)

	now := common.GetTimestamp()
	session := &model.UserSession{
		SID:             "channel-key-route-session",
		UserID:          user.Id,
		Version:         1,
		UserAuthVersion: user.AuthVersion,
		Status:          model.UserSessionStatusActive,
		RefreshHash:     "refresh-hash",
		LoginMethod:     "password",
		LastActiveAt:    now,
		ExpiresAt:       now + 3600,
	}
	require.NoError(t, model.CreateUserSession(session))

	accessToken, _, err := service.IssueAccessToken(service.AuthIdentity{
		UserID:          user.Id,
		SessionID:       session.SID,
		UserAuthVersion: session.UserAuthVersion,
		SessionVersion:  session.Version,
	})
	require.NoError(t, err)

	channel := &model.Channel{
		Name:        "key-route-channel",
		Key:         "sk-channel-key-plain",
		Status:      common.ChannelStatusEnabled,
		Group:       "default",
		Models:      "gpt-4o",
		CreatedTime: now,
	}
	require.NoError(t, model.DB.Create(channel).Error)

	engine := gin.New()
	registerChannelRoutes(engine.Group("/api"))

	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/channel/%d/key", channel.Id), nil)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	require.Empty(t, request.Header.Get("X-Security-Proof"))
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	assert.NotContains(t, response.Body.String(), "SECURITY_PROOF_REQUIRED")
	assert.Equal(t, http.StatusOK, response.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, channel.Key, body.Data.Key)
}

func setupChannelKeyRouteTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	previousMemoryCache := common.MemoryCacheEnabled
	previousSecret := common.SessionSecret
	previousLogDB := model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.Channel{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.SessionSecret = "channel-key-route-test-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		common.MemoryCacheEnabled = previousMemoryCache
		common.SessionSecret = previousSecret
	})
}

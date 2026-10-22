package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserInviterControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))
	model.DB, model.LOG_DB = db, db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})
	return db
}

func seedUserInviterControllerUser(t *testing.T, id int, username string, role int, inviterId int) {
	t.Helper()
	user := &model.User{
		Id:        id,
		Username:  username,
		Password:  "password123",
		Role:      role,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   username + "-aff",
		InviterId: inviterId,
	}
	require.NoError(t, model.DB.Create(user).Error)
}

func newUserInviterRouter(actorId int, actorRole int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	actorContext := func(c *gin.Context) {
		c.Set("id", actorId)
		c.Set("role", actorRole)
		c.Set("username", "actor")
	}
	router.POST("/api/user/inviter/bind", actorContext, BindUserInviter)
	return router
}

type userInviterAPIResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

func callBindUserInviter(t *testing.T, router *gin.Engine, payload map[string]interface{}) userInviterAPIResponse {
	t.Helper()
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/inviter/bind", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	response := userInviterAPIResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestBindUserInviterBindsEverySelectedUser(t *testing.T) {
	setupUserInviterControllerTest(t)
	seedUserInviterControllerUser(t, 1, "root-actor", common.RoleRootUser, 0)
	seedUserInviterControllerUser(t, 40, "api-inviter", common.RoleCommonUser, 0)
	seedUserInviterControllerUser(t, 41, "api-target-one", common.RoleCommonUser, 0)
	seedUserInviterControllerUser(t, 42, "api-target-two", common.RoleCommonUser, 0)
	router := newUserInviterRouter(1, common.RoleRootUser)

	response := callBindUserInviter(t, router, map[string]interface{}{
		"user_ids":   []int{41, 42},
		"inviter_id": 40,
	})
	require.True(t, response.Success, "binding must succeed: "+response.Message)
	updated, ok := response.Data["updated"].(float64)
	require.True(t, ok, "response must report how many users were rebound")
	assert.Equal(t, float64(2), updated)

	for _, id := range []int{41, 42} {
		var user model.User
		require.NoError(t, model.DB.Select("id", "inviter_id").Where("id = ?", id).First(&user).Error)
		assert.Equal(t, 40, user.InviterId, "user %d must point at the new inviter", id)
	}
}

func TestBindUserInviterRejectsTargetTheActorCannotManage(t *testing.T) {
	setupUserInviterControllerTest(t)
	seedUserInviterControllerUser(t, 2, "admin-actor", common.RoleAdminUser, 0)
	seedUserInviterControllerUser(t, 50, "api-peer-admin", common.RoleAdminUser, 0)
	seedUserInviterControllerUser(t, 51, "api-target-user", common.RoleCommonUser, 0)
	router := newUserInviterRouter(2, common.RoleAdminUser)

	response := callBindUserInviter(t, router, map[string]interface{}{
		"user_ids":   []int{50},
		"inviter_id": 51,
	})
	assert.False(t, response.Success)

	var target model.User
	require.NoError(t, model.DB.Select("id", "inviter_id").Where("id = ?", 50).First(&target).Error)
	assert.Zero(t, target.InviterId, "a rejected request must not change the target")
}

func TestBindUserInviterRejectsSelfBinding(t *testing.T) {
	setupUserInviterControllerTest(t)
	seedUserInviterControllerUser(t, 1, "root-actor", common.RoleRootUser, 0)
	seedUserInviterControllerUser(t, 60, "api-self-target", common.RoleCommonUser, 0)
	router := newUserInviterRouter(1, common.RoleRootUser)

	response := callBindUserInviter(t, router, map[string]interface{}{
		"user_ids":   []int{60},
		"inviter_id": 60,
	})
	assert.False(t, response.Success)

	var target model.User
	require.NoError(t, model.DB.Select("id", "inviter_id").Where("id = ?", 60).First(&target).Error)
	assert.Zero(t, target.InviterId)
}

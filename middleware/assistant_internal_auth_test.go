package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInternalAuthTest(t *testing.T) *gin.Engine {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	model.InitDatabaseColumnNames()
	dsn := fmt.Sprintf("file:internal_auth_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{
		Id: 7, Username: "alice", Password: "password", DisplayName: "alice",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: "aff-internal-7", AuthVersion: 1,
	}).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/user/self", middleware.UserAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"id": c.GetInt("id"), "role": c.GetInt("role")}})
	})
	engine.GET("/api/admin/probe", middleware.AdminAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	engine.GET("/api/option/probe", middleware.RootAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	assistantops.Initialize(engine)
	return engine
}

func TestInternalActorAuthUsesDatabaseRoleAndPermissions(t *testing.T) {
	engine := setupInternalAuthTest(t)

	userOp, ok := assistantops.Find("GET /api/user/self")
	require.True(t, ok)
	result := assistantops.Invoke(context.Background(), assistantops.Actor{UserID: 7, Role: 1}, userOp, nil, nil, nil)
	require.True(t, result.Success, result.Message)
	userData, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 7, userData["id"])
	assert.EqualValues(t, 1, userData["role"])

	adminOp, ok := assistantops.Find("GET /api/admin/probe")
	require.True(t, ok)
	result = assistantops.Invoke(context.Background(), assistantops.Actor{UserID: 7, Role: 1}, adminOp, nil, nil, nil)
	require.False(t, result.Success)
	assert.Equal(t, http.StatusForbidden, result.StatusCode)

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 7).Update("role", common.RoleAdminUser).Error)

	result = assistantops.Invoke(context.Background(), assistantops.Actor{UserID: 7, Role: 10}, adminOp, nil, nil, nil)
	require.True(t, result.Success, result.Message)

	rootOp, ok := assistantops.Find("GET /api/option/probe")
	require.True(t, ok)
	result = assistantops.Invoke(context.Background(), assistantops.Actor{UserID: 7, Role: 10}, rootOp, nil, nil, nil)
	require.False(t, result.Success)
	assert.Equal(t, http.StatusForbidden, result.StatusCode)

	// A plain external request has no private actor context and must not pass auth.
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/self", nil))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

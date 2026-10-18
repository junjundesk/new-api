package router

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserOperationCannotReadAnotherUsersToken(t *testing.T) {
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	model.InitDatabaseColumnNames()
	dsn := fmt.Sprintf("file:assistant_scope_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}))
	require.NoError(t, db.Create(&model.User{
		Id: 1, Username: "alice", Password: "password", DisplayName: "alice",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: "aff-scope-1", AuthVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id: 2, Username: "bob", Password: "password", DisplayName: "bob",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: "aff-scope-2", AuthVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		UserId: 1, Key: "sk-alice-scope", Name: "alice-key", UnlimitedQuota: true, ExpiredTime: -1,
		Status: common.TokenStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		UserId: 2, Key: "sk-bob-scope", Name: "bob-key", UnlimitedQuota: true, ExpiredTime: -1,
		Status: common.TokenStatusEnabled,
	}).Error)
	aliceToken, err := model.GetTokenByKey("sk-alice-scope", true)
	require.NoError(t, err)
	bobToken, err := model.GetTokenByKey("sk-bob-scope", true)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	op, ok := assistantops.Find("GET /api/token/:id")
	require.True(t, ok)
	actor := assistantops.Actor{UserID: 1, Role: common.RoleCommonUser, Profile: "user"}

	own := assistantops.Invoke(context.Background(), actor, op, map[string]any{"id": aliceToken.Id}, nil, nil)
	require.True(t, own.Success, own.Message)

	other := assistantops.Invoke(context.Background(), actor, op, map[string]any{"id": bobToken.Id}, nil, nil)
	require.False(t, other.Success)
	assert.NotContains(t, fmt.Sprint(other.Data), "bob-key")
}

func TestAdminRevokeUserBonusViaOperationCatalog(t *testing.T) {
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	model.InitDatabaseColumnNames()
	dsn := fmt.Sprintf("file:assistant_bonus_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.UserBonusGrant{},
		&model.BonusPreConsumeRecord{},
		&model.BonusPreConsumeAllocation{},
		&model.Log{},
	))
	require.NoError(t, db.Create(&model.User{
		Id: 1, Username: "root", Password: "password", DisplayName: "root",
		Role: common.RoleRootUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: "aff-bonus-root", AuthVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id: 2, Username: "target", Password: "password", DisplayName: "target",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: "aff-bonus-target", AuthVersion: 1,
	}).Error)
	grant := &model.UserBonusGrant{UserId: 2, AmountTotal: 1000, ExpireTime: 0, Status: model.BonusStatusActive}
	require.NoError(t, db.Create(grant).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	op, ok := assistantops.Find("POST /api/user/:id/bonus/:grant_id/revoke")
	require.True(t, ok)
	actor := assistantops.Actor{UserID: 1, Role: common.RoleRootUser, Profile: "admin"}
	result := assistantops.Invoke(context.Background(), actor, op, map[string]any{"id": 2, "grant_id": grant.Id}, nil, map[string]any{"remark": "wrong expiry"})
	require.True(t, result.Success, result.Message)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 1000, data["revoked_amount"])

	var reloaded model.UserBonusGrant
	require.NoError(t, db.First(&reloaded, grant.Id).Error)
	assert.Equal(t, model.BonusStatusRevoked, reloaded.Status)
	assert.Equal(t, "wrong expiry", reloaded.RevokeRemark)
	assert.NotZero(t, reloaded.RevokedAt)
}

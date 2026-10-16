package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findAccountEventLog 按 other.op.action 找到一条账户事件日志。
func findAccountEventLog(t *testing.T, userId int, action string) *model.Log {
	t.Helper()
	logs := make([]*model.Log, 0)
	require.NoError(t, model.LOG_DB.Where("user_id = ?", userId).Order("id asc").Find(&logs).Error)
	for _, log := range logs {
		other := map[string]interface{}{}
		require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
		op, ok := other["op"].(map[string]interface{})
		if ok && op["action"] == action {
			return log
		}
	}
	t.Fatalf("no account event log for action %s", action)
	return nil
}

// 密钥的增删必须写入「用户日志」可展示的功能日志：这些动作此前完全没有记录，
// 日志类型为 Manage，正文由语言无关的 op 描述渲染（供导出使用的英文兜底）。
func TestTokenMutationsRecordAccountEventLogs(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))
	user := &model.User{
		Id:       101,
		Username: "token-log-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	createRequest := map[string]any{
		"name":            "account-log-key",
		"expired_time":    -1,
		"remain_quota":    0,
		"unlimited_quota": true,
		"group":           "default",
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", createRequest, user.Id)
	AddToken(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success, "token creation must succeed")

	var created model.Token
	require.NoError(t, model.DB.Where("name = ?", "account-log-key").First(&created).Error)

	createLog := findAccountEventLog(t, user.Id, "token.create")
	assert.Equal(t, model.LogTypeManage, createLog.Type)
	assert.Equal(t, "Created API key account-log-key", createLog.Content)

	deleteCtx, deleteRecorder := newAuthenticatedContext(t, http.MethodDelete, "/api/token/"+strconv.Itoa(created.Id), nil, user.Id)
	deleteCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(created.Id)}}
	DeleteToken(deleteCtx)
	require.True(t, decodeAPIResponse(t, deleteRecorder).Success, "token deletion must succeed")

	deleteLog := findAccountEventLog(t, user.Id, "token.delete")
	assert.Equal(t, model.LogTypeManage, deleteLog.Type)
	assert.Equal(t, "Deleted API key account-log-key (ID: "+strconv.Itoa(created.Id)+")", deleteLog.Content)
}

// 注册成功要留下账户事件日志（type=System），否则「用户日志」页看不到注册。
func TestRegisterAuditWritesAccountEventLog(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))

	ctx, _ := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", nil, 0)
	recordRegisterAudit(&model.User{Id: 42, Username: "alice"}, ctx)

	log := findAccountEventLog(t, 42, "user.register")
	assert.Equal(t, model.LogTypeSystem, log.Type)
	assert.Equal(t, "Registered account alice", log.Content)
	assert.Contains(t, log.Other, "alice")
}

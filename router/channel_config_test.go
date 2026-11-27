package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newChannelConfigTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	setupChannelKeyRouteTestDB(t)
	pool, err := model.DB.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{}))
	for name, role := range map[string]int{"config-root": common.RoleRootUser, "config-admin": common.RoleAdminUser} {
		user := model.User{Username: name, AffCode: name, Role: role, Status: common.UserStatusEnabled, AuthVersion: 1}
		user.SetAccessToken(name + "-pat")
		require.NoError(t, model.DB.Create(&user).Error)
	}
	engine := gin.New()
	registerChannelRoutes(engine.Group("/api"))
	return engine
}

type channelConfigTestResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func postChannelConfig(t *testing.T, engine *gin.Engine, action, token, body string) (*httptest.ResponseRecorder, channelConfigTestResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/channel/"+action, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	engine.ServeHTTP(result, req)
	var reply channelConfigTestResponse
	require.NoError(t, common.Unmarshal(result.Body.Bytes(), &reply))
	return result, reply
}

func TestChannelConfigRoundTripCreatesNewRoutableChannelsWithoutSourceHistory(t *testing.T) {
	engine := newChannelConfigTestRouter(t)
	source := []model.Channel{
		{Name: "subscription", Type: constant.ChannelTypeCodex, Key: `{"access_token":"fixture-token","account_id":"fixture-account"}`, Models: "model-a", Group: "default", Status: common.ChannelStatusManuallyDisabled},
		{Name: "pool", Type: constant.ChannelTypeOpenAI, Key: "fixture-first\nfixture-second", Models: "model-b", Group: "vip",
			BaseURLs: []string{"https://one.example", "https://two.example"}, Weight: common.GetPointer(uint(0)), AutoBan: common.GetPointer(0),
			ModelMapping: common.GetPointer(`{"model-b":["upstream-b","backup-b"]}`), HeaderOverride: common.GetPointer(`{"X-Tenant":"tenant"}`),
			OtherSettings: `{"vendor_option":"keep","upstream_model_update_last_removed_models":["stale"]}`,
			Balance:       10, UsedQuota: 123, TestTime: 456, CreatedTime: 123, OtherInfo: "source history",
			ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeyMode: constant.MultiKeyModePolling, MultiKeySize: 2, MultiKeyPollingIndex: 1, MultiKeyStatusList: map[int]int{1: common.ChannelStatusManuallyDisabled}},
		},
		{Name: "unselected", Type: constant.ChannelTypeOpenAI, Key: "fixture-unselected", Models: "model-c"},
	}
	require.NoError(t, model.DB.Create(&source).Error)
	selection, err := common.Marshal(gin.H{"ids": []int{source[1].Id, source[0].Id}})
	require.NoError(t, err)
	w, exported := postChannelConfig(t, engine, "export", "config-root-pat", string(selection))
	require.True(t, exported.Success, exported.Message)
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
	var doc struct {
		Version  int              `json:"version"`
		Channels []map[string]any `json:"channels"`
	}
	require.NoError(t, common.Unmarshal(exported.Data, &doc))
	assert.Equal(t, 1, doc.Version)
	require.Len(t, doc.Channels, 2)
	assert.Equal(t, source[1].Key, doc.Channels[0]["key"])
	assert.Equal(t, source[0].Key, doc.Channels[1]["key"])
	assert.NotContains(t, string(exported.Data), "fixture-unselected")
	for _, field := range []string{"id", "used_quota", "balance", "test_time", "created_time", "other_info", "channel_info"} {
		assert.NotContains(t, doc.Channels[0], field)
	}

	_, imported := postChannelConfig(t, engine, "import", "config-root-pat", string(exported.Data))
	require.True(t, imported.Success, imported.Message)
	assert.Equal(t, "2", string(imported.Data))
	var created []model.Channel
	require.NoError(t, model.DB.Where("id > ?", source[2].Id).Order("id").Find(&created).Error)
	require.Len(t, created, 2)
	pool := created[0]
	assert.Equal(t, source[1].Name, pool.Name)
	assert.Equal(t, []string{"fixture-first", "fixture-second"}, pool.GetKeys())
	assert.Equal(t, source[1].BaseURLs, pool.BaseURLs)
	assert.Equal(t, source[1].BaseURLs[0], pool.GetBaseURL())
	assert.Equal(t, source[1].AutoBan, pool.AutoBan)
	assert.Equal(t, source[1].Weight, pool.Weight)
	assert.Equal(t, source[1].HeaderOverride, pool.HeaderOverride)
	assert.Equal(t, source[1].ModelMapping, pool.ModelMapping)
	assert.JSONEq(t, `{"vendor_option":"keep"}`, pool.OtherSettings)
	assert.Equal(t, model.ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling, MultiKeyStatusList: map[int]int{1: common.ChannelStatusManuallyDisabled}}, pool.ChannelInfo)
	assert.Zero(t, pool.Balance)
	assert.Zero(t, pool.UsedQuota)
	assert.Zero(t, pool.TestTime)
	assert.Greater(t, pool.CreatedTime, source[1].CreatedTime)
	assert.Equal(t, source[0].Key, created[1].Key)
	var abilities []model.Ability
	require.NoError(t, model.DB.Order("channel_id").Find(&abilities).Error)
	require.Len(t, abilities, 2)
	assert.Equal(t, pool.Id, abilities[0].ChannelId)
	assert.Equal(t, "model-b", abilities[0].Model)
	assert.Equal(t, "vip", abilities[0].Group)
	assert.True(t, abilities[0].Enabled)
	assert.False(t, abilities[1].Enabled)

	var audits []model.Log
	require.NoError(t, model.LOG_DB.Order("id").Find(&audits).Error)
	require.Len(t, audits, 2)
	assert.Contains(t, audits[0].Other, "channel.export")
	assert.Contains(t, audits[1].Other, "channel.import")
	for _, log := range audits {
		assert.NotContains(t, log.Other+log.Content, "fixture-")
	}
}

func TestChannelConfigRejectsInvalidDocumentsWithoutCreatingAnyChannels(t *testing.T) {
	engine := newChannelConfigTestRouter(t)
	const first = `{"name":"valid","type":1,"key":"fixture-key","models":"model"}`
	for _, tc := range []struct{ name, document string }{
		{"unknown version", `{"version":2,"channels":[` + first + `]}`},
		{"empty list", `{"version":1,"channels":[]}`},
		{"destination owned field", `{"version":1,"channels":[` + first + `,{"id":1,"name":"replace","type":1,"key":"key","models":"m"}]}`},
		{"invalid provider config after valid channel", `{"version":1,"channels":[` + first + `,{"name":"bad","type":60,"key":"key","models":"m"}]}`},
		{"key status outside actual key list", `{"version":1,"channels":[` + first + `,{"name":"bad","type":1,"models":"m","key":"only-one","multi_key_mode":"polling","multi_key_status_list":{"1":2}}]}`},
		{"trailing document", `{"version":1,"channels":[` + first + `]} {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, result := postChannelConfig(t, engine, "import", "config-root-pat", tc.document)
			require.False(t, result.Success)
			assert.NotEmpty(t, result.Message)
			var count int64
			require.NoError(t, model.DB.Model(&model.Channel{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestChannelConfigImportRollsBackChannelsIfRoutingCannotBeSaved(t *testing.T) {
	engine := newChannelConfigTestRouter(t)
	require.NoError(t, model.DB.Migrator().DropTable(&model.Ability{}))
	_, result := postChannelConfig(t, engine, "import", "config-root-pat", `{"version":1,"channels":[{"name":"new","type":1,"key":"fixture-key","models":"m"}]}`)
	require.False(t, result.Success)
	var count int64
	require.NoError(t, model.DB.Model(&model.Channel{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestChannelConfigExportDoesNotDisclosePartialOrUnauthorizedSelections(t *testing.T) {
	engine := newChannelConfigTestRouter(t)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 7, Name: "saved", Type: 1, Key: "fixture-private", Models: "model"}).Error)
	for _, tc := range []struct {
		token, ids string
		status     int
	}{
		{"config-admin-pat", `{"ids":[7]}`, http.StatusForbidden},
		{"config-root-pat", `{"ids":[7,42]}`, http.StatusOK},
		{"config-root-pat", `{"ids":[]}`, http.StatusOK},
	} {
		w, result := postChannelConfig(t, engine, "export", tc.token, tc.ids)
		assert.Equal(t, tc.status, w.Code)
		assert.False(t, result.Success)
		assert.Empty(t, result.Data)
		assert.NotContains(t, w.Body.String(), "fixture-private")
	}
	w, result := postChannelConfig(t, engine, "import", "config-admin-pat", `{"version":1,"channels":[{"name":"new","type":1,"key":"new-key","models":"m"}]}`)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, result.Success)
}

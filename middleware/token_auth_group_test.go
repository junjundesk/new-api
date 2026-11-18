package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const tokenAuthDisabledGroup = "retired-group"

func setupTokenAuthGroupTest(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	model.InitDatabaseColumnNames()
	common.RedisEnabled = false

	savedUsableGroups := setting.UserUsableGroups2JSONString()
	savedGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(savedUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroupRatio))
	})

	// The test group is neither a usable group nor a configured pricing group.
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
}

func createTokenAuthGroupToken(t *testing.T, role int) *model.Token {
	t.Helper()
	username := "group-gate-common"
	if role >= common.RoleAdminUser {
		username = "group-gate-admin"
	}
	user := &model.User{
		Username: username,
		Password: "password-placeholder",
		Role:     role,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "token-auth-group-" + username,
	}
	require.NoError(t, model.DB.Create(user).Error)

	token := &model.Token{
		UserId:         user.Id,
		Key:            "groupgatetoken" + strings.ReplaceAll(username, "-", ""),
		Name:           "group-gate",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          tokenAuthDisabledGroup,
	}
	require.NoError(t, model.DB.Create(token).Error)
	return token
}

func callTokenAuth(t *testing.T, token *model.Token) (*httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/group", TokenAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		})
	})
	request := httptest.NewRequest(http.MethodGet, "/group", nil)
	request.Header.Set("Authorization", "Bearer sk-"+token.Key)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var body struct {
		Group string `json:"group"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	return response, body.Group
}

func TestTokenAuthAdminTokenCallsDisabledGroup(t *testing.T) {
	setupTokenAuthGroupTest(t)
	require.NoError(t, i18n.Init())
	token := createTokenAuthGroupToken(t, common.RoleAdminUser)

	response, group := callTokenAuth(t, token)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, tokenAuthDisabledGroup, group)
}

func TestTokenAuthCommonTokenRejectsDisabledGroup(t *testing.T) {
	setupTokenAuthGroupTest(t)
	require.NoError(t, i18n.Init())
	token := createTokenAuthGroupToken(t, common.RoleCommonUser)

	response, _ := callTokenAuth(t, token)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTokenAuthCommonTokenRejectsGroupWithoutPricing(t *testing.T) {
	setupTokenAuthGroupTest(t)
	require.NoError(t, i18n.Init())
	token := createTokenAuthGroupToken(t, common.RoleCommonUser)
	require.NoError(t, model.DB.Model(token).Update("group", "vip").Error)
	token.Group = "vip"

	response, _ := callTokenAuth(t, token)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

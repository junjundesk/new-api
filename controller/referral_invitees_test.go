package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type referralInviteesAPIResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Items []model.ReferralInvitee `json:"items"`
		Total int64                   `json:"total"`
	} `json:"data"`
}

func callReferralInvitees(t *testing.T, router *gin.Engine, path string) referralInviteesAPIResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	response := referralInviteesAPIResponse{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func setupReferralInviteesTest(t *testing.T) *gin.Engine {
	t.Helper()
	db := setupUserInviterControllerTest(t)
	require.NoError(t, db.AutoMigrate(&model.ReferralCharge{}))
	seedUserInviterControllerUser(t, 50, "first-inviter", common.RoleCommonUser, 0)
	seedUserInviterControllerUser(t, 51, "invited-friend", common.RoleCommonUser, 50)
	seedUserInviterControllerUser(t, 60, "other-inviter", common.RoleCommonUser, 0)
	seedUserInviterControllerUser(t, 61, "someone-else", common.RoleCommonUser, 60)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/user/referral/invitees", func(c *gin.Context) { c.Set("id", 50) }, GetReferralInvitees)
	router.GET("/api/user/:id/invitees", AdminListReferralInvitees)
	return router
}

// An inviter sees only the accounts they invited, and never another user's
// full username.
func TestGetReferralInviteesMasksUsernamesAndScopesToCaller(t *testing.T) {
	router := setupReferralInviteesTest(t)

	response := callReferralInvitees(t, router, "/api/user/referral/invitees")

	require.True(t, response.Success)
	assert.Equal(t, int64(1), response.Data.Total)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, 51, response.Data.Items[0].Id)
	assert.Equal(t, "in***nd", response.Data.Items[0].Username)
}

func TestAdminListReferralInviteesReturnsFullUsernames(t *testing.T) {
	router := setupReferralInviteesTest(t)

	response := callReferralInvitees(t, router, "/api/user/60/invitees")

	require.True(t, response.Success)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, "someone-else", response.Data.Items[0].Username)
}

func TestAdminListReferralInviteesRejectsInvalidInviterId(t *testing.T) {
	router := setupReferralInviteesTest(t)

	response := callReferralInvitees(t, router, "/api/user/abc/invitees")

	assert.False(t, response.Success)
}

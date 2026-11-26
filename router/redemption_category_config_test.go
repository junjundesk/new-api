/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const categoryConfigPath = "/api/redemption/categories/"

type categoryConfigFixture struct {
	router      *gin.Engine
	admin, user string
}

func setupCategoryConfig(t *testing.T) categoryConfigFixture {
	t.Helper()
	setupChannelKeyRouteTestDB(t)
	pool, err := model.DB.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	require.NoError(t, model.DB.AutoMigrate(&model.Redemption{}, &model.RedemptionCategory{}, &model.RedemptionBonusRule{}))
	f := categoryConfigFixture{router: gin.New(), admin: "config-admin-pat", user: "config-user-pat"}
	for _, account := range []struct {
		token string
		role  int
	}{{f.admin, common.RoleAdminUser}, {f.user, common.RoleCommonUser}} {
		u := model.User{Username: account.token, AffCode: account.token, Role: account.role, Status: common.UserStatusEnabled, AuthVersion: 1}
		u.SetAccessToken(account.token)
		require.NoError(t, model.DB.Create(&u).Error)
	}
	SetApiRouter(f.router)
	return f
}

func (f categoryConfigFixture) request(t *testing.T, method, action, token, payload string, status int, success bool) json.RawMessage {
	t.Helper()
	req := httptest.NewRequest(method, categoryConfigPath+action, strings.NewReader(payload))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	require.Equal(t, status, w.Code, w.Body.String())
	var body struct {
		Success bool
		Data    json.RawMessage
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, success, body.Success, w.Body.String())
	return body.Data
}

func TestRedemptionCategoryConfigRoundTripWithoutCodes(t *testing.T) {
	f := setupCategoryConfig(t)
	empty := f.request(t, http.MethodGet, "export", f.admin, "", http.StatusOK, true)
	assert.JSONEq(t, `{"version":1,"categories":[]}`, string(empty))
	assert.JSONEq(t, `{"created":0,"skipped":0}`, string(f.request(t, http.MethodPost, "import", f.admin, string(empty), http.StatusOK, true)))

	standard := model.RedemptionCategory{Name: "Standard", Quota: 5000000}
	require.NoError(t, standard.SaveSettings())
	rule := model.RedemptionBonusRule{CategoryId: standard.Id, BonusRateBps: 250}
	require.NoError(t, model.DB.Create(&rule).Error)
	codes := []model.Redemption{{Key: "secret-issued-code", Name: "Gift", Quota: standard.Quota, CategoryId: standard.Id}, {Key: "secret-uncategorized", Quota: 800}}
	require.NoError(t, model.DB.Create(&codes).Error)
	const payload = `{"version":1,"categories":[{"name":"Standard","quota":5000000,"bonus_rate_bps":250},{"name":"Trial","quota":1000,"bonus_rate_bps":0},{"name":"Large","quota":9007199254740990}]}`
	data := f.request(t, http.MethodPost, "import", f.admin, payload, http.StatusOK, true)
	assert.JSONEq(t, `{"created":2,"skipped":1}`, string(data))
	exported := f.request(t, http.MethodGet, "export", f.admin, "", http.StatusOK, true)
	assert.JSONEq(t, `{"version":1,"categories":[{"name":"Trial","quota":1000,"bonus_rate_bps":0},{"name":"Standard","quota":5000000,"bonus_rate_bps":250},{"name":"Large","quota":9007199254740990}]}`, string(exported))
	data = f.request(t, http.MethodPost, "import", f.admin, string(exported), http.StatusOK, true)
	assert.JSONEq(t, `{"created":0,"skipped":3}`, string(data))
	var actual []model.Redemption
	require.NoError(t, model.DB.Unscoped().Order("id ASC").Find(&actual).Error)
	assert.Equal(t, codes, actual, "import must not issue, delete or reassign any codes")
}

func TestRedemptionCategoryConfigImportRejectsWholeBatch(t *testing.T) {
	f := setupCategoryConfig(t)
	baseline := `{"version":1,"categories":[{"name":"Standard","quota":1000,"bonus_rate_bps":0}]}`
	f.request(t, http.MethodPost, "import", f.admin, baseline, http.StatusOK, true)
	// A valid new category with a rule comes first. A failure on the second entry must roll back both.
	prefix := `{"version":1,"categories":[{"name":"New","quota":2000,"bonus_rate_bps":50},`
	for _, tc := range []struct{ name, payload string }{
		{"different face value", prefix + `{"name":"Standard","quota":500,"bonus_rate_bps":0}]}`},
		{"different bonus rate", prefix + `{"name":"Standard","quota":1000,"bonus_rate_bps":500}]}`},
		{"a zero rate is not an absent rule", prefix + `{"name":"Standard","quota":1000}]}`},
		{"duplicate name after trimming", prefix + `{"name":" New ","quota":2000}]}`},
		{"invalid name", prefix + `{"name":" ","quota":1000}]}`},
		{"quota at saturation limit", prefix + `{"name":"Large","quota":9007199254740991}]}`},
		{"invalid bonus rate", prefix + `{"name":"Large","quota":1000,"bonus_rate_bps":100001}]}`},
		{"bonus would saturate", prefix + `{"name":"Large","quota":9007199254740990,"bonus_rate_bps":20000}]}`},
		{"fractional quota", prefix + `{"name":"Fractional","quota":10.5}]}`},
		{"code field", prefix + `{"name":"Unsafe","quota":2000,"key":"not-a-setting"}]}`},
		{"database id", prefix + `{"name":"Unsafe","quota":2000,"id":77}]}`},
		{"top level codes", `{"version":1,"categories":[],"codes":[]}`},
		{"unsupported version", `{"version":2,"categories":[]}`},
		{"missing categories", `{"version":1}`},
		{"trailing JSON", baseline + `{}`},
		{"oversized document", baseline + strings.Repeat(" ", 1<<20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.request(t, http.MethodPost, "import", f.admin, tc.payload, http.StatusOK, false)
			data := f.request(t, http.MethodGet, "export", f.admin, "", http.StatusOK, true)
			assert.JSONEq(t, baseline, string(data))
			var rules int64
			require.NoError(t, model.DB.Model(&model.RedemptionBonusRule{}).Count(&rules).Error)
			assert.EqualValues(t, 1, rules, "no orphan bonus rule may survive")
		})
	}
}

func TestRedemptionCategoryConfigRequiresAdmin(t *testing.T) {
	f := setupCategoryConfig(t)
	for _, endpoint := range []struct{ method, action string }{{http.MethodGet, "export"}, {http.MethodPost, "import"}} {
		for _, auth := range []struct {
			token  string
			status int
		}{{"", http.StatusUnauthorized}, {f.user, http.StatusForbidden}} {
			assert.Empty(t, f.request(t, endpoint.method, endpoint.action, auth.token, `{"version":1,"categories":[{"name":"Trial","quota":1000}]}`, auth.status, false))
		}
	}
	assert.JSONEq(t, `{"version":1,"categories":[]}`, string(f.request(t, http.MethodGet, "export", f.admin, "", http.StatusOK, true)))
}

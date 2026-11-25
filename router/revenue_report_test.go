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

func TestRevenueRoutesRequireAdminAndNeverReturnSiteDataToUsers(t *testing.T) {
	setupChannelKeyRouteTestDB(t) // Standard dashboard auth fixture (User, Session and Log).
	pool, err := model.DB.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1) // The SQLite :memory: database is private to a connection, including audit reads.
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}, &model.TopUp{}, &model.Redemption{}, &model.UserRechargeRecord{}, &model.SubscriptionOrder{}))
	model.InitDatabaseColumnNames()
	for _, role := range []struct {
		name string
		role int
	}{{"rev-user", common.RoleCommonUser}, {"rev-admin", common.RoleAdminUser}} {
		u := model.User{Username: role.name, AffCode: role.name, Role: role.role, Status: common.UserStatusEnabled, AuthVersion: 1}
		u.SetAccessToken(role.name + "-pat")
		require.NoError(t, model.DB.Create(&u).Error)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	for _, tc := range []struct{ method, path string }{{"PUT", "/api/revenue/groups"}, {"GET", "/api/revenue/groups"}, {"GET", "/api/revenue/report?range=today"}} {
		for _, auth := range []struct {
			token  string
			status int
		}{{"", http.StatusUnauthorized}, {"rev-user-pat", http.StatusForbidden}, {"rev-admin-pat", http.StatusOK}} {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"groups":[{"group":"default","cost_ratio":"0.5","models":[]}]}`))
			if auth.token != "" {
				req.Header.Set("Authorization", "Bearer "+auth.token)
			}
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			require.Equal(t, auth.status, w.Code, w.Body.String())
			var body struct {
				Success bool
				Data    any
			}
			require.NoError(t, common.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, auth.status == http.StatusOK, body.Success, w.Body.String())
			if auth.status != http.StatusOK {
				assert.Nil(t, body.Data)
			}
		}
	}
}

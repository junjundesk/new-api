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

func TestRedemptionMigrationRequiresAdminAndExplicitRule(t *testing.T) {
	setupChannelKeyRouteTestDB(t)
	pool, err := model.DB.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	require.NoError(t, model.DB.AutoMigrate(&model.Redemption{}, &model.RedemptionCategory{}))
	for _, role := range []struct {
		name string
		role int
	}{{"migrate-user", common.RoleCommonUser}, {"migrate-admin", common.RoleAdminUser}} {
		u := model.User{Username: role.name, Role: role.role, AffCode: role.name, Status: common.UserStatusEnabled, AuthVersion: 1}
		u.SetAccessToken(role.name + "-pat")
		require.NoError(t, model.DB.Create(&u).Error)
	}
	category := model.RedemptionCategory{Name: "Standard", Quota: 1000}
	require.NoError(t, category.SaveSettings())
	code := model.Redemption{Key: "legacy", Quota: 1030}
	require.NoError(t, model.DB.Create(&code).Error)
	engine := gin.New()
	SetApiRouter(engine)
	for _, tc := range []struct {
		token   string
		body    string
		status  int
		success bool
	}{
		{"migrate-user-pat", `{"mode":"fuzzy"}`, http.StatusForbidden, false},
		{"migrate-admin-pat", `{}`, http.StatusOK, false},
		{"migrate-admin-pat", `{"mode":"nearest"}`, http.StatusOK, false},
		{"migrate-admin-pat", `{"mode":"fuzzy"}`, http.StatusOK, true},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/redemption/migrate-uncategorized", strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		require.Equal(t, tc.status, w.Code, w.Body.String())
		var body struct {
			Success bool
			Data    *model.RedemptionMigrationResult
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, tc.success, body.Success, w.Body.String())
		stored, err := model.GetRedemptionById(code.Id)
		require.NoError(t, err)
		if !tc.success {
			assert.Nil(t, body.Data)
			assert.Zero(t, stored.CategoryId)
			assert.Equal(t, 1030, stored.Quota)
			continue
		}
		require.NotNil(t, body.Data)
		assert.Equal(t, model.RedemptionMigrationResult{Migrated: 1}, *body.Data)
		assert.Equal(t, category.Id, stored.CategoryId)
		assert.Equal(t, category.Quota, stored.Quota)
	}
}

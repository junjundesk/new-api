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
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletTierVisibilityRequiresAdmin(t *testing.T) {
	f := setupCategoryConfig(t)
	tier := model.RedemptionCategory{Name: "Standard", Quota: 1000}
	require.NoError(t, tier.SaveSettings())
	body, err := common.Marshal(map[string][]int{"show": {tier.Id}})
	require.NoError(t, err)
	for _, tc := range []struct {
		token     string
		status    int
		published bool
	}{{"", http.StatusUnauthorized, false}, {f.user, http.StatusForbidden, false}, {f.admin, http.StatusOK, true}} {
		req := httptest.NewRequest(http.MethodPut, "/api/redemption/wallet-visibility", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		f.router.ServeHTTP(w, req)
		require.Equal(t, tc.status, w.Code, w.Body.String())
		var stored model.RedemptionCategory
		require.NoError(t, model.DB.First(&stored, tier.Id).Error)
		assert.Equal(t, tc.published, stored.WalletVisible)
	}
}

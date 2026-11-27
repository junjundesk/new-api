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
package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCardStoreWalletUsesLiveRedemptionTiersAndPreservesGatewayMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupManageUserTestDB(t)
	type legacyCategory struct {
		Id    int
		Name  string `gorm:"type:varchar(20);uniqueIndex;not null"`
		Quota int    `gorm:"not null"`
	}
	// Existing categories have no visibility field before AutoMigrate.
	private := legacyCategory{Name: "Staff only", Quota: 25}
	require.NoError(t, db.Table("redemption_categories").AutoMigrate(&legacyCategory{}))
	require.NoError(t, db.Table("redemption_categories").Create(&private).Error)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.RedemptionCategory{}, &model.RedemptionBonusRule{}))
	confirmPaymentComplianceForTest(t)
	oldCfg := operation_setting.GetCardStoreConfig()
	oldOptions := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() { operation_setting.GetPaymentSetting().CardStore = oldCfg; common.OptionMap = oldOptions })
	engine := gin.New()
	engine.PUT("/settings", UpdateOption)
	engine.GET("/info", GetTopUpInfo)
	engine.PUT("/wallet-visibility", UpdateWalletTierVisibility)
	engine.POST("/new-wallet-order", RequireWalletGatewayMode, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	save := func(enabled bool, url string) bool {
		t.Helper()
		// A storefront saved by the old all-or-nothing UI must not publish unchecked tiers.
		value, err := common.Marshal(gin.H{"enabled": enabled, "url": url, "show_tiers": true})
		require.NoError(t, err)
		body, err := common.Marshal(OptionUpdateRequest{Key: operation_setting.CardStoreOptionKey, Value: string(value)})
		require.NoError(t, err)
		var reply struct{ Success bool }
		require.NoError(t, common.Unmarshal(request("PUT", "/settings", string(body)).Body.Bytes(), &reply))
		return reply.Success
	}
	setVisibility := func(show, hide []int) bool {
		t.Helper()
		body, err := common.Marshal(gin.H{"show": show, "hide": hide})
		require.NoError(t, err)
		var reply struct{ Success bool }
		require.NoError(t, common.Unmarshal(request("PUT", "/wallet-visibility", string(body)).Body.Bytes(), &reply))
		return reply.Success
	}
	const url = "https://shop.example.test/shop?tier=all"
	require.True(t, save(true, url))
	require.False(t, save(true, "data:text/html,invalid"))
	assert.Equal(t, url, operation_setting.GetCardStoreConfig().URL)
	var persisted model.Option
	require.NoError(t, db.Where(&model.Option{Key: operation_setting.CardStoreOptionKey}).First(&persisted).Error)
	cfg, err := operation_setting.ParseCardStoreConfig(persisted.Value)
	require.NoError(t, err)
	assert.Equal(t, operation_setting.GetCardStoreConfig(), cfg)
	assert.Equal(t, http.StatusConflict, request("POST", "/new-wallet-order", "").Code)
	tier1 := model.RedemptionCategory{Name: "Small", Quota: 25}
	tier2 := model.RedemptionCategory{Name: "Large", Quota: 100}
	require.NoError(t, tier2.SaveSettings())
	require.NoError(t, tier1.SaveSettings())
	_, err = model.ReplaceRedemptionBonusRules([]model.RedemptionBonusRule{{CategoryId: tier1.Id, BonusRateBps: 200}})
	require.NoError(t, err)
	type storeView struct {
		URL       string
		ShowTiers bool `json:"show_tiers"`
		Tiers     []map[string]any
	}
	info := func() *storeView {
		t.Helper()
		var reply struct {
			Success bool
			Data    struct {
				Store *storeView `json:"card_store"`
			}
		}
		w := request("GET", "/info", "")
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &reply))
		require.True(t, reply.Success, w.Body.String())
		return reply.Data.Store
	}
	store := info()
	require.NotNil(t, store)
	assert.Equal(t, url, store.URL)
	assert.False(t, store.ShowTiers)
	assert.Equal(t, []map[string]any{}, store.Tiers, "unpublished tiers must not reach the wallet")
	require.True(t, setVisibility([]int{tier2.Id}, nil))
	store = info()
	require.Len(t, store.Tiers, 1)
	assert.Equal(t, "Large", store.Tiers[0]["name"])
	require.True(t, setVisibility([]int{tier1.Id}, nil))
	store = info()
	assert.True(t, store.ShowTiers)
	require.Len(t, store.Tiers, 2)
	assert.Equal(t, map[string]any{"id": float64(tier1.Id), "name": "Small", "quota": float64(25), "bonus_quota": float64(1)}, store.Tiers[0])
	assert.Equal(t, float64(0), store.Tiers[1]["bonus_quota"])
	_, err = model.ReplaceRedemptionBonusRules([]model.RedemptionBonusRule{{CategoryId: tier2.Id, BonusRateBps: 2500}})
	require.NoError(t, err)
	store = info()
	assert.Equal(t, float64(0), store.Tiers[0]["bonus_quota"])
	assert.Equal(t, float64(25), store.Tiers[1]["bonus_quota"])
	// A stale or contradictory batch must not partially change the catalog.
	assert.False(t, setVisibility([]int{private.Id}, []int{max(private.Id, tier1.Id, tier2.Id) + 1}))
	assert.False(t, setVisibility([]int{private.Id}, []int{private.Id}))
	require.Equal(t, store.Tiers, info().Tiers)
	require.True(t, setVisibility([]int{private.Id}, []int{tier1.Id, tier2.Id}))
	store = info()
	require.Len(t, store.Tiers, 1)
	assert.Equal(t, "Staff only", store.Tiers[0]["name"])
	rename := model.RedemptionCategory{Id: private.Id, Name: "Member"}
	require.NoError(t, rename.SaveSettings())
	assert.True(t, rename.WalletVisible, "renaming must preserve the published state")
	require.True(t, setVisibility(nil, []int{private.Id}))
	store = info()
	assert.Empty(t, store.Tiers)
	assert.False(t, store.ShowTiers)
	assert.Equal(t, url, store.URL)
	require.NoError(t, db.AutoMigrate(&model.Redemption{}))
	require.NoError(t, model.DeleteRedemptionCategory(tier1.Id))
	require.NoError(t, model.DeleteRedemptionCategory(tier2.Id))
	assert.NotNil(t, info().Tiers)
	assert.Empty(t, info().Tiers)
	operation_setting.GetPaymentSetting().ComplianceConfirmed = false
	assert.Nil(t, info())
	assert.False(t, save(true, url))
	require.True(t, save(false, url))
	assert.Equal(t, url, operation_setting.GetCardStoreConfig().URL)
	operation_setting.GetPaymentSetting().ComplianceConfirmed = true
	assert.Nil(t, info())
	assert.Equal(t, http.StatusNoContent, request("POST", "/new-wallet-order", "").Code)
}

package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPricingBillingTest(t *testing.T) {
	t.Helper()
	resetPricingEndpointTestTables(t)
	groups := ratio_setting.GroupRatio2JSONString()
	specials := ratio_setting.GroupGroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2,"vip":3,"hidden":9,"all":4}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"default":0.5,"vip":1},"free":{"default":0}}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(groups))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(specials))
	})
}

func insertPricingBillingChannel(t *testing.T, name, group, settings string, status int, enabled bool, priority int64) *Channel {
	t.Helper()
	channel := &Channel{
		Type: constant.ChannelTypeOpenAI, Key: "private-pricing-test-key", Name: "private-channel-name",
		Status: status, Models: name, Group: group, Priority: &priority,
	}
	if settings != "" {
		channel.Setting = &settings
	}
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, DB.Create(&Ability{
		Group: group, Model: name, ChannelId: channel.Id, Enabled: enabled, Priority: &priority,
	}).Error)
	return channel
}

func TestPricingBillingRatioResolution(t *testing.T) {
	setupPricingBillingTest(t)
	tests := []struct {
		name      string
		userGroup string
		settings  []string
		want      BillingRatioRange
	}{
		{"inherit", "", []string{""}, BillingRatioRange{2, 2, "group"}},
		{"free", "member", []string{`{"model_group_ratios":{"free":0}}`}, BillingRatioRange{0, 0, "channel_model"}},
		{"one", "member", []string{`{"model_group_ratios":{"one":1}}`}, BillingRatioRange{1, 1, "channel_model"}},
		{"special", "member", []string{""}, BillingRatioRange{0.5, 0.5, "user_group"}},
		{"free-special", "free", []string{""}, BillingRatioRange{0, 0, "user_group"}},
		{"mixed", "member", []string{`{"model_group_ratios":{"mixed":0}}`, ""}, BillingRatioRange{0, 0.5, "mixed"}},
		{"same-price", "member", []string{`{"model_group_ratios":{"same-price":0.5}}`, ""}, BillingRatioRange{0.5, 0.5, "mixed"}},
		{"same-group-price", "", []string{`{"model_group_ratios":{"same-group-price":2}}`, ""}, BillingRatioRange{2, 2, "mixed"}},
		{"channel-range", "member", []string{`{"model_group_ratios":{"channel-range":0}}`, `{"model_group_ratios":{"channel-range":1}}`}, BillingRatioRange{0, 1, "channel_model"}},
		{"channel-same", "member", []string{`{"model_group_ratios":{"channel-same":1}}`, `{"model_group_ratios":{"channel-same":1}}`}, BillingRatioRange{1, 1, "channel_model"}},
		{"exact-name", "member", []string{`{"model_group_ratios":{"exact-*":0,"upstream":0}}`}, BillingRatioRange{0.5, 0.5, "user_group"}},
		{"invalid-legacy", "member", []string{`{"model_group_ratios":{"invalid-legacy":null}}`, `{"model_group_ratios":{"invalid-legacy":-1}}`, `{"model_group_ratios":{"invalid-legacy":1001}}`}, BillingRatioRange{0.5, 0.5, "user_group"}},
	}
	for _, tt := range tests {
		for i, settings := range tt.settings {
			// Lower priorities must remain in the price range as retry candidates.
			insertPricingBillingChannel(t, tt.name, "default", settings, common.ChannelStatusEnabled, true, int64(10-i))
		}
	}
	RefreshPricing()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found *Pricing
			for _, item := range GetPricingForUser(tt.userGroup, map[string]string{"default": "Default"}, nil) {
				if item.ModelName == tt.name {
					found = &item
					break
				}
			}
			require.NotNil(t, found)
			assert.Equal(t, map[string]BillingRatioRange{"default": tt.want}, found.BillingRatios)
		})
	}
}

func TestPricingBillingExcludesUnavailableCandidates(t *testing.T) {
	setupPricingBillingTest(t)
	insertPricingBillingChannel(t, "available", "default", `{"model_group_ratios":{"available":1}}`, common.ChannelStatusEnabled, true, 10)
	insertPricingBillingChannel(t, "available", "default", `{"model_group_ratios":{"available":99}}`, common.ChannelStatusManuallyDisabled, true, 20)
	insertPricingBillingChannel(t, "available", "default", `{"model_group_ratios":{"available":88}}`, common.ChannelStatusEnabled, false, 30)
	insertPricingBillingChannel(t, "disabled-channel", "default", "", common.ChannelStatusAutoDisabled, true, 10)
	insertPricingBillingChannel(t, "disabled-ability", "default", "", common.ChannelStatusEnabled, false, 10)
	insertPricingBillingChannel(t, "disabled-model", "default", "", common.ChannelStatusEnabled, true, 10)
	require.NoError(t, DB.Create(&Model{ModelName: "disabled-model", Status: 2, NameRule: NameRuleExact}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "default", Model: "orphan", ChannelId: 100000, Enabled: true}).Error)
	RefreshPricing()

	pricing := GetPricingForUser("member", map[string]string{"default": "Default"}, nil)
	require.Len(t, pricing, 1)
	assert.Equal(t, "available", pricing[0].ModelName)
	assert.Equal(t, BillingRatioRange{1, 1, "channel_model"}, pricing[0].BillingRatios["default"])
}

func TestPricingBillingUserIsolationAndVisibility(t *testing.T) {
	setupPricingBillingTest(t)
	insertPricingBillingChannel(t, "shared", "default", "", common.ChannelStatusEnabled, true, 10)
	insertPricingBillingChannel(t, "shared", "vip", `{"model_group_ratios":{"shared":1}}`, common.ChannelStatusEnabled, true, 10)
	insertPricingBillingChannel(t, "shared", "hidden", `{"model_group_ratios":{"shared":99}}`, common.ChannelStatusEnabled, true, 10)
	insertPricingBillingChannel(t, "hidden-only", "hidden", "", common.ChannelStatusEnabled, true, 10)
	RefreshPricing()
	visibleGroups := map[string]string{"default": "Default", "vip": "VIP"}

	member := GetPricingForUser("member", visibleGroups, nil)
	require.Len(t, member, 1)
	assert.ElementsMatch(t, []string{"default", "vip"}, member[0].EnableGroup)
	assert.Equal(t, map[string]BillingRatioRange{
		"default": {0.5, 0.5, "user_group"}, "vip": {1, 1, "channel_model"},
	}, member[0].BillingRatios)
	payload, err := common.Marshal(member)
	require.NoError(t, err)
	for _, secret := range []string{"channel_id", "private-pricing-test-key", "private-channel-name", "model_group_ratios", "billingCandidates", "setting", "hidden", "99"} {
		assert.NotContains(t, string(payload), secret)
	}

	member[0].EnableGroup[0] = "corrupted"
	member[0].SupportedEndpointTypes[0] = "corrupted"
	member[0].BillingRatios["default"] = BillingRatioRange{99, 99, "corrupted"}
	guest := GetPricingForUser("", map[string]string{"default": "Default"}, nil)
	require.Len(t, guest, 1)
	assert.Equal(t, []string{"default"}, guest[0].EnableGroup)
	assert.Equal(t, BillingRatioRange{2, 2, "group"}, guest[0].BillingRatios["default"])
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, guest[0].SupportedEndpointTypes)
	assert.Empty(t, GetPricingForUser("member", nil, nil))

	for _, price := range GetPricing() {
		assert.Nil(t, price.BillingRatios)
		assert.Nil(t, price.billingCandidates)
		if price.ModelName == "shared" {
			assert.ElementsMatch(t, []string{"default", "vip", "hidden"}, price.EnableGroup)
		}
	}
	// Current group settings are resolved per response, without refreshing channels.
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"default":0.25}}`))
	updated := GetPricingForUser("member", map[string]string{"default": "Default"}, nil)
	require.Len(t, updated, 1)
	assert.Equal(t, BillingRatioRange{0.25, 0.25, "user_group"}, updated[0].BillingRatios["default"])
}

func TestPricingBillingAllRemainsVisibilityMarker(t *testing.T) {
	setupPricingBillingTest(t)
	insertPricingBillingChannel(t, "all-model", "all", `{"model_group_ratios":{"all-model":0}}`, common.ChannelStatusEnabled, true, 10)
	insertPricingBillingChannel(t, "all-model", "default", "", common.ChannelStatusEnabled, true, 10)
	insertPricingBillingChannel(t, "all-only", "all", `{"model_group_ratios":{"all-only":0}}`, common.ChannelStatusEnabled, true, 10)
	RefreshPricing()

	for _, price := range GetPricingForUser("member", map[string]string{"default": "Default", "vip": "VIP"}, nil) {
		assert.Contains(t, price.EnableGroup, "all")
		if price.ModelName == "all-model" {
			assert.Equal(t, map[string]BillingRatioRange{"default": {0.5, 0.5, "user_group"}}, price.BillingRatios)
		} else {
			assert.Empty(t, price.BillingRatios)
			payload, err := common.Marshal(price)
			require.NoError(t, err)
			assert.NotContains(t, string(payload), "billing_ratios")
		}
	}
	prices := GetPricingForUser("member", map[string]string{"all": "All"}, nil)
	require.Len(t, prices, 2)
	for _, price := range prices {
		assert.Equal(t, map[string]BillingRatioRange{"all": {0, 0, "channel_model"}}, price.BillingRatios)
	}
}

func TestPricingBillingRefreshAndFailedFetch(t *testing.T) {
	setupPricingBillingTest(t)
	channel := insertPricingBillingChannel(t, "refreshable", "default", `{"model_group_ratios":{"refreshable":1}}`, common.ChannelStatusEnabled, true, 10)
	RefreshPricing()
	visible := map[string]string{"default": "Default"}
	before := GetPricingForUser("member", visible, nil)
	require.Len(t, before, 1)
	assert.Equal(t, BillingRatioRange{1, 1, "channel_model"}, before[0].BillingRatios["default"])

	require.NoError(t, DB.Model(channel).Update("setting", `{"model_group_ratios":{"refreshable":0}}`).Error)
	InvalidatePricingCache()
	after := GetPricingForUser("member", visible, nil)
	require.Len(t, after, 1)
	assert.Equal(t, BillingRatioRange{0, 0, "channel_model"}, after[0].BillingRatios["default"])
	assert.Equal(t, BillingRatioRange{1, 1, "channel_model"}, before[0].BillingRatios["default"])

	// A failed refresh must not replace known channel overrides with inheritance.
	emptyDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := emptyDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	originalDB := DB
	DB = emptyDB
	InvalidatePricingCache()
	failed := GetPricingForUser("member", visible, nil)
	DB = originalDB
	assert.Equal(t, after, failed)

	require.NoError(t, DB.Model(channel).Update("status", common.ChannelStatusManuallyDisabled).Error)
	RefreshPricing()
	assert.Empty(t, GetPricingForUser("member", visible, nil))
}

func TestPricingBillingSnapshotRefreshWithAndWithoutMemoryCache(t *testing.T) {
	for _, memoryCache := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory=%t", memoryCache), func(t *testing.T) {
			setupPricingBillingTest(t)
			common.MemoryCacheEnabled = memoryCache
			channel := insertPricingBillingChannel(t, "cached", "default", `{"model_group_ratios":{"cached":0}}`, common.ChannelStatusEnabled, true, 10)
			InitChannelCache()
			visible := map[string]string{"default": "Default"}
			initial := GetPricingForUser("member", visible, nil)
			require.Len(t, initial, 1)
			assert.Equal(t, BillingRatioRange{0, 0, "channel_model"}, initial[0].BillingRatios["default"])

			require.NoError(t, DB.Model(channel).Update("setting", `{"model_group_ratios":{}}`).Error)
			InitChannelCache()
			updated := GetPricingForUser("member", visible, nil)
			require.Len(t, updated, 1)
			assert.Equal(t, BillingRatioRange{0.5, 0.5, "user_group"}, updated[0].BillingRatios["default"])
		})
	}
}

func TestPricingPublicCopyDoesNotMutateCache(t *testing.T) {
	setupPricingBillingTest(t)
	cacheRatios := ratio_setting.CacheRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(`{"gpt-4o":0.5}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateCacheRatioByJSONString(cacheRatios)) })
	insertPricingBillingChannel(t, "gpt-4o", "default", "", common.ChannelStatusEnabled, true, 10)
	RefreshPricing()
	original := GetPricing()
	require.Len(t, original, 1)
	copy := GetPricing()
	require.NotNil(t, copy[0].CacheRatio)
	*copy[0].CacheRatio = 123
	copy[0].EnableGroup[0] = "corrupted"
	copy[0].SupportedEndpointTypes[0] = "corrupted"
	copy[0].ModelName = "corrupted"
	assert.Equal(t, original, GetPricing())
}

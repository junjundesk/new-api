package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tokenUsageQuotaData struct {
	TotalGranted      float64 `json:"total_granted"`
	TotalUsed         float64 `json:"total_used"`
	TotalAvailable    float64 `json:"total_available"`
	TotalGrantedQuota int64   `json:"total_granted_quota"`
	TotalUsedQuota    int64   `json:"total_used_quota"`
	TotalAvailQuota   int64   `json:"total_available_quota"`
	UnlimitedQuota    bool    `json:"unlimited_quota"`
	Currency          string  `json:"currency"`
	CurrencySymbol    string  `json:"currency_symbol"`
}

type tokenUsageAPIResponse struct {
	Code bool                `json:"code"`
	Data tokenUsageQuotaData `json:"data"`
}

type billingSubscriptionBody struct {
	Object       string  `json:"object"`
	HardLimitUSD float64 `json:"hard_limit_usd"`
	AccessUntil  int64   `json:"access_until"`
}

type billingUsageBody struct {
	Object     string  `json:"object"`
	TotalUsage float64 `json:"total_usage"`
}

// setQuotaDisplay 初始化站点额度展示配置，并在测试结束后恢复，避免影响其他用例。
func setQuotaDisplay(t *testing.T, displayType string, usdExchangeRate float64, quotaPerUnit float64) {
	t.Helper()
	previousType := operation_setting.GetGeneralSetting().QuotaDisplayType
	previousRate := operation_setting.USDExchangeRate
	previousPerUnit := common.QuotaPerUnit
	operation_setting.GetGeneralSetting().QuotaDisplayType = displayType
	operation_setting.USDExchangeRate = usdExchangeRate
	common.QuotaPerUnit = quotaPerUnit
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = previousType
		operation_setting.USDExchangeRate = previousRate
		common.QuotaPerUnit = previousPerUnit
	})
}

func createQuotaSourceFixture(t *testing.T, displayWalletQuota bool, unlimitedQuota bool) (*model.User, *model.Token) {
	t.Helper()
	db := setupTokenControllerTestDB(t)
	model.InitDatabaseColumnNames()
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))

	// 账户钱包：余额 5,000,000，已用 1,000,000。
	user := &model.User{
		Id:        211,
		Username:  "quota-source-user",
		Password:  "password",
		Group:     "default",
		Status:    common.UserStatusEnabled,
		Quota:     5000000,
		UsedQuota: 1000000,
	}
	require.NoError(t, db.Create(user).Error)

	// 密钥自身额度：剩余 1,000,000，已用 250,000。
	token := &model.Token{
		Id:                 311,
		UserId:             user.Id,
		Key:                "quotasourcekey",
		Name:               "quota-source-key",
		RemainQuota:        1000000,
		UsedQuota:          250000,
		UnlimitedQuota:     unlimitedQuota,
		DisplayWalletQuota: displayWalletQuota,
		ExpiredTime:        12345,
	}
	require.NoError(t, db.Create(token).Error)
	return user, token
}

func TestTokenUsageReportsQuotaSource(t *testing.T) {
	cases := []struct {
		name               string
		displayWalletQuota bool
		displayType        string
		usdExchangeRate    float64
		wantGranted        float64
		wantUsed           float64
		wantAvailable      float64
		wantGrantedQuota   int64
		wantUsedQuota      int64
		wantAvailQuota     int64
		wantCurrency       string
		wantSymbol         string
	}{
		{
			name:               "key quota in USD by default",
			displayWalletQuota: false,
			displayType:        operation_setting.QuotaDisplayTypeUSD,
			usdExchangeRate:    7,
			wantGranted:        2.5, // 1,250,000 / 500000
			wantUsed:           0.5, // 250,000 / 500000
			wantAvailable:      2,   // 1,000,000 / 500000
			wantGrantedQuota:   1250000,
			wantUsedQuota:      250000,
			wantAvailQuota:     1000000,
			wantCurrency:       "USD",
			wantSymbol:         "$",
		},
		{
			name:               "owner wallet quota in USD when wallet display is enabled",
			displayWalletQuota: true,
			displayType:        operation_setting.QuotaDisplayTypeUSD,
			usdExchangeRate:    7,
			wantGranted:        12, // 6,000,000 / 500000
			wantUsed:           2,  // 1,000,000 / 500000
			wantAvailable:      10, // 5,000,000 / 500000
			wantGrantedQuota:   6000000,
			wantUsedQuota:      1000000,
			wantAvailQuota:     5000000,
			wantCurrency:       "USD",
			wantSymbol:         "$",
		},
		{
			name:               "key quota converted to CNY",
			displayWalletQuota: false,
			displayType:        operation_setting.QuotaDisplayTypeCNY,
			usdExchangeRate:    7,
			wantGranted:        17.5, // 2.5 * 7
			wantUsed:           3.5,  // 0.5 * 7
			wantAvailable:      14,   // 2 * 7
			wantGrantedQuota:   1250000,
			wantUsedQuota:      250000,
			wantAvailQuota:     1000000,
			wantCurrency:       "CNY",
			wantSymbol:         "¥",
		},
		{
			name:               "owner wallet quota converted to CNY",
			displayWalletQuota: true,
			displayType:        operation_setting.QuotaDisplayTypeCNY,
			usdExchangeRate:    7,
			wantGranted:        84, // 12 * 7
			wantUsed:           14, // 2 * 7
			wantAvailable:      70, // 10 * 7
			wantGrantedQuota:   6000000,
			wantUsedQuota:      1000000,
			wantAvailQuota:     5000000,
			wantCurrency:       "CNY",
			wantSymbol:         "¥",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setQuotaDisplay(t, tc.displayType, tc.usdExchangeRate, 500000)
			_, token := createQuotaSourceFixture(t, tc.displayWalletQuota, true)
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/usage/token/", nil, 0)
			ctx.Request.Header.Set("Authorization", "Bearer sk-"+token.Key)

			GetTokenUsage(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var body tokenUsageAPIResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
			require.True(t, body.Code)
			assert.InDelta(t, tc.wantGranted, body.Data.TotalGranted, 1e-9)
			assert.InDelta(t, tc.wantUsed, body.Data.TotalUsed, 1e-9)
			assert.InDelta(t, tc.wantAvailable, body.Data.TotalAvailable, 1e-9)
			assert.Equal(t, tc.wantGrantedQuota, body.Data.TotalGrantedQuota)
			assert.Equal(t, tc.wantUsedQuota, body.Data.TotalUsedQuota)
			assert.Equal(t, tc.wantAvailQuota, body.Data.TotalAvailQuota)
			assert.Equal(t, tc.wantCurrency, body.Data.Currency)
			assert.Equal(t, tc.wantSymbol, body.Data.CurrencySymbol)
			assert.Equal(t, !tc.displayWalletQuota, body.Data.UnlimitedQuota)
		})
	}
}

func TestBillingDashboardReportsQuotaSource(t *testing.T) {
	previousDisplayTokenStat := common.DisplayTokenStatEnabled
	common.DisplayTokenStatEnabled = true
	setQuotaDisplay(t, operation_setting.QuotaDisplayTypeUSD, 7, 500000)
	t.Cleanup(func() {
		common.DisplayTokenStatEnabled = previousDisplayTokenStat
	})

	cases := []struct {
		name               string
		displayWalletQuota bool
		wantHardLimitUSD   float64
		wantAccessUntil    int64
		wantTotalUsage     float64
	}{
		{
			name:               "key stats when wallet display is disabled",
			displayWalletQuota: false,
			wantHardLimitUSD:   2.5, // (1,000,000+250,000)/500000
			wantAccessUntil:    12345,
			wantTotalUsage:     50, // 250,000/500000*100
		},
		{
			name:               "owner wallet stats when wallet display is enabled",
			displayWalletQuota: true,
			wantHardLimitUSD:   12, // (5,000,000+1,000,000)/500000
			wantAccessUntil:    0,
			wantTotalUsage:     200, // 1,000,000/500000*100
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user, token := createQuotaSourceFixture(t, tc.displayWalletQuota, false)

			subscriptionCtx, subscriptionRecorder := newAuthenticatedContext(t, http.MethodGet, "/v1/dashboard/billing/subscription", nil, user.Id)
			subscriptionCtx.Set("token_id", token.Id)
			GetSubscription(subscriptionCtx)

			var subscription billingSubscriptionBody
			require.NoError(t, common.Unmarshal(subscriptionRecorder.Body.Bytes(), &subscription))
			assert.Equal(t, "billing_subscription", subscription.Object)
			assert.InDelta(t, tc.wantHardLimitUSD, subscription.HardLimitUSD, 1e-9)
			assert.Equal(t, tc.wantAccessUntil, subscription.AccessUntil)

			usageCtx, usageRecorder := newAuthenticatedContext(t, http.MethodGet, "/v1/dashboard/billing/usage", nil, user.Id)
			usageCtx.Set("token_id", token.Id)
			GetUsage(usageCtx)

			var usage billingUsageBody
			require.NoError(t, common.Unmarshal(usageRecorder.Body.Bytes(), &usage))
			assert.Equal(t, "list", usage.Object)
			assert.InDelta(t, tc.wantTotalUsage, usage.TotalUsage, 1e-9)
		})
	}
}

func TestTokenMutationsPersistDisplayWalletQuota(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	model.InitDatabaseColumnNames()
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))
	user := &model.User{
		Id:       221,
		Username: "wallet-display-mutation-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	createPayload := map[string]any{
		"name":                 "wallet-display-key",
		"expired_time":         -1,
		"remain_quota":         0,
		"unlimited_quota":      true,
		"display_wallet_quota": true,
		"group":                "default",
	}
	createCtx, createRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", createPayload, user.Id)
	AddToken(createCtx)
	require.True(t, decodeAPIResponse(t, createRecorder).Success, "token creation must succeed")

	var created model.Token
	require.NoError(t, db.Where("name = ?", "wallet-display-key").First(&created).Error)
	assert.True(t, created.DisplayWalletQuota)

	updatePayload := map[string]any{
		"id":                   created.Id,
		"name":                 created.Name,
		"expired_time":         -1,
		"remain_quota":         0,
		"unlimited_quota":      true,
		"display_wallet_quota": false,
		"group":                "default",
	}
	updateCtx, updateRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", updatePayload, user.Id)
	UpdateToken(updateCtx)
	require.True(t, decodeAPIResponse(t, updateRecorder).Success, "token update must succeed")

	var updated model.Token
	require.NoError(t, db.First(&updated, created.Id).Error)
	assert.False(t, updated.DisplayWalletQuota)
}

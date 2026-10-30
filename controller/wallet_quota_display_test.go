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
	TotalGranted   int  `json:"total_granted"`
	TotalUsed      int  `json:"total_used"`
	TotalAvailable int  `json:"total_available"`
	UnlimitedQuota bool `json:"unlimited_quota"`
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

func createQuotaSourceFixture(t *testing.T, displayWalletQuota bool, unlimitedQuota bool) (*model.User, *model.Token) {
	t.Helper()
	db := setupTokenControllerTestDB(t)
	model.InitDatabaseColumnNames()
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))

	user := &model.User{
		Id:        211,
		Username:  "quota-source-user",
		Password:  "password",
		Group:     "default",
		Status:    common.UserStatusEnabled,
		Quota:     900,
		UsedQuota: 100,
	}
	require.NoError(t, db.Create(user).Error)
	token := &model.Token{
		Id:                 311,
		UserId:             user.Id,
		Key:                "quotasourcekey",
		Name:               "quota-source-key",
		RemainQuota:        30,
		UsedQuota:          70,
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
		wantGranted        int
		wantUsed           int
		wantAvailable      int
	}{
		{
			name:               "key quota when wallet display is disabled",
			displayWalletQuota: false,
			wantGranted:        100,
			wantUsed:           70,
			wantAvailable:      30,
		},
		{
			name:               "owner wallet quota when wallet display is enabled",
			displayWalletQuota: true,
			wantGranted:        1000,
			wantUsed:           100,
			wantAvailable:      900,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, token := createQuotaSourceFixture(t, tc.displayWalletQuota, true)
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/usage/token/", nil, 0)
			ctx.Request.Header.Set("Authorization", "Bearer sk-"+token.Key)

			GetTokenUsage(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var body tokenUsageAPIResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
			require.True(t, body.Code)
			assert.Equal(t, tc.wantGranted, body.Data.TotalGranted)
			assert.Equal(t, tc.wantUsed, body.Data.TotalUsed)
			assert.Equal(t, tc.wantAvailable, body.Data.TotalAvailable)
			assert.Equal(t, !tc.displayWalletQuota, body.Data.UnlimitedQuota)
		})
	}
}

func TestBillingDashboardReportsQuotaSource(t *testing.T) {
	previousDisplayTokenStat := common.DisplayTokenStatEnabled
	previousQuotaPerUnit := common.QuotaPerUnit
	previousDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	common.DisplayTokenStatEnabled = true
	common.QuotaPerUnit = 500000
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	t.Cleanup(func() {
		common.DisplayTokenStatEnabled = previousDisplayTokenStat
		common.QuotaPerUnit = previousQuotaPerUnit
		operation_setting.GetGeneralSetting().QuotaDisplayType = previousDisplayType
	})

	cases := []struct {
		name               string
		displayWalletQuota bool
		// hard_limit 单位美元，total_usage 单位美分。
		wantHardLimitUSD float64
		wantAccessUntil  int64
		wantTotalUsage   float64
	}{
		{
			name:               "key stats when wallet display is disabled",
			displayWalletQuota: false,
			wantHardLimitUSD:   0.0002, // (30+70)/500000
			wantAccessUntil:    12345,
			wantTotalUsage:     0.014, // 70/500000*100
		},
		{
			name:               "owner wallet stats when wallet display is enabled",
			displayWalletQuota: true,
			wantHardLimitUSD:   0.002, // (900+100)/500000
			wantAccessUntil:    0,
			wantTotalUsage:     0.02, // 100/500000*100
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
			assert.InDelta(t, tc.wantHardLimitUSD, subscription.HardLimitUSD, 1e-12)
			assert.Equal(t, tc.wantAccessUntil, subscription.AccessUntil)

			usageCtx, usageRecorder := newAuthenticatedContext(t, http.MethodGet, "/v1/dashboard/billing/usage", nil, user.Id)
			usageCtx.Set("token_id", token.Id)
			GetUsage(usageCtx)

			var usage billingUsageBody
			require.NoError(t, common.Unmarshal(usageRecorder.Body.Bytes(), &usage))
			assert.Equal(t, "list", usage.Object)
			assert.InDelta(t, tc.wantTotalUsage, usage.TotalUsage, 1e-12)
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

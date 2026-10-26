package helper

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	kitcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newFallbackTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, recorder
}

func fallbackTestError(code types.ErrorCode, status int, options ...types.NewAPIErrorOptions) *types.NewAPIError {
	return types.NewErrorWithStatusCode(errors.New("test failure"), code, status, options...)
}

func newFallbackTriggerInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		UsingGroup:      "default",
		RelayFormat:     types.RelayFormatOpenAI,
		Fallback: &relaycommon.FallbackState{
			OriginalModel: "gpt-5",
			OriginalGroup: "default",
			Configured: []relaycommon.FallbackTarget{
				{Group: "vip", Model: "claude-sonnet-4-5"},
			},
		},
	}
}

func TestBuildFallbackStateCopiesOrderedConfiguration(t *testing.T) {
	c, _ := newFallbackTestContext(t)
	configured := []setting.TokenFallbackModel{
		{Group: "default", Model: "gpt-4.1-mini"},
		{SourceModel: "mapped-model", Group: "vip", Model: "claude-sonnet-4-5"},
	}
	kitcommon.SetContextKey(c, constant.ContextKeyTokenFallbackModels, configured)
	info := &relaycommon.RelayInfo{OriginModelName: "mapped-model", UsingGroup: "auto"}

	state := BuildFallbackState(c, info)
	require.NotNil(t, state)
	assert.Equal(t, "mapped-model", state.OriginalModel)
	assert.Equal(t, "auto", state.OriginalGroup)
	assert.Equal(t, []relaycommon.FallbackTarget{
		{Group: "default", Model: "gpt-4.1-mini"},
		{SourceModel: "mapped-model", Group: "vip", Model: "claude-sonnet-4-5"},
	}, state.Configured)

	configured[0].Model = "changed-after-authentication"
	assert.Equal(t, "gpt-4.1-mini", state.Configured[0].Model, "request state must not alias mutable token configuration")

	kitcommon.SetContextKey(c, constant.ContextKeyTokenFallbackModels, []setting.TokenFallbackModel{})
	assert.Nil(t, BuildFallbackState(c, info))
}

func TestMatchingFallbackTargetsUsesExactAndGenericRulesInOrder(t *testing.T) {
	state := &relaycommon.FallbackState{
		OriginalModel: "mapped-model",
		Configured: []relaycommon.FallbackTarget{
			{SourceModel: "other-model", Group: "other", Model: "must-not-run"},
			{Group: "default", Model: "generic-first"},
			{SourceModel: "mapped-model", Group: "vip", Model: "exact-second"},
			{Group: "azure", Model: "generic-third"},
		},
	}
	assert.Equal(t, []relaycommon.FallbackTarget{
		{Group: "default", Model: "generic-first"},
		{SourceModel: "mapped-model", Group: "vip", Model: "exact-second"},
		{Group: "azure", Model: "generic-third"},
	}, MatchingFallbackTargets(state))

	state.OriginalModel = "unconfigured-model"
	assert.Equal(t, []relaycommon.FallbackTarget{
		{Group: "default", Model: "generic-first"},
		{Group: "azure", Model: "generic-third"},
	}, MatchingFallbackTargets(state))
	assert.Nil(t, MatchingFallbackTargets(nil))
}

func TestShouldTryFallbackEligibleErrorMatrix(t *testing.T) {
	tests := []struct {
		name    string
		code    types.ErrorCode
		status  int
		options []types.NewAPIErrorOptions
	}{
		{name: "no available channel even with skip retry", code: types.ErrorCodeGetChannelFailed, status: 0, options: []types.NewAPIErrorOptions{types.ErrOptionWithSkipRetry()}},
		{name: "upstream server error", code: types.ErrorCodeBadResponseStatusCode, status: http.StatusServiceUnavailable},
		{name: "upstream rate limit", code: types.ErrorCodeBadResponseStatusCode, status: http.StatusTooManyRequests},
		{name: "upstream timeout status", code: types.ErrorCodeBadResponseStatusCode, status: http.StatusRequestTimeout},
		{name: "upstream model not found", code: types.ErrorCodeBadResponseStatusCode, status: http.StatusNotFound},
		{name: "upstream gateway timeout even with skip retry", code: types.ErrorCodeBadResponseStatusCode, status: http.StatusGatewayTimeout, options: []types.NewAPIErrorOptions{types.ErrOptionWithSkipRetry()}},
		{name: "bad upstream response", code: types.ErrorCodeBadResponse, status: 0},
		{name: "bad upstream response body", code: types.ErrorCodeBadResponseBody, status: 0},
		{name: "empty upstream response", code: types.ErrorCodeEmptyResponse, status: 0},
		{name: "upstream connection failure", code: types.ErrorCodeDoRequestFailed, status: 0},
		{name: "model not found error", code: types.ErrorCodeModelNotFound, status: http.StatusNotFound},
		{name: "channel response timeout", code: types.ErrorCodeChannelResponseTimeExceeded, status: 0},
		{name: "aws client failure", code: types.ErrorCodeChannelAwsClientError, status: 0},
		{name: "unclassified upstream outage", code: types.ErrorCode("upstream_unavailable"), status: http.StatusBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newFallbackTestContext(t)
			assert.True(t, ShouldTryFallback(c, newFallbackTriggerInfo(), fallbackTestError(tt.code, tt.status, tt.options...)))
		})
	}
}

func TestShouldTryFallbackRejectsClientAndPolicyErrors(t *testing.T) {
	// NewError defaults these errors to status 500. Error-code exclusions must
	// therefore take precedence over the otherwise eligible HTTP status.
	for _, code := range []types.ErrorCode{
		types.ErrorCodeInvalidRequest,
		types.ErrorCodeAccessDenied,
		types.ErrorCodeSensitiveWordsDetected,
		types.ErrorCodeInsufficientUserQuota,
		types.ErrorCodePreConsumeTokenQuotaFailed,
		types.ErrorCodePromptBlocked,
	} {
		t.Run(string(code), func(t *testing.T) {
			c, _ := newFallbackTestContext(t)
			err := types.NewError(errors.New("request must not be retried with another model"), code)
			assert.False(t, ShouldTryFallback(c, newFallbackTriggerInfo(), err))
		})
	}
}

func TestShouldTryFallbackRejectsClientHTTPStatuses(t *testing.T) {
	for _, status := range []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusRequestEntityTooLarge,
		http.StatusUnprocessableEntity,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			c, _ := newFallbackTestContext(t)
			err := fallbackTestError(types.ErrorCodeBadResponseStatusCode, status)
			assert.False(t, ShouldTryFallback(c, newFallbackTriggerInfo(), err))
		})
	}
}

func TestShouldTryFallbackRequiresUnstartedResponseAndSupportedRequest(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		assert.False(t, ShouldTryFallback(c, newFallbackTriggerInfo(), nil))
	})
	t.Run("no configured target", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		info := newFallbackTriggerInfo()
		info.Fallback.Configured = nil
		assert.False(t, ShouldTryFallback(c, info, fallbackTestError(types.ErrorCodeBadResponse, 0)))
	})
	t.Run("no target matches original model", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		info := newFallbackTriggerInfo()
		info.Fallback.Configured[0].SourceModel = "another-model"
		assert.False(t, ShouldTryFallback(c, info, fallbackTestError(types.ErrorCodeBadResponse, 0)))
	})
	t.Run("channel test", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		info := newFallbackTriggerInfo()
		info.IsChannelTest = true
		assert.False(t, ShouldTryFallback(c, info, fallbackTestError(types.ErrorCodeBadResponse, 0)))
	})
	t.Run("response body already written", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		c.Writer.WriteHeaderNow()
		assert.False(t, ShouldTryFallback(c, newFallbackTriggerInfo(), fallbackTestError(types.ErrorCodeBadResponse, 0)))
	})
	t.Run("response chunk already sent", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		info := newFallbackTriggerInfo()
		info.SendResponseCount = 1
		assert.False(t, ShouldTryFallback(c, info, fallbackTestError(types.ErrorCodeBadResponse, 0)))
	})
	t.Run("upstream output already received", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		info := newFallbackTriggerInfo()
		info.ReceivedResponseCount = 1
		assert.False(t, ShouldTryFallback(c, info, fallbackTestError(types.ErrorCodeBadResponse, 0)), "received output must not be silently discarded by a fallback")
	})
}

func TestShouldTryFallbackRelayFormatMatrix(t *testing.T) {
	for _, format := range []types.RelayFormat{
		types.RelayFormatOpenAI,
		types.RelayFormatClaude,
		types.RelayFormatGemini,
		types.RelayFormatOpenAIResponses,
		types.RelayFormatEmbedding,
	} {
		t.Run("supported_"+string(format), func(t *testing.T) {
			c, _ := newFallbackTestContext(t)
			info := newFallbackTriggerInfo()
			info.RelayFormat = format
			assert.True(t, ShouldTryFallback(c, info, fallbackTestError(types.ErrorCodeBadResponse, 0)))
		})
	}
	for _, format := range []types.RelayFormat{
		types.RelayFormatOpenAIImage,
		types.RelayFormatOpenAIAudio,
		types.RelayFormatOpenAIRealtime,
		types.RelayFormatRerank,
		types.RelayFormatTask,
	} {
		t.Run("unsupported_"+string(format), func(t *testing.T) {
			c, _ := newFallbackTestContext(t)
			info := newFallbackTriggerInfo()
			info.RelayFormat = format
			assert.False(t, ShouldTryFallback(c, info, fallbackTestError(types.ErrorCodeBadResponse, 0)))
		})
	}
}

func TestPrepareFallbackTargetSwitchesRequestAndAttemptState(t *testing.T) {
	c, _ := newFallbackTestContext(t)
	request := &dto.GeneralOpenAIRequest{Model: "old-model"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "old-model",
		UsingGroup:      "old-group",
		Request:         request,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "old-upstream-model",
			IsModelMapped:     true,
		},
		ModelCandidate:        &relaycommon.ModelCandidateState{ChannelId: 7, Index: 1},
		PriceData:             hosttypes.PriceData{BillingModelName: "old-model", ModelRatio: 2, QuotaToPreConsume: 100},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{ExprString: "old-expression"},
		BillingRequestInput:   &billingexpr.RequestInput{},
	}
	kitcommon.SetContextKey(c, constant.ContextKeyUsingGroup, "old-group")
	kitcommon.SetContextKey(c, constant.ContextKeyAutoGroup, "old-group")
	kitcommon.SetContextKey(c, constant.ContextKeyAutoGroupIndex, 3)
	kitcommon.SetContextKey(c, constant.ContextKeyAutoGroupRetryIndex, 2)

	target := relaycommon.FallbackTarget{Group: "vip", Model: "claude-sonnet-4-5"}
	PrepareFallbackTarget(c, info, target)

	assert.Equal(t, target.Model, info.OriginModelName)
	assert.Equal(t, target.Group, info.UsingGroup)
	assert.Equal(t, target.Model, info.UpstreamModelName)
	assert.False(t, info.IsModelMapped)
	assert.Nil(t, info.ModelCandidate)
	assert.Equal(t, target.Model, request.Model)
	assert.Equal(t, target.Model, c.GetString("original_model"))
	assert.Equal(t, target.Group, kitcommon.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	assert.Equal(t, target.Group, kitcommon.GetContextKeyString(c, constant.ContextKeyAutoGroup))
	assert.Zero(t, kitcommon.GetContextKeyInt(c, constant.ContextKeyAutoGroupIndex))
	assert.Zero(t, kitcommon.GetContextKeyInt(c, constant.ContextKeyAutoGroupRetryIndex))
	assert.Zero(t, info.PriceData)
	assert.Nil(t, info.TieredBillingSnapshot)
	assert.Nil(t, info.BillingRequestInput)
}

func TestShouldTryFallbackRejectsCanceledRequest(t *testing.T) {
	c, _ := newFallbackTestContext(t)
	requestContext, cancel := context.WithCancel(context.Background())
	c.Request = c.Request.WithContext(requestContext)
	cancel()

	err := fallbackTestError(types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable)
	assert.False(t, ShouldTryFallback(c, newFallbackTriggerInfo(), err),
		"a canceled client request must not spend quota starting another upstream call")
}

func TestPrepareFallbackTargetRewritesPassThroughBody(t *testing.T) {
	c, _ := newFallbackTestContext(t)
	body := []byte("{\"model\":\"gpt-5\",\"messages\":[]}")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	info := &relaycommon.RelayInfo{OriginModelName: "gpt-5", RelayFormat: types.RelayFormatOpenAI}

	PrepareFallbackTarget(c, info, relaycommon.FallbackTarget{Group: "vip", Model: "claude-sonnet-4-5"})

	storage, err := kitcommon.GetBodyStorage(c)
	require.NoError(t, err)
	updated, err := storage.Bytes()
	require.NoError(t, err)
	var parsed map[string]interface{}
	require.NoError(t, kitcommon.Unmarshal(updated, &parsed))
	assert.Equal(t, "claude-sonnet-4-5", parsed["model"],
		"pass-through channels replay the cached body, so the fallback model must replace the original")
	assert.Equal(t, int64(len(updated)), c.Request.ContentLength)
}

func TestPrepareFallbackTargetKeepsBodyWithoutModelField(t *testing.T) {
	c, _ := newFallbackTestContext(t)
	body := []byte("{\"messages\":[]}")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	info := &relaycommon.RelayInfo{OriginModelName: "gpt-5", RelayFormat: types.RelayFormatOpenAI}

	PrepareFallbackTarget(c, info, relaycommon.FallbackTarget{Group: "vip", Model: "claude-sonnet-4-5"})

	storage, err := kitcommon.GetBodyStorage(c)
	require.NoError(t, err)
	updated, err := storage.Bytes()
	require.NoError(t, err)
	assert.JSONEq(t, string(body), string(updated), "a body without a model field must not be rewritten")
}

func TestPrepareFallbackTargetInitializesMissingChannelMeta(t *testing.T) {
	c, _ := newFallbackTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName: "old-model",
		Request:         &dto.GeneralOpenAIRequest{Model: "old-model"},
	}
	target := relaycommon.FallbackTarget{Group: "vip", Model: "claude-sonnet-4-5"}

	require.NotPanics(t, func() {
		PrepareFallbackTarget(c, info, target)
	}, "a failed first channel attempt has not initialized ChannelMeta yet")
	require.NotNil(t, info.ChannelMeta)
	assert.Equal(t, target.Model, info.UpstreamModelName)
}

func TestCheckFallbackTargetEnforcesCurrentGroupAndModelRules(t *testing.T) {
	previousDB := model.DB
	previousMainType := kitcommon.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	kitcommon.SetMainDatabaseType(kitcommon.DatabaseTypeSQLite)
	model.InitDatabaseColumnNames()
	previousGroups := setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP","auto":"Auto","chain:7":"Chain"}`))
	t.Cleanup(func() {
		model.DB = previousDB
		kitcommon.SetMainDatabaseType(previousMainType)
		model.InitDatabaseColumnNames()
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	for _, ability := range []model.Ability{
		{Group: "vip", Model: "backup-model", ChannelId: 9101, Enabled: true},
		{Group: "auto", Model: "backup-model", ChannelId: 9102, Enabled: true},
		{Group: "chain:7", Model: "backup-model", ChannelId: 9103, Enabled: true},
		{Group: "vip", Model: "gpt-4-gizmo-123", ChannelId: 9104, Enabled: true},
	} {
		require.NoError(t, db.Create(&ability).Error)
	}
	info := &relaycommon.RelayInfo{UserGroup: "default"}

	t.Run("enabled concrete group and model", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		assert.NoError(t, CheckFallbackTarget(c, info, relaycommon.FallbackTarget{Group: "vip", Model: "backup-model"}))
	})
	t.Run("group no longer available to user", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		err := CheckFallbackTarget(c, info, relaycommon.FallbackTarget{Group: "secret", Model: "backup-model"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available")
	})
	t.Run("model no longer enabled in target group", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		err := CheckFallbackTarget(c, info, relaycommon.FallbackTarget{Group: "vip", Model: "removed-model"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available in group")
	})
	t.Run("auto is not a concrete fallback target", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		err := CheckFallbackTarget(c, info, relaycommon.FallbackTarget{Group: "auto", Model: "backup-model"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available")
	})
	t.Run("channel chain is not a concrete fallback target", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		err := CheckFallbackTarget(c, info, relaycommon.FallbackTarget{Group: "chain:7", Model: "backup-model"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available")
	})
	t.Run("key model limit is enforced", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		kitcommon.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
		kitcommon.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"another-model": true})
		err := CheckFallbackTarget(c, info, relaycommon.FallbackTarget{Group: "vip", Model: "backup-model"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed by this key")
	})
	t.Run("key model limit accepts its wildcard equivalent", func(t *testing.T) {
		c, _ := newFallbackTestContext(t)
		kitcommon.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
		kitcommon.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"gpt-4-gizmo-*": true})
		assert.NoError(t, CheckFallbackTarget(c, info, relaycommon.FallbackTarget{Group: "vip", Model: "gpt-4-gizmo-123"}))
	})
}

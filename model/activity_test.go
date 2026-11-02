package model

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const activityTestGroup = "activity-test-group"

// setupActivityFixture 清理并准备活动码测试数据：一个用户、一个可用的目标分组、一张活动码。
func setupActivityFixture(t *testing.T, code ActivityCode, userQuota int) (*User, *ActivityCode) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &ActivityCode{}, &ActivityGrant{}, &UserBonusGrant{}, &Log{}))
	reset := func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&ActivityCode{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&ActivityGrant{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&UserBonusGrant{}).Error)
		DB.Exec("DELETE FROM users")
		DB.Exec("DELETE FROM logs")
	}
	reset()
	t.Cleanup(reset)

	// 目标分组必须存在，否则兑换会被判为分组不可用。
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"activity-test-group":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})

	user := &User{Username: "activity-user", Password: "password", Status: common.UserStatusEnabled, Quota: userQuota}
	require.NoError(t, DB.Create(user).Error)

	if code.Code == "" {
		code.Code = "act-test-code"
	}
	if code.Name == "" {
		code.Name = "activity test code"
	}
	if code.GroupName == "" {
		code.GroupName = activityTestGroup
	}
	if code.RatioMode == "" {
		code.RatioMode = ActivityRatioModeInherit
	}
	if code.PerUserLimit == 0 {
		code.PerUserLimit = 1
	}
	if code.Status == 0 {
		code.Status = ActivityCodeStatusEnabled
	}
	require.NoError(t, DB.Create(&code).Error)
	return user, &code
}

func reloadActivityCode(t *testing.T, codeId int) *ActivityCode {
	t.Helper()
	code := &ActivityCode{}
	require.NoError(t, DB.First(code, "id = ?", codeId).Error)
	return code
}

func reloadUserQuota(t *testing.T, userId int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.First(&user, "id = ?", userId).Error)
	return user.Quota
}

func TestRedeemActivityCodeFreeGrantCreatesGrant(t *testing.T) {
	code := ActivityCode{Code: "hdmfree", Name: "免费活动", DurationSeconds: 24 * 3600, RatioMode: ActivityRatioModeOverride, RatioOverride: 0}
	user, created := setupActivityFixture(t, code, 0)

	result, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "HDMFree", UserId: user.Id})
	require.NoError(t, err)
	assert.Equal(t, activityTestGroup, result.GroupName)
	assert.Equal(t, 0, result.PriceQuota)
	assert.Equal(t, 0, result.BalanceAfter, "免费活动码不改变余额")
	require.Greater(t, result.ExpireTime, common.GetTimestamp())

	grant := &ActivityGrant{}
	require.NoError(t, DB.First(grant, "id = ?", result.GrantId).Error)
	assert.Equal(t, user.Id, grant.UserId)
	assert.Equal(t, created.Id, grant.CodeId)
	assert.Equal(t, activityTestGroup, grant.GroupName)
	assert.Equal(t, ActivityRatioModeOverride, grant.RatioMode)
	assert.Equal(t, float64(0), grant.RatioOverride)
	assert.Equal(t, ActivityGrantStatusActive, grant.Status)

	assert.Equal(t, 1, reloadActivityCode(t, created.Id).UsedCount)

	grants, err := GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, result.GrantId, grants[0].Id)
}

func TestRedeemActivityCodeChargesWalletQuotaNotBonus(t *testing.T) {
	code := ActivityCode{Code: "hdmpaid", Name: "付费活动", DurationSeconds: 3600, PriceQuota: 100}
	user, created := setupActivityFixture(t, code, 1000)
	bonus, err := GrantBonus(user.Id, 5000, 0, BonusSourceAdmin, "activity test bonus")
	require.NoError(t, err)

	result, err := RedeemActivityCode(RedeemActivityCodeRequest{
		Code:               "hdmpaid",
		UserId:             user.Id,
		Confirm:            true,
		ExpectedPriceQuota: 100,
	})
	require.NoError(t, err)
	assert.Equal(t, 100, result.PriceQuota)
	assert.Equal(t, 900, result.BalanceAfter)

	assert.Equal(t, 900, reloadUserQuota(t, user.Id), "只扣真余额")
	reloadedBonus := &UserBonusGrant{}
	require.NoError(t, DB.First(reloadedBonus, "id = ?", bonus.Id).Error)
	assert.Equal(t, int64(0), reloadedBonus.AmountUsed, "赠金不参与兑换")
	assert.Equal(t, 1, reloadActivityCode(t, created.Id).UsedCount)
}

func TestRedeemActivityCodeRejectsWhenWalletQuotaTooLow(t *testing.T) {
	code := ActivityCode{Code: "hdmpaid2", Name: "付费活动", DurationSeconds: 3600, PriceQuota: 100}
	user, created := setupActivityFixture(t, code, 50)
	_, err := GrantBonus(user.Id, 100000, 0, BonusSourceAdmin, "activity test bonus")
	require.NoError(t, err)

	_, err = RedeemActivityCode(RedeemActivityCodeRequest{
		Code:               "hdmpaid2",
		UserId:             user.Id,
		Confirm:            true,
		ExpectedPriceQuota: 100,
	})
	require.ErrorIs(t, err, ErrActivityCodeBalanceNotEnough)

	assert.Equal(t, 50, reloadUserQuota(t, user.Id), "失败时余额不变")
	assert.Equal(t, 0, reloadActivityCode(t, created.Id).UsedCount, "失败时事务回滚，兑换次数不递增")
	var grantCount int64
	require.NoError(t, DB.Model(&ActivityGrant{}).Where("user_id = ?", user.Id).Count(&grantCount).Error)
	assert.Equal(t, int64(0), grantCount, "失败时不产生授权")
}

func TestRedeemActivityCodeRequiresConfirmationAndStablePrice(t *testing.T) {
	code := ActivityCode{Code: "hdmconfirm", Name: "付费活动", DurationSeconds: 3600, PriceQuota: 100}
	user, created := setupActivityFixture(t, code, 1000)

	_, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "hdmconfirm", UserId: user.Id})
	require.ErrorIs(t, err, ErrActivityCodeNeedConfirm)
	assert.Equal(t, 1000, reloadUserQuota(t, user.Id))
	assert.Equal(t, 0, reloadActivityCode(t, created.Id).UsedCount)

	_, err = RedeemActivityCode(RedeemActivityCodeRequest{
		Code:               "hdmconfirm",
		UserId:             user.Id,
		Confirm:            true,
		ExpectedPriceQuota: 90,
	})
	require.ErrorIs(t, err, ErrActivityCodePriceChanged)
	assert.Equal(t, 1000, reloadUserQuota(t, user.Id))
	assert.Equal(t, 0, reloadActivityCode(t, created.Id).UsedCount)
}

func TestRedeemActivityCodeRejectsInvalidStates(t *testing.T) {
	now := common.GetTimestamp()
	tests := []struct {
		name    string
		code    ActivityCode
		prepare func(t *testing.T, user *User, code *ActivityCode)
		wantErr error
	}{
		{
			name:    "disabled code",
			code:    ActivityCode{Code: "act-disabled", Status: ActivityCodeStatusDisabled},
			wantErr: ErrActivityCodeDisabled,
		},
		{
			name:    "not started yet",
			code:    ActivityCode{Code: "act-future", StartTime: now + 3600},
			wantErr: ErrActivityCodeNotStarted,
		},
		{
			name:    "window already closed",
			code:    ActivityCode{Code: "act-ended", EndTime: now - 10},
			wantErr: ErrActivityCodeEnded,
		},
		{
			name:    "target group is not configured",
			code:    ActivityCode{Code: "act-nogroup", GroupName: "missing-group"},
			wantErr: ErrActivityCodeGroupUnavailable,
		},
		{
			name: "total limit reached",
			code: ActivityCode{Code: "act-soldout", TotalLimit: 1},
			prepare: func(t *testing.T, user *User, code *ActivityCode) {
				require.NoError(t, DB.Model(&ActivityCode{}).Where("id = ?", code.Id).
					Update("used_count", 1).Error)
			},
			wantErr: ErrActivityCodeSoldOut,
		},
		{
			name: "per user limit reached",
			code: ActivityCode{Code: "act-userlimit", PerUserLimit: 1},
			prepare: func(t *testing.T, user *User, code *ActivityCode) {
				require.NoError(t, DB.Create(&ActivityGrant{
					UserId: user.Id, CodeId: code.Id, Code: code.Code, GroupName: activityTestGroup,
					Status: ActivityGrantStatusActive, StartTime: now,
				}).Error)
			},
			wantErr: ErrActivityCodeUserLimitReached,
		},
		{
			name:    "unknown code",
			code:    ActivityCode{Code: "act-known"},
			wantErr: ErrActivityCodeInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, created := setupActivityFixture(t, tt.code, 0)
			if tt.prepare != nil {
				tt.prepare(t, user, created)
			}
			var beforeCount int64
			require.NoError(t, DB.Model(&ActivityGrant{}).Where("user_id = ? AND code_id = ?", user.Id, created.Id).Count(&beforeCount).Error)

			redeemCode := tt.code.Code
			if tt.wantErr == ErrActivityCodeInvalid {
				redeemCode = "act-unknown"
			}
			_, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: redeemCode, UserId: user.Id})
			require.ErrorIs(t, err, tt.wantErr)

			var grantCount int64
			require.NoError(t, DB.Model(&ActivityGrant{}).Where("user_id = ? AND code_id = ?", user.Id, created.Id).Count(&grantCount).Error)
			assert.Equal(t, beforeCount, grantCount, "被拒绝的兑换不产生新授权")
		})
	}
}

// 同一个活动码并发兑换时只能有一次成功（used_count 的 CAS 是无条件的并发闸门）。
func TestRedeemActivityCodeConcurrentSingleSuccess(t *testing.T) {
	code := ActivityCode{Code: "act-concurrent", TotalLimit: 1, PerUserLimit: 1}
	user, created := setupActivityFixture(t, code, 0)

	const goroutines = 5
	successes := make([]bool, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			if _, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-concurrent", UserId: user.Id}); err == nil {
				successes[idx] = true
			}
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, ok := range successes {
		if ok {
			successCount++
		}
	}
	assert.Equal(t, 1, successCount, "同一个活动码只允许一次并发兑换成功")

	var grantCount int64
	require.NoError(t, DB.Model(&ActivityGrant{}).Where("user_id = ? AND code_id = ?", user.Id, created.Id).Count(&grantCount).Error)
	assert.Equal(t, int64(1), grantCount)
	assert.Equal(t, 1, reloadActivityCode(t, created.Id).UsedCount)
}

// 同一活动码允许多次兑换时，新的授权在原到期时间上叠加时长（续期而非覆盖）。
func TestRedeemActivityCodeExtendsExpireTime(t *testing.T) {
	code := ActivityCode{Code: "act-renew", DurationSeconds: 3600, PerUserLimit: 2}
	user, _ := setupActivityFixture(t, code, 0)

	first, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-renew", UserId: user.Id})
	require.NoError(t, err)
	second, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-renew", UserId: user.Id})
	require.NoError(t, err)

	assert.Equal(t, first.ExpireTime+3600, second.ExpireTime, "第二次兑换在原到期时间上叠加时长")
}

func TestActivityGrantExpireAndRevoke(t *testing.T) {
	code := ActivityCode{Code: "act-lifecycle", DurationSeconds: 3600, PerUserLimit: 2}
	user, created := setupActivityFixture(t, code, 0)

	result, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-lifecycle", UserId: user.Id})
	require.NoError(t, err)

	// 直接改到期时间模拟过期：读取时必须立即被过滤（缓存不会延长授权）。
	require.NoError(t, DB.Model(&ActivityGrant{}).Where("id = ?", result.GrantId).
		Update("expire_time", common.GetTimestamp()-10).Error)
	require.NoError(t, InvalidateActivityGrantCache(user.Id))
	grants, err := GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	assert.Empty(t, grants, "已到期的授权立即失效")

	expired, err := ExpireDueActivityGrants()
	require.NoError(t, err)
	assert.Equal(t, int64(1), expired)
	reloaded := &ActivityGrant{}
	require.NoError(t, DB.First(reloaded, "id = ?", result.GrantId).Error)
	assert.Equal(t, ActivityGrantStatusExpired, reloaded.Status)

	// 新授权可被撤销，撤销后立即失效。
	second, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-lifecycle", UserId: user.Id})
	require.NoError(t, err)
	require.NoError(t, RevokeActivityGrant(second.GrantId, "测试撤销"))
	grants, err = GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	assert.Empty(t, grants)
	assert.Equal(t, 2, reloadActivityCode(t, created.Id).UsedCount, "撤销不返还兑换次数")
}

func TestActivityCodeFormatSupportsNonLatinScripts(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{name: "中文", code: "夏日免费", want: true},
		{name: "中文与数字分隔符混排", code: "hdm-免费_2026", want: true},
		{name: "日文与西里尔字母", code: "夏休み-скидка", want: true},
		{name: "空字符串", code: "", want: false},
		{name: "含空格", code: "夏日 免费", want: false},
		{name: "含表情符号", code: "免费🎉", want: false},
		{name: "含邮箱符号", code: "free@2026", want: false},
		{name: "64 个汉字", code: strings.Repeat("免", 64), want: true},
		{name: "65 个汉字", code: strings.Repeat("免", 65), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidActivityCode(tt.code))
		})
	}
}

func TestRedeemActivityCodeNormalizesUserInput(t *testing.T) {
	tests := []struct {
		name       string
		storedCode string
		input      string
	}{
		{name: "中文码带首尾空格", storedCode: "夏日免费", input: "  夏日免费  "},
		{name: "拉丁字母大小写与空格归一化", storedCode: "hdm-免费", input: " HDM-免费 "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, created := setupActivityFixture(t, ActivityCode{Code: tt.storedCode, DurationSeconds: 3600}, 0)

			result, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: tt.input, UserId: user.Id})
			require.NoError(t, err)
			assert.Equal(t, created.Id, result.CodeId)
			assert.Equal(t, tt.storedCode, result.Code)
		})
	}
}

// 不存在的活动码与空活动码必须返回同一种错误，避免通过错误类型区分活动码是否存在。
func TestRedeemActivityCodeInvalidCodeDoesNotLeakExistence(t *testing.T) {
	user, _ := setupActivityFixture(t, ActivityCode{Code: "act-exists"}, 0)

	_, errMissing := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-missing", UserId: user.Id})
	_, errEmpty := RedeemActivityCode(RedeemActivityCodeRequest{Code: "   ", UserId: user.Id})
	require.ErrorIs(t, errMissing, ErrActivityCodeInvalid)
	require.ErrorIs(t, errEmpty, ErrActivityCodeInvalid)
}

// 「指定模型」类型：兑换时写入授权；之后编辑活动码会实时同步到有效授权，
// 但已用 Token、到期时间与已支付价格保持不变。
func TestUpdateActivityCodeSyncsIssuedModelRatioOverrides(t *testing.T) {
	code := ActivityCode{
		Code:                "act-models",
		Name:                "指定模型活动",
		DurationSeconds:     24 * 3600,
		RatioScope:          ActivityRatioScopeModels,
		ModelRatioOverrides: ActivityModelRatioOverrides{"gpt-5": 0, "o3": 0.5},
	}
	user, created := setupActivityFixture(t, code, 0)

	result, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-models", UserId: user.Id})
	require.NoError(t, err)

	grant := &ActivityGrant{}
	require.NoError(t, DB.First(grant, "id = ?", result.GrantId).Error)
	assert.Equal(t, ActivityRatioScopeModels, grant.RatioScope)
	assert.Equal(t, map[string]float64{"gpt-5": 0, "o3": 0.5}, map[string]float64(grant.ModelRatioOverrides))
	originalExpireTime := grant.ExpireTime

	created.ModelRatioOverrides = ActivityModelRatioOverrides{"gpt-5": 0.9}
	require.NoError(t, created.Update())

	reloaded := &ActivityGrant{}
	require.NoError(t, DB.First(reloaded, "id = ?", result.GrantId).Error)
	assert.Equal(t, 0.9, reloaded.ModelRatioOverrides["gpt-5"], "编辑活动码后有效授权实时同步")
	assert.NotContains(t, reloaded.ModelRatioOverrides, "o3", "被移除的模型倍率同步删除")
	assert.Equal(t, originalExpireTime, reloaded.ExpireTime, "到期时间不因编辑活动码而变化")
	assert.Equal(t, 0, reloaded.PriceQuota, "已支付价格不因编辑活动码而变化")
}

func TestActivityCodeUpdatePersistsRatioScopeAndModelRatios(t *testing.T) {
	code := ActivityCode{
		Code:                "act-scope-update",
		RatioScope:          ActivityRatioScopeModels,
		ModelRatioOverrides: ActivityModelRatioOverrides{"gpt-5": 0.25},
	}
	_, created := setupActivityFixture(t, code, 0)

	created.RatioScope = ActivityRatioScopeGroup
	created.RatioMode = ActivityRatioModeOverride
	created.RatioOverride = 0.5
	created.ModelRatioOverrides = nil
	require.NoError(t, created.Update())

	stored := reloadActivityCode(t, created.Id)
	assert.Equal(t, ActivityRatioScopeGroup, stored.RatioScope)
	assert.Equal(t, 0.5, stored.RatioOverride)
	assert.Empty(t, stored.ModelRatioOverrides, "切回整个分组后模型倍率被清空")

	created.RatioScope = ActivityRatioScopeModels
	created.ModelRatioOverrides = ActivityModelRatioOverrides{"o3": 0}
	require.NoError(t, created.Update())

	stored = reloadActivityCode(t, created.Id)
	assert.Equal(t, ActivityRatioScopeModels, stored.RatioScope)
	assert.Equal(t, map[string]float64{"o3": 0}, map[string]float64(stored.ModelRatioOverrides))
}

// 新增列在存量行上是 NULL，Scan 必须把 NULL/空值当成空表，而不是报错。
func TestActivityModelRatioOverridesScanAndValue(t *testing.T) {
	t.Run("空表写 NULL", func(t *testing.T) {
		var empty ActivityModelRatioOverrides
		value, err := empty.Value()
		require.NoError(t, err)
		assert.Nil(t, value)

		value, err = ActivityModelRatioOverrides{}.Value()
		require.NoError(t, err)
		assert.Nil(t, value)
	})

	t.Run("非空表写 JSON 对象", func(t *testing.T) {
		value, err := ActivityModelRatioOverrides{"gpt-5": 0, "o3": 0.5}.Value()
		require.NoError(t, err)
		raw, ok := value.([]byte)
		require.True(t, ok)
		assert.JSONEq(t, `{"gpt-5":0,"o3":0.5}`, string(raw))
	})

	t.Run("NULL 与空值读回空表", func(t *testing.T) {
		for _, stored := range []interface{}{nil, []byte(nil), "", []byte(""), []byte("{}"), []byte("null")} {
			overrides := ActivityModelRatioOverrides{"stale": 1}
			require.NoError(t, overrides.Scan(stored))
			assert.Empty(t, overrides, "存量空值必须读成空表: %v", stored)
		}
	})

	t.Run("字符串与字节读回映射", func(t *testing.T) {
		fromString := ActivityModelRatioOverrides{}
		require.NoError(t, fromString.Scan(`{"gpt-5":0,"o3":0.5}`))
		assert.Equal(t, ActivityModelRatioOverrides{"gpt-5": 0, "o3": 0.5}, fromString)

		fromBytes := ActivityModelRatioOverrides{}
		require.NoError(t, fromBytes.Scan([]byte(`{"gpt-5":0.5}`)))
		assert.Equal(t, 0.5, fromBytes["gpt-5"])
	})

	t.Run("非法 JSON 报错", func(t *testing.T) {
		overrides := ActivityModelRatioOverrides{}
		require.Error(t, overrides.Scan([]byte("{not json")))
	})
}

func TestGrantActivityCodeByAdminDoesNotChargeOrConsumeQuota(t *testing.T) {
	code := ActivityCode{Code: "hcmgift", Name: "赠送活动", DurationSeconds: 24 * 3600, PriceQuota: 500, TotalLimit: 1, PerUserLimit: 1}
	user, created := setupActivityFixture(t, code, 1000)

	result, err := GrantActivityCode(GrantActivityCodeRequest{CodeId: created.Id, UserId: user.Id, Remark: "运营赠送"})
	require.NoError(t, err)
	assert.Equal(t, activityTestGroup, result.GroupName)
	require.Greater(t, result.ExpireTime, common.GetTimestamp())

	assert.Equal(t, 1000, reloadUserQuota(t, user.Id), "赠送不扣余额")
	assert.Equal(t, 0, reloadActivityCode(t, created.Id).UsedCount, "赠送不占兑换次数")

	grant := &ActivityGrant{}
	require.NoError(t, DB.First(grant, "id = ?", result.GrantId).Error)
	assert.Equal(t, ActivityGrantSourceAdmin, grant.Source)
	assert.Equal(t, 0, grant.PriceQuota)
	assert.Equal(t, "运营赠送", grant.Remark)
	assert.Equal(t, ActivityGrantStatusActive, grant.Status)

	// 已占满每人上限与总量上限的活动码仍可继续赠送，说明管理员赠送不受兑换名额约束。
	_, err = GrantActivityCode(GrantActivityCodeRequest{CodeId: created.Id, UserId: user.Id})
	require.NoError(t, err)
	assert.Equal(t, 0, reloadActivityCode(t, created.Id).UsedCount)
	require.Greater(t, result.ExpireTime, int64(0))
}

func TestGrantActivityCodeExtendsExistingGrantAndRejectsDisabledCode(t *testing.T) {
	code := ActivityCode{Code: "hcmgift2", Name: "赠送活动", DurationSeconds: 3600}
	user, created := setupActivityFixture(t, code, 0)

	first, err := GrantActivityCode(GrantActivityCodeRequest{CodeId: created.Id, UserId: user.Id})
	require.NoError(t, err)
	second, err := GrantActivityCode(GrantActivityCodeRequest{CodeId: created.Id, UserId: user.Id})
	require.NoError(t, err)
	// 第二次赠送在原授权到期时间上再叠加 1 小时；两次计算之间最多相差几秒。
	assert.GreaterOrEqual(t, second.ExpireTime, first.ExpireTime+3600, "重复赠送按续期叠加")
	assert.Less(t, second.ExpireTime, first.ExpireTime+3600+5)

	require.NoError(t, UpdateActivityCodeStatus(created.Id, ActivityCodeStatusDisabled))
	_, err = GrantActivityCode(GrantActivityCodeRequest{CodeId: created.Id, UserId: user.Id})
	require.ErrorIs(t, err, ErrActivityCodeDisabled)

	var grantCount int64
	require.NoError(t, DB.Model(&ActivityGrant{}).Where("user_id = ?", user.Id).Count(&grantCount).Error)
	assert.Equal(t, int64(2), grantCount, "停用后不再新增授权")
}

func TestRedeemBenefitActivityCreatesGrantPerBenefit(t *testing.T) {
	code := ActivityCode{
		Code:            "act-benefit",
		Name:            "多分组权益",
		Type:            ActivityCodeTypeBenefit,
		DurationSeconds: 3600,
		PerUserLimit:    1,
		Benefits: ActivityBenefits{
			{Key: "key-a", GroupName: "benefit-a", Scope: ActivityBenefitScopeGroup, RpmLimit: 10, TokenTotal: 1000},
			{Key: "key-b", GroupName: "benefit-b", Scope: ActivityBenefitScopeModel, ModelName: "gpt-5", RpmLimit: 20},
		},
	}
	user, created := setupActivityFixture(t, code, 0)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"activity-test-group":1,"benefit-a":1,"benefit-b":1}`))

	result, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-benefit", UserId: user.Id})
	require.NoError(t, err)
	assert.Equal(t, []string{"benefit-a", "benefit-b"}, result.GroupNames)
	assert.Equal(t, "benefit-a", result.GroupName)
	assert.Equal(t, 0, result.PriceQuota)

	grants := make([]ActivityGrant, 0)
	require.NoError(t, DB.Where("code_id = ?", created.Id).Order("id asc").Find(&grants).Error)
	require.Len(t, grants, 2)
	assert.NotEmpty(t, grants[0].RedeemBatch)
	assert.Equal(t, grants[0].RedeemBatch, grants[1].RedeemBatch, "同一次兑换的多条权益共享批次")
	assert.Equal(t, grants[0].ExpireTime, grants[1].ExpireTime)
	assert.Equal(t, "key-a", grants[0].BenefitKey)
	assert.Equal(t, 1000, int(grants[0].TokenTotal))
	assert.Equal(t, 10, grants[0].RpmLimit)
	assert.Equal(t, ActivityRatioModeOverride, grants[0].RatioMode)
	assert.Equal(t, float64(0), grants[0].RatioOverride)
	assert.Equal(t, "key-b", grants[1].BenefitKey)
	assert.Equal(t, ActivityRatioScopeModels, grants[1].RatioScope)
	assert.Equal(t, map[string]float64{"gpt-5": 0}, map[string]float64(grants[1].ModelRatioOverrides))
	assert.Equal(t, 0, int(grants[1].TokenTotal))
	assert.Equal(t, 1, reloadActivityCode(t, created.Id).UsedCount)

	// 一次兑换生成两条授权，但每人上限只应算一次。
	_, err = RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-benefit", UserId: user.Id})
	require.ErrorIs(t, err, ErrActivityCodeUserLimitReached)
}

func findGrantByBenefitKey(t *testing.T, codeId int, key string) *ActivityGrant {
	t.Helper()
	grant := &ActivityGrant{}
	require.NoError(t, DB.First(grant, "code_id = ? AND benefit_key = ?", codeId, key).Error)
	return grant
}

func TestUpdateBenefitActivitySyncsIssuedGrants(t *testing.T) {
	code := ActivityCode{
		Code:            "act-benefit-sync",
		Name:            "同步权益",
		Type:            ActivityCodeTypeBenefit,
		DurationSeconds: 3600,
		PerUserLimit:    1,
		Benefits: ActivityBenefits{
			{Key: "key-a", GroupName: "benefit-a", Scope: ActivityBenefitScopeGroup, TokenTotal: 1000},
			{Key: "key-b", GroupName: "benefit-b", Scope: ActivityBenefitScopeModel, ModelName: "gpt-5"},
		},
	}
	user, created := setupActivityFixture(t, code, 0)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"activity-test-group":1,"benefit-a":1,"benefit-b":1,"benefit-c":1}`))
	_, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-benefit-sync", UserId: user.Id})
	require.NoError(t, err)

	grantA := findGrantByBenefitKey(t, created.Id, "key-a")
	grantB := findGrantByBenefitKey(t, created.Id, "key-b")
	require.NoError(t, AddActivityGrantTokenUsage(grantA.Id, 123))
	grantA = findGrantByBenefitKey(t, created.Id, "key-a")
	assert.Equal(t, int64(123), grantA.TokenUsed)

	created.Name = "同步权益 v2"
	created.Benefits = ActivityBenefits{
		{Key: "key-a", GroupName: "benefit-a", Scope: ActivityBenefitScopeGroup, RpmLimit: 60, TokenTotal: 500},
		{Key: "key-c", GroupName: "benefit-c", Scope: ActivityBenefitScopeGroup},
	}
	require.NoError(t, created.Update())

	grantA = findGrantByBenefitKey(t, created.Id, "key-a")
	assert.Equal(t, int64(500), grantA.TokenTotal, "Token 总量同步")
	assert.Equal(t, int64(123), grantA.TokenUsed, "已用 Token 不因编辑而清零")
	assert.Equal(t, 60, grantA.RpmLimit)
	assert.Equal(t, "同步权益 v2", grantA.CodeName)
	assert.Equal(t, ActivityGrantStatusActive, grantA.Status)

	grantB = findGrantByBenefitKey(t, created.Id, "key-b")
	assert.Equal(t, ActivityGrantStatusRevoked, grantB.Status, "被删除的权益授权撤销")

	grantC := findGrantByBenefitKey(t, created.Id, "key-c")
	assert.Equal(t, ActivityGrantStatusActive, grantC.Status, "新增权益为有效批次补建授权")
	assert.Equal(t, grantA.RedeemBatch, grantC.RedeemBatch)
	assert.Equal(t, grantA.ExpireTime, grantC.ExpireTime, "补建授权沿用批次到期时间")
	assert.Equal(t, grantA.StartTime, grantC.StartTime)
	assert.Equal(t, int64(0), grantC.TokenUsed)

	// 管理员手动撤销的权益，之后再次编辑活动码不应被补回来。
	require.NoError(t, RevokeActivityGrant(grantC.Id, "手动撤销"))
	require.NoError(t, created.Update())
	grantC = findGrantByBenefitKey(t, created.Id, "key-c")
	assert.Equal(t, ActivityGrantStatusRevoked, grantC.Status)
	var grantCount int64
	require.NoError(t, DB.Model(&ActivityGrant{}).Where("code_id = ?", created.Id).Count(&grantCount).Error)
	assert.Equal(t, int64(3), grantCount, "不应重复补建已手动撤销的权益")
}

func TestAddActivityGrantTokenUsage(t *testing.T) {
	code := ActivityCode{Code: "act-token-usage", Name: "Token 用量", DurationSeconds: 3600}
	user, created := setupActivityFixture(t, code, 0)
	packageGrant := ActivityGrant{
		UserId: user.Id, CodeId: created.Id, Code: created.Code, GroupName: activityTestGroup,
		Status: ActivityGrantStatusActive, TokenTotal: 100, TokenUsed: 0,
	}
	require.NoError(t, DB.Create(&packageGrant).Error)
	require.NoError(t, AddActivityGrantTokenUsage(packageGrant.Id, 30))
	require.NoError(t, AddActivityGrantTokenUsage(packageGrant.Id, 0))
	require.NoError(t, AddActivityGrantTokenUsage(packageGrant.Id, -5))
	reloaded := &ActivityGrant{}
	require.NoError(t, DB.First(reloaded, "id = ?", packageGrant.Id).Error)
	assert.Equal(t, int64(30), reloaded.TokenUsed)

	nonPackageGrant := ActivityGrant{
		UserId: user.Id, CodeId: created.Id, Code: created.Code, GroupName: activityTestGroup,
		Status: ActivityGrantStatusActive,
	}
	require.NoError(t, DB.Create(&nonPackageGrant).Error)
	require.NoError(t, AddActivityGrantTokenUsage(nonPackageGrant.Id, 10))
	reloaded = &ActivityGrant{}
	require.NoError(t, DB.First(reloaded, "id = ?", nonPackageGrant.Id).Error)
	assert.Equal(t, int64(0), reloaded.TokenUsed, "非 Token 套餐授权不累加用量")
}

func TestCountActivityRedemptionsGroupsByBatch(t *testing.T) {
	grants := []ActivityGrant{
		{Id: 1, RedeemBatch: "batch-1"},
		{Id: 2, RedeemBatch: "batch-1"},
		{Id: 3, RedeemBatch: "batch-2"},
		{Id: 4},
		{Id: 5},
	}
	assert.Equal(t, 4, CountActivityRedemptions(grants))
	assert.Equal(t, 1, CountActivityRedemptions(grants[:2]))
	assert.Equal(t, 0, CountActivityRedemptions(nil))
}

func TestActivityBenefitsScanAndValue(t *testing.T) {
	var empty ActivityBenefits
	value, err := empty.Value()
	require.NoError(t, err)
	assert.Nil(t, value)

	value, err = ActivityBenefits{{Key: "a", GroupName: "vip", Scope: ActivityBenefitScopeGroup, TokenTotal: 100}}.Value()
	require.NoError(t, err)
	raw, ok := value.([]byte)
	require.True(t, ok)
	assert.JSONEq(t, `[{"key":"a","group_name":"vip","scope":"group","limit_mode":"","rpm_limit":0,"concurrency_limit":0,"token_total":100}]`, string(raw))

	for _, stored := range []interface{}{nil, []byte(nil), "", []byte(""), []byte("null")} {
		benefits := ActivityBenefits{{Key: "stale"}}
		require.NoError(t, benefits.Scan(stored))
		assert.Empty(t, benefits, "存量空值必须读成空列表: %v", stored)
	}
	fromString := ActivityBenefits{}
	require.NoError(t, fromString.Scan(`[{"key":"a","group_name":"vip","scope":"group"}]`))
	require.Len(t, fromString, 1)
	assert.Equal(t, "a", fromString[0].Key)

	invalid := ActivityBenefits{}
	require.Error(t, invalid.Scan([]byte("{not json")))
}

// 活动到期时间在发放时被快照到授权：兑换与管理员赠送都必须写入。
// 兑换后的授权有效期（expire_time）与活动到期时间互相独立。
func TestActivityCodeIssueSnapshotsActivityEndTime(t *testing.T) {
	now := common.GetTimestamp()
	tests := []struct {
		name  string
		issue func(t *testing.T, user *User, code *ActivityCode) (grantId int, expireTime int64)
	}{
		{
			name: "redeem",
			issue: func(t *testing.T, user *User, code *ActivityCode) (int, int64) {
				result, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: code.Code, UserId: user.Id})
				require.NoError(t, err)
				return result.GrantId, result.ExpireTime
			},
		},
		{
			name: "admin grant",
			issue: func(t *testing.T, user *User, code *ActivityCode) (int, int64) {
				result, err := GrantActivityCode(GrantActivityCodeRequest{CodeId: code.Id, UserId: user.Id})
				require.NoError(t, err)
				return result.GrantId, result.ExpireTime
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := ActivityCode{
				Code:            "hdmactive",
				Name:            "活动到期快照",
				DurationSeconds: 7 * 24 * 3600,
				ActiveEndTime:   now + 3600,
			}
			user, created := setupActivityFixture(t, code, 0)

			grantId, expireTime := tt.issue(t, user, created)
			require.Greater(t, expireTime, created.ActiveEndTime, "授权有效期与活动到期时间互相独立")

			grant := &ActivityGrant{}
			require.NoError(t, DB.First(grant, "id = ?", grantId).Error)
			assert.Equal(t, created.ActiveEndTime, grant.ActivityEndTime, "发放时快照活动到期时间")
			assert.Equal(t, created.ActiveEndTime, grant.EffectiveExpire(), "生效到期时间被活动到期时间 cap")
		})
	}
}

// 活动到期后授权立即失效：即使 expire_time 还是永久（0），也不再出现在有效授权里。
func TestActivityGrantDisappearsWhenActivityEnds(t *testing.T) {
	now := common.GetTimestamp()
	code := ActivityCode{Code: "hdmendgrant", Name: "活动到期失效", ActiveEndTime: now + 3600}
	user, _ := setupActivityFixture(t, code, 0)

	result, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "hdmendgrant", UserId: user.Id})
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.ExpireTime, "未配置时长时授权永久有效")

	grants, err := GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	require.Len(t, grants, 1, "活动未到期时授权有效")

	require.NoError(t, DB.Model(&ActivityGrant{}).Where("id = ?", result.GrantId).
		Update("activity_end_time", now-1).Error)
	grants, err = GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	assert.Empty(t, grants, "活动到期后不再返回该授权")
}

// 活动到期后管理员赠送同样被拒绝：任何发放路径都不能产生瞬间失效的授权。
func TestGrantActivityCodeRejectsEndedActivity(t *testing.T) {
	now := common.GetTimestamp()
	code := ActivityCode{Code: "hdmgrantended", Name: "已结束赠送", DurationSeconds: 3600, ActiveEndTime: now - 1}
	user, created := setupActivityFixture(t, code, 100)

	_, err := GrantActivityCode(GrantActivityCodeRequest{CodeId: created.Id, UserId: user.Id})
	require.ErrorIs(t, err, ErrActivityCodeActivityEnded)

	var count int64
	require.NoError(t, DB.Model(&ActivityGrant{}).Count(&count).Error)
	assert.Zero(t, count, "拒绝赠送时不写授权")
}

// 活动到期后拒绝兑换，并且不写授权、不扣余额。
func TestRedeemActivityCodeRejectsEndedActivity(t *testing.T) {
	now := common.GetTimestamp()
	code := ActivityCode{
		Code:            "hdmended",
		Name:            "已结束活动",
		DurationSeconds: 3600,
		PriceQuota:      100,
		ActiveEndTime:   now - 1,
	}
	user, created := setupActivityFixture(t, code, 1000)

	_, err := RedeemActivityCode(RedeemActivityCodeRequest{
		Code:               "hdmended",
		UserId:             user.Id,
		Confirm:            true,
		ExpectedPriceQuota: 100,
	})
	require.ErrorIs(t, err, ErrActivityCodeActivityEnded, "活动到期与兑换截止是两个独立原因")

	var count int64
	require.NoError(t, DB.Model(&ActivityGrant{}).Count(&count).Error)
	assert.Zero(t, count, "拒绝兑换时不写授权")
	assert.Equal(t, 1000, reloadUserQuota(t, user.Id), "拒绝兑换时不扣余额")
	assert.Equal(t, 0, reloadActivityCode(t, created.Id).UsedCount, "拒绝兑换时不占名额")
}

// 限流模式归一化：存量缺模式按 rpm，合法值标准化，非法值返回空串由调用方拒绝。
func TestNormalizeActivityLimitMode(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"空值按 rpm", "", ActivityLimitModeRPM},
		{"纯空白按 rpm", "   ", ActivityLimitModeRPM},
		{"合法 rpm 小写", "rpm", ActivityLimitModeRPM},
		{"合法 rpm 大小写混合", "RPM", ActivityLimitModeRPM},
		{"合法 rpm 带空白", " rpm ", ActivityLimitModeRPM},
		{"合法 concurrency", "concurrency", ActivityLimitModeConcurrency},
		{"合法 concurrency 大写", "Concurrency", ActivityLimitModeConcurrency},
		{"未知模式返回空串", "qps", ""},
		{"相似值返回空串", "rpm-per-minute", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeActivityLimitMode(tt.value))
		})
	}
}

// 旧缓存/旧落库 JSON 没有新字段：反序列化后按 rpm 处理，读取归一化兼容存量数据。
func TestActivityLimitModeLegacyJSONDefaultsToRpm(t *testing.T) {
	var legacyCode ActivityCode
	require.NoError(t, common.Unmarshal([]byte(`{"code":"legacy","group_name":"vip","rpm_limit":10}`), &legacyCode))
	assert.Equal(t, "", legacyCode.LimitMode)
	assert.Equal(t, 0, legacyCode.ConcurrencyLimit)
	specs := legacyCode.grantSpecs()
	require.Len(t, specs, 1)
	assert.Equal(t, ActivityLimitModeRPM, specs[0].LimitMode, "存量活动码缺模式按 rpm 发放")
	assert.Equal(t, 10, specs[0].RpmLimit)

	var legacyGrant ActivityGrant
	require.NoError(t, common.Unmarshal([]byte(`{"id":1,"rpm_limit":10}`), &legacyGrant))
	assert.Equal(t, "", legacyGrant.LimitMode)
	assert.Equal(t, ActivityLimitModeRPM, NormalizeActivityLimitMode(legacyGrant.LimitMode), "存量授权缺模式按 rpm 处理")
	assert.Equal(t, 0, legacyGrant.ConcurrencyLimit)

	legacyBenefits := ActivityBenefits{}
	require.NoError(t, legacyBenefits.Scan([]byte(`[{"key":"a","group_name":"vip","scope":"group","rpm_limit":10}]`)))
	require.Len(t, legacyBenefits, 1)
	assert.Equal(t, "", legacyBenefits[0].LimitMode)
	assert.Equal(t, 0, legacyBenefits[0].ConcurrencyLimit)
	spec := activityGrantSpecForBenefit(legacyBenefits[0])
	assert.Equal(t, ActivityLimitModeRPM, spec.LimitMode, "存量权益缺模式按 rpm 发放")
	assert.Equal(t, 10, spec.RpmLimit)
}

// 双类型限流落库：concurrency 模式写入并发上限，兑换与管理员赠送两条发放路径都要贯通，
// 编辑活动码后有效授权实时同步。
func TestActivityCodeConcurrencyPersistsAndSyncsToGrants(t *testing.T) {
	code := ActivityCode{
		Code:             "act-concurrency",
		Name:             "并发活动",
		DurationSeconds:  3600,
		LimitMode:        ActivityLimitModeConcurrency,
		ConcurrencyLimit: 5,
	}
	user, created := setupActivityFixture(t, code, 0)

	result, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-concurrency", UserId: user.Id})
	require.NoError(t, err)
	grant := &ActivityGrant{}
	require.NoError(t, DB.First(grant, "id = ?", result.GrantId).Error)
	assert.Equal(t, ActivityLimitModeConcurrency, grant.LimitMode, "兑换授权写入并发模式")
	assert.Equal(t, 5, grant.ConcurrencyLimit)

	gift, err := GrantActivityCode(GrantActivityCodeRequest{CodeId: created.Id, UserId: user.Id})
	require.NoError(t, err)
	giftGrant := &ActivityGrant{}
	require.NoError(t, DB.First(giftGrant, "id = ?", gift.GrantId).Error)
	assert.Equal(t, ActivityLimitModeConcurrency, giftGrant.LimitMode, "管理员赠送授权写入并发模式")
	assert.Equal(t, 5, giftGrant.ConcurrencyLimit)

	created.ConcurrencyLimit = 9
	require.NoError(t, created.Update())
	reloaded := &ActivityGrant{}
	require.NoError(t, DB.First(reloaded, "id = ?", result.GrantId).Error)
	assert.Equal(t, ActivityLimitModeConcurrency, reloaded.LimitMode)
	assert.Equal(t, 9, reloaded.ConcurrencyLimit, "编辑活动码后有效授权实时同步")

	// 缓存键升级到 v2：同结构数据经缓存读回后模式与并发值不丢。
	require.NoError(t, DB.Model(&ActivityGrant{}).Where("id = ?", result.GrantId).Update("limit_mode", ActivityLimitModeConcurrency).Error)
	cached, err := GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	require.NotEmpty(t, cached)
	assert.Equal(t, ActivityLimitModeConcurrency, cached[0].LimitMode)
	assert.Equal(t, 9, cached[0].ConcurrencyLimit)
}

// benefit 条目各自携带限流模式：兑换发放按条目落库，编辑活动码按 key 同步。
func TestBenefitLimitModePersistsAndSyncsByKey(t *testing.T) {
	code := ActivityCode{
		Code:            "act-benefit-limits",
		Name:            "权益限流",
		Type:            ActivityCodeTypeBenefit,
		DurationSeconds: 3600,
		PerUserLimit:    1,
		Benefits: ActivityBenefits{
			{Key: "key-a", GroupName: "benefit-a", Scope: ActivityBenefitScopeGroup, LimitMode: ActivityLimitModeRPM, RpmLimit: 10},
			{Key: "key-b", GroupName: "benefit-b", Scope: ActivityBenefitScopeGroup, LimitMode: ActivityLimitModeConcurrency, ConcurrencyLimit: 3},
		},
	}
	user, created := setupActivityFixture(t, code, 0)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"activity-test-group":1,"benefit-a":1,"benefit-b":1}`))
	_, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-benefit-limits", UserId: user.Id})
	require.NoError(t, err)

	grantA := findGrantByBenefitKey(t, created.Id, "key-a")
	assert.Equal(t, ActivityLimitModeRPM, grantA.LimitMode)
	assert.Equal(t, 10, grantA.RpmLimit)
	assert.Equal(t, 0, grantA.ConcurrencyLimit, "RPM 条目不写并发")
	grantB := findGrantByBenefitKey(t, created.Id, "key-b")
	assert.Equal(t, ActivityLimitModeConcurrency, grantB.LimitMode)
	assert.Equal(t, 3, grantB.ConcurrencyLimit)
	assert.Equal(t, 0, grantB.RpmLimit, "并发条目不写 RPM")

	created.Benefits = ActivityBenefits{
		{Key: "key-a", GroupName: "benefit-a", Scope: ActivityBenefitScopeGroup, LimitMode: ActivityLimitModeRPM, RpmLimit: 20},
		{Key: "key-b", GroupName: "benefit-b", Scope: ActivityBenefitScopeGroup, LimitMode: ActivityLimitModeConcurrency, ConcurrencyLimit: 8},
	}
	require.NoError(t, created.Update())

	grantA = findGrantByBenefitKey(t, created.Id, "key-a")
	assert.Equal(t, 20, grantA.RpmLimit)
	assert.Equal(t, 0, grantA.ConcurrencyLimit)
	grantB = findGrantByBenefitKey(t, created.Id, "key-b")
	assert.Equal(t, ActivityLimitModeConcurrency, grantB.LimitMode)
	assert.Equal(t, 8, grantB.ConcurrencyLimit, "按 key 同步并发上限")
}

// activityCodeLegacySchema/activityGrantLegacySchema 是加限流列之前的表结构快照，
// 用它先落库存量数据再跑 AutoMigrate，验证真实升级路径而不只是新建表。
type activityCodeLegacySchema struct {
	Id                  int    `gorm:"primaryKey"`
	Code                string `gorm:"type:varchar(64);uniqueIndex"`
	Name                string `gorm:"type:varchar(64);index"`
	Description         string `gorm:"type:text"`
	Status              int    `gorm:"default:1"`
	Type                string `gorm:"type:varchar(16);default:'ratio'"`
	GroupName           string `gorm:"type:varchar(64);index"`
	DurationSeconds     int64  `gorm:"bigint;default:0"`
	RpmLimit            int    `gorm:"default:0"`
	PriceQuota          int    `gorm:"default:0"`
	RatioMode           string `gorm:"type:varchar(16);default:'inherit'"`
	RatioOverride       float64
	RatioScope          string                      `gorm:"type:varchar(16);default:'group'"`
	ModelRatioOverrides ActivityModelRatioOverrides `gorm:"type:json"`
	Benefits            ActivityBenefits            `gorm:"type:json"`
	StartTime           int64                       `gorm:"bigint;default:0"`
	EndTime             int64                       `gorm:"bigint;default:0"`
	TotalLimit          int                         `gorm:"default:0"`
	UsedCount           int                         `gorm:"default:0"`
	PerUserLimit        int                         `gorm:"default:1"`
	PublicVisible       int                         `gorm:"default:0"`
	SortWeight          int                         `gorm:"default:0;index"`
	ImageUrl            string                      `gorm:"type:varchar(512);default:''"`
	ActiveEndTime       int64                       `gorm:"bigint;default:0"`
	CreatedBy           int
	CreatedTime         int64          `gorm:"bigint"`
	UpdatedTime         int64          `gorm:"bigint"`
	DeletedAt           gorm.DeletedAt `gorm:"index"`
}

type activityGrantLegacySchema struct {
	Id                  int    `gorm:"primaryKey"`
	UserId              int    `gorm:"index"`
	CodeId              int    `gorm:"index"`
	Code                string `gorm:"type:varchar(64)"`
	CodeName            string `gorm:"type:varchar(64)"`
	GroupName           string `gorm:"type:varchar(64);index"`
	RatioMode           string `gorm:"type:varchar(16);default:'inherit'"`
	RatioOverride       float64
	RatioScope          string                      `gorm:"type:varchar(16);default:'group'"`
	ModelRatioOverrides ActivityModelRatioOverrides `gorm:"type:json"`
	RpmLimit            int                         `gorm:"default:0"`
	PriceQuota          int                         `gorm:"default:0"`
	RedeemBatch         string                      `gorm:"type:varchar(64);index"`
	BenefitKey          string                      `gorm:"type:varchar(32);index"`
	TokenTotal          int64                       `gorm:"bigint;default:0"`
	TokenUsed           int64                       `gorm:"bigint;default:0"`
	StartTime           int64                       `gorm:"bigint"`
	ExpireTime          int64                       `gorm:"bigint;index"`
	ActivityEndTime     int64                       `gorm:"bigint;default:0"`
	Status              string                      `gorm:"type:varchar(32);index"`
	Source              string                      `gorm:"type:varchar(32);default:'redeem'"`
	Remark              string                      `gorm:"type:varchar(255);default:''"`
	CreatedTime         int64                       `gorm:"bigint"`
	UpdatedTime         int64                       `gorm:"bigint"`
}

// SQLite 旧 schema 升级：加列前落库的存量 RPM 行在 AutoMigrate 后保留原值、模式读取为 rpm，
// 新的并发列可以正常保存——真实升级路径，而不是新建表后的空库验证。
func TestActivityLimitModeSchemaUpgradeKeepsLegacyRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Table("activity_codes").AutoMigrate(&activityCodeLegacySchema{}))
	require.NoError(t, db.Table("activity_grants").AutoMigrate(&activityGrantLegacySchema{}))
	now := common.GetTimestamp()
	legacyCode := &activityCodeLegacySchema{
		Code: "legacy-rpm", Name: "存量活动", GroupName: activityTestGroup,
		DurationSeconds: 3600, RpmLimit: 42, Status: ActivityCodeStatusEnabled,
		RatioMode: ActivityRatioModeInherit, PerUserLimit: 1, CreatedTime: now, UpdatedTime: now,
	}
	require.NoError(t, db.Table("activity_codes").Create(legacyCode).Error)
	legacyGrant := &activityGrantLegacySchema{
		UserId: 5, CodeId: legacyCode.Id, Code: "legacy-rpm", GroupName: activityTestGroup,
		RatioMode: ActivityRatioModeInherit, RpmLimit: 12, Status: ActivityGrantStatusActive,
		Source: ActivityGrantSourceRedeem, StartTime: now, ExpireTime: now + 3600,
		CreatedTime: now, UpdatedTime: now,
	}
	require.NoError(t, db.Table("activity_grants").Create(legacyGrant).Error)

	// 与启动时相同的升级入口。
	require.NoError(t, db.AutoMigrate(&ActivityCode{}, &ActivityGrant{}))

	var code ActivityCode
	require.NoError(t, db.Table("activity_codes").First(&code, "id = ?", legacyCode.Id).Error)
	assert.Equal(t, 42, code.RpmLimit, "存量 RPM 值必须保留")
	assert.Equal(t, ActivityLimitModeRPM, code.EffectiveLimitMode(), "存量行模式读取为 rpm")
	assert.Equal(t, 0, code.ConcurrencyLimit)

	var grant ActivityGrant
	require.NoError(t, db.Table("activity_grants").First(&grant, "id = ?", legacyGrant.Id).Error)
	assert.Equal(t, 12, grant.RpmLimit, "存量授权 RPM 值必须保留")
	assert.Equal(t, ActivityLimitModeRPM, NormalizeActivityLimitMode(grant.LimitMode), "存量授权模式读取为 rpm")
	assert.Equal(t, 0, grant.ConcurrencyLimit)

	// 新并发列可保存：升级后的表能写入并读回新模式与并发上限。
	code.LimitMode = ActivityLimitModeConcurrency
	code.ConcurrencyLimit = 7
	require.NoError(t, db.Table("activity_codes").Save(&code).Error)
	var reloadedCode ActivityCode
	require.NoError(t, db.Table("activity_codes").First(&reloadedCode, "id = ?", legacyCode.Id).Error)
	assert.Equal(t, ActivityLimitModeConcurrency, reloadedCode.LimitMode)
	assert.Equal(t, 7, reloadedCode.ConcurrencyLimit)

	grant.LimitMode = ActivityLimitModeConcurrency
	grant.ConcurrencyLimit = 5
	require.NoError(t, db.Table("activity_grants").Save(&grant).Error)
	var reloadedGrant ActivityGrant
	require.NoError(t, db.Table("activity_grants").First(&reloadedGrant, "id = ?", legacyGrant.Id).Error)
	assert.Equal(t, ActivityLimitModeConcurrency, reloadedGrant.LimitMode)
	assert.Equal(t, 5, reloadedGrant.ConcurrencyLimit)
}

// 授权缓存 v2：限流新字段随缓存读写不丢；旧 v1 缓存不能让并发授权按 0 并发放行；
// 编辑活动码与撤销授权都必须失效缓存。
func TestActivityGrantCacheV2KeepsLimitFieldsAndInvalidates(t *testing.T) {
	code := ActivityCode{
		Code: "act-cache-v2", Name: "缓存并发活动", DurationSeconds: 3600,
		LimitMode: ActivityLimitModeConcurrency, ConcurrencyLimit: 7,
	}
	user, created := setupActivityFixture(t, code, 0)
	result, err := RedeemActivityCode(RedeemActivityCodeRequest{Code: "act-cache-v2", UserId: user.Id})
	require.NoError(t, err)

	server := useUserCacheMiniRedis(t)

	// 旧 v1 缓存负载：旧结构没有新字段，却声称有一条永久有效的授权；
	// 若代码仍读 v1 键，这条授权会以 0 并发放行。
	legacyKey := "activity:grants:" + strconv.Itoa(user.Id)
	legacyPayload := `[{"id":99999,"user_id":` + strconv.Itoa(user.Id) + `,"code":"act-cache-v2","group_name":"activity-test-group","ratio_mode":"override","ratio_override":0,"ratio_scope":"group","rpm_limit":0,"status":"active","source":"redeem","start_time":1,"expire_time":0,"created_time":1,"updated_time":1}]`
	require.NoError(t, common.RedisSet(legacyKey, legacyPayload, time.Minute))
	require.True(t, server.Exists(legacyKey))

	grants, err := GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.NotEqual(t, 99999, grants[0].Id, "旧 v1 缓存必须被忽略")
	assert.Equal(t, result.GrantId, grants[0].Id)
	assert.Equal(t, ActivityLimitModeConcurrency, grants[0].LimitMode)
	assert.Equal(t, 7, grants[0].ConcurrencyLimit, "并发授权不能按旧缓存的 0 值放行")

	// 第二次读取命中 v2 缓存，新字段不丢。
	cachedRaw, err := common.RedisGet(activityGrantCacheKey(user.Id))
	require.NoError(t, err)
	require.Contains(t, cachedRaw, `"concurrency_limit":7`)
	require.Contains(t, cachedRaw, `"limit_mode":"concurrency"`)
	cached, err := GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	require.Len(t, cached, 1)
	assert.Equal(t, ActivityLimitModeConcurrency, cached[0].LimitMode)
	assert.Equal(t, 7, cached[0].ConcurrencyLimit, "缓存命中后并发值不丢")

	// 编辑活动码失效缓存，读到同步后的新值。
	created.ConcurrencyLimit = 9
	require.NoError(t, created.Update())
	_, err = common.RedisGet(activityGrantCacheKey(user.Id))
	assert.Error(t, err, "编辑活动码后 v2 缓存必须失效")
	updated, err := GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	require.Len(t, updated, 1)
	assert.Equal(t, 9, updated[0].ConcurrencyLimit, "编辑后读到同步的新值")

	// 撤销授权同样失效缓存，下一次读取不再返回。
	require.NoError(t, RevokeActivityGrant(result.GrantId, "测试撤销"))
	_, err = common.RedisGet(activityGrantCacheKey(user.Id))
	assert.Error(t, err, "撤销授权后 v2 缓存必须失效")
	remaining, err := GetActiveActivityGrants(user.Id)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

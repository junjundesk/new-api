package model

import (
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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

// 「指定模型」类型：兑换时把模型倍率快照进授权，之后改活动码不影响已发放的授权。
func TestRedeemActivityCodeSnapshotsModelRatioOverrides(t *testing.T) {
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

	created.ModelRatioOverrides = ActivityModelRatioOverrides{"gpt-5": 0.9}
	require.NoError(t, created.Update())

	reloaded := &ActivityGrant{}
	require.NoError(t, DB.First(reloaded, "id = ?", result.GrantId).Error)
	assert.Equal(t, float64(0), reloaded.ModelRatioOverrides["gpt-5"], "已发放的授权是快照，不随后续修改变化")
	assert.Equal(t, 0.5, reloaded.ModelRatioOverrides["o3"])
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

package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildSelfUserDataEnterpriseUnlockLockIn covers the persistent enterprise
// mark: meeting the configured threshold once writes the mark into the user's
// setting, so later threshold increases must not revoke the status.
func TestBuildSelfUserDataEnterpriseUnlockLockIn(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.UserRechargeRecord{}, &model.UserBonusGrant{}))

	setting := operation_setting.GetEnterpriseSetting()
	prevEnabled, prevThreshold := setting.Enabled, setting.TotalRechargeThreshold
	setting.Enabled = true
	setting.TotalRechargeThreshold = 100
	t.Cleanup(func() {
		setting.Enabled, setting.TotalRechargeThreshold = prevEnabled, prevThreshold
	})

	user := &model.User{
		Username: "enterprise-user",
		Password: "123456",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	// Below threshold: not unlocked and nothing persisted.
	data := buildSelfUserData(user)
	assert.False(t, data["enterprise_unlocked"].(bool))
	fresh, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.False(t, fresh.GetSetting().EnterpriseUnlocked)

	// Recharge well past the 100 threshold: unlocked, mark persisted.
	require.NoError(t, model.RecordAdminQuotaRecharge(user.Id, int(1000*common.QuotaPerUnit)))
	data = buildSelfUserData(user)
	assert.True(t, data["enterprise_unlocked"].(bool))
	fresh, err = model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.True(t, fresh.GetSetting().EnterpriseUnlocked)

	// Raising the threshold far above the recharge must not revoke the status.
	setting.TotalRechargeThreshold = 100000
	fresh, err = model.GetUserById(user.Id, false)
	require.NoError(t, err)
	data = buildSelfUserData(fresh)
	assert.True(t, data["enterprise_unlocked"].(bool))
}

package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedBindInviterUser(t *testing.T, id int, username string, inviterId int) *User {
	t.Helper()
	user := &User{
		Id:        id,
		Username:  username,
		Password:  "password123",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   username + "-aff",
		InviterId: inviterId,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func loadBindInviterTarget(t *testing.T, id int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("id", "inviter_id").Where("id = ?", id).First(&user).Error)
	return user
}

func TestBindInviterForUsersBindsEverySelectedUser(t *testing.T) {
	truncateTables(t)
	seedBindInviterUser(t, 20, "bind-inviter", 0)
	seedBindInviterUser(t, 21, "bind-target-one", 0)
	seedBindInviterUser(t, 22, "bind-target-two", 0)
	seedBindInviterUser(t, 23, "bind-target-three", 0)

	updated, err := BindInviterForUsers([]int{21, 22, 23}, 20)
	require.NoError(t, err)
	assert.Equal(t, 3, updated)

	for _, id := range []int{21, 22, 23} {
		assert.Equal(t, 20, loadBindInviterTarget(t, id).InviterId, "user %d must point at the new inviter", id)
	}

	var charges int64
	require.NoError(t, DB.Model(&ReferralCharge{}).Count(&charges).Error)
	assert.Zero(t, charges, "manual binding must not replay invitation rewards")
}

func TestBindInviterForUsersOverwritesThePreviousInviter(t *testing.T) {
	truncateTables(t)
	seedBindInviterUser(t, 30, "bind-old-inviter", 0)
	seedBindInviterUser(t, 31, "bind-new-inviter", 0)
	seedBindInviterUser(t, 32, "bind-moved-user", 30)

	updated, err := BindInviterForUsers([]int{32}, 31)
	require.NoError(t, err)
	assert.Equal(t, 1, updated)
	assert.Equal(t, 31, loadBindInviterTarget(t, 32).InviterId)
}

func TestBindInviterForUsersAcceptsAlreadyBoundTargets(t *testing.T) {
	truncateTables(t)
	seedBindInviterUser(t, 80, "bind-existing-inviter", 0)
	seedBindInviterUser(t, 81, "bind-already-bound", 80)
	seedBindInviterUser(t, 82, "bind-new-target", 0)

	// MySQL reports changed rows rather than matched rows by default. Model
	// that driver result after the real update so SQLite tests cover retries.
	changedRows := int64(1)
	const callback = "test:inviter_mysql_changed_rows"
	require.NoError(t, DB.Callback().Update().After("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Error == nil && tx.Statement.Table == "users" {
			tx.RowsAffected = changedRows
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, DB.Callback().Update().Remove(callback))
	})

	updated, err := BindInviterForUsers([]int{81, 82}, 80)
	require.NoError(t, err)
	assert.Equal(t, 2, updated)
	assert.Equal(t, 80, loadBindInviterTarget(t, 82).InviterId)

	changedRows = 0
	updated, err = BindInviterForUsers([]int{81, 82}, 80)
	require.NoError(t, err)
	assert.Equal(t, 2, updated)
}

func TestBindInviterForUsersRejectsSelfBinding(t *testing.T) {
	truncateTables(t)
	seedBindInviterUser(t, 40, "bind-self-user", 0)

	_, err := BindInviterForUsers([]int{40}, 40)
	assert.ErrorIs(t, err, ErrInviterSelf)
	assert.Zero(t, loadBindInviterTarget(t, 40).InviterId)
}

func TestBindInviterForUsersRejectsCycle(t *testing.T) {
	truncateTables(t)
	seedBindInviterUser(t, 50, "bind-cycle-root", 0)
	seedBindInviterUser(t, 51, "bind-cycle-child", 50)

	// Making the root point at its own downstream user closes the cycle.
	_, err := BindInviterForUsers([]int{50}, 51)
	assert.ErrorIs(t, err, ErrInviterCycle)
	assert.Zero(t, loadBindInviterTarget(t, 50).InviterId)
}

func TestBindInviterForUsersRejectsUnknownInviter(t *testing.T) {
	truncateTables(t)
	seedBindInviterUser(t, 60, "bind-orphan-target", 0)

	_, err := BindInviterForUsers([]int{60}, 9999)
	assert.ErrorIs(t, err, ErrInviterNotFound)
	assert.Zero(t, loadBindInviterTarget(t, 60).InviterId)
}

func TestBindInviterForUsersRollsBackWhenATargetIsGone(t *testing.T) {
	truncateTables(t)
	seedBindInviterUser(t, 70, "bind-live-inviter", 0)
	seedBindInviterUser(t, 71, "bind-live-target", 0)

	_, err := BindInviterForUsers([]int{71, 9998}, 70)
	assert.ErrorIs(t, err, ErrInviterTargetMissing)
	assert.Zero(t, loadBindInviterTarget(t, 71).InviterId, "a partial selection update must be rolled back")
}

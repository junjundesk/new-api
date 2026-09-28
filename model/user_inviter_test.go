package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedInviterPair(t *testing.T) {
	t.Helper()
	inviter := &User{
		Id:       10,
		Username: "inviter-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "inviter-aff",
	}
	invitee := &User{
		Id:        11,
		Username:  "invitee-user",
		Password:  "password123",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   "invitee-aff",
		InviterId: 10,
	}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Create(invitee).Error)
}

func indexUsersById(users []*User) map[int]*User {
	byId := make(map[int]*User, len(users))
	for _, user := range users {
		byId[user.Id] = user
	}
	return byId
}

func TestGetAllUsersAttachesInviterUsername(t *testing.T) {
	truncateTables(t)
	seedInviterPair(t)

	users, total, err := GetAllUsers(&common.PageInfo{Page: 1, PageSize: 20}, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	byId := indexUsersById(users)
	require.Contains(t, byId, 11)
	assert.Equal(t, 10, byId[11].InviterId)
	assert.Equal(t, "inviter-user", byId[11].InviterUsername)
	assert.Empty(t, byId[10].InviterUsername)
}

func TestSearchUsersAttachesInviterUsername(t *testing.T) {
	truncateTables(t)
	seedInviterPair(t)

	users, total, err := SearchUsers("user", "", nil, nil, 0, 20, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	byId := indexUsersById(users)
	require.Contains(t, byId, 11)
	assert.Equal(t, "inviter-user", byId[11].InviterUsername)
	assert.Empty(t, byId[10].InviterUsername)
}

func TestInviterUsernameIsNotPersisted(t *testing.T) {
	truncateTables(t)

	user := &User{
		Id:              12,
		Username:        "derived-field-user",
		Password:        "password123",
		Role:            common.RoleCommonUser,
		Status:          common.UserStatusEnabled,
		Group:           "default",
		AffCode:         "derived-aff",
		InviterId:       10,
		InviterUsername: "should-not-be-stored",
	}
	require.NoError(t, DB.Create(user).Error)

	reloaded := User{Id: 12}
	require.NoError(t, DB.First(&reloaded).Error)
	assert.Empty(t, reloaded.InviterUsername)
}

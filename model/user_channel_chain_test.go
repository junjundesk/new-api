package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelChainTest(t *testing.T) *User {
	t.Helper()
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&UserChannelChain{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Token{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&UserChannelChain{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Token{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
	})
	user := User{
		Id:       92001,
		Username: "channel-chain-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, DB.Create(&user).Error)
	return &user
}

func newModelChainToken(t *testing.T, user *User, key string, chain *UserChannelChain) Token {
	t.Helper()
	token := Token{
		UserId:          user.Id,
		Key:             key,
		Status:          common.TokenStatusEnabled,
		Name:            key,
		CreatedTime:     common.GetTimestamp(),
		AccessedTime:    common.GetTimestamp(),
		ExpiredTime:     -1,
		UnlimitedQuota:  true,
		Group:           "chain:" + chain.ChainId,
		FallbackEnabled: true,
	}
	require.NoError(t, token.SetFallbackModels([]setting.TokenFallbackModel{{SourceModel: "model-x", Group: "vip", Model: "model-y"}}))
	require.NoError(t, DB.Create(&token).Error)
	return token
}

func TestParseUserChannelChain(t *testing.T) {
	chainId, ok := ParseUserChannelChain("chain:OUwA5H5q")
	require.True(t, ok)
	assert.Equal(t, "OUwA5H5q", chainId)

	_, ok = ParseUserChannelChain("chain")
	assert.False(t, ok)
	_, ok = ParseUserChannelChain("chain:")
	assert.False(t, ok)
	_, ok = ParseUserChannelChain("default")
	assert.False(t, ok)
}

func TestResolveLogGroupName(t *testing.T) {
	user := setupChannelChainTest(t)
	chain, err := CreateUserChannelChain(user.Id, "customer-priority", []string{"vip", "default"})
	require.NoError(t, err)

	assert.Equal(t, "customer-priority", resolveLogGroupName(user.Id, "chain:"+chain.ChainId))
	assert.Equal(t, "default", resolveLogGroupName(user.Id, "default"))
	assert.Equal(t, "chain:missing", resolveLogGroupName(user.Id, "chain:missing"))
}

func TestUserChannelChainCRUDAndTokenReset(t *testing.T) {
	user := setupChannelChainTest(t)

	chain, err := CreateUserChannelChain(user.Id, "chain-a", []string{"vip", "default"})
	require.NoError(t, err)
	require.NotEmpty(t, chain.ChainId)
	assert.Equal(t, []string{"vip", "default"}, chain.GetGroupList())

	chainGroup := "chain:" + chain.ChainId
	token := Token{
		UserId:          user.Id,
		Key:             "channel-chain-token-key",
		Status:          common.TokenStatusEnabled,
		Name:            "chain-token",
		CreatedTime:     common.GetTimestamp(),
		AccessedTime:    common.GetTimestamp(),
		ExpiredTime:     -1,
		RemainQuota:     0,
		UnlimitedQuota:  true,
		Group:           chainGroup,
		CrossGroupRetry: false,
	}
	require.NoError(t, DB.Create(&token).Error)

	err = UpdateUserChannelChain(user.Id, chain.ChainId, "chain-b", []string{"default", "vip"})
	require.NoError(t, err)
	updated, err := GetUserChannelChain(user.Id, chain.ChainId)
	require.NoError(t, err)
	assert.Equal(t, "chain-b", updated.Name)
	assert.Equal(t, []string{"default", "vip"}, updated.GetGroupList())

	resetTokens, err := DeleteUserChannelChain(user.Id, chain.ChainId)
	require.NoError(t, err)
	assert.Equal(t, 1, resetTokens)

	_, err = GetUserChannelChain(user.Id, chain.ChainId)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var stored Token
	require.NoError(t, DB.First(&stored, "id = ?", token.Id).Error)
	assert.Equal(t, "default", stored.Group)
	assert.False(t, stored.CrossGroupRetry)
	assert.Empty(t, stored.AutoGroups)
}

func TestCreateUserChannelChainEnforcesLimit(t *testing.T) {
	user := setupChannelChainTest(t)

	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&UserChannelChain{}).Error)
	for i := 0; i < MaxUserChannelChains; i++ {
		_, err := CreateUserChannelChain(user.Id, "chain", []string{"default"})
		require.NoError(t, err)
	}
	_, err := CreateUserChannelChain(user.Id, "overflow", []string{"default"})
	require.ErrorIs(t, err, ErrChannelChainLimit)
}

func TestUserChannelChainModelsSetItemsDerivesOrderedGroups(t *testing.T) {
	items := []UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x", "model-y"}},
		{Group: "default", Models: []string{"model-x"}},
	}
	chain := &UserChannelChain{Type: UserChannelChainTypeModels}
	require.NoError(t, chain.SetItems(items))
	assert.Equal(t, []string{"vip", "default"}, chain.GetGroupList())
	// A stale persisted groups column cannot widen a models chain.
	require.NoError(t, chain.SetGroupList([]string{"stale"}))
	assert.Equal(t, []string{"vip", "default"}, chain.GetGroupList())

	stored, err := chain.GetItems()
	require.NoError(t, err)
	assert.Equal(t, items, stored)

	legacy := &UserChannelChain{}
	assert.Equal(t, UserChannelChainTypeGroups, legacy.GetType())
	rows, err := legacy.GetItems()
	require.NoError(t, err)
	assert.Nil(t, rows)
}

func TestUserChannelChainModelsSetItemsRejectsInvalidRows(t *testing.T) {
	longName := strings.Repeat("m", setting.MaxTokenModelNameLength+1)
	tooManyModels := make([]string, MaxModelsPerUserChannelChainItem+1)
	for i := range tooManyModels {
		tooManyModels[i] = fmt.Sprintf("model-%d", i)
	}
	tooManyRows := make([]UserChannelChainItem, MaxGroupsPerUserChannelChain+1)
	for i := range tooManyRows {
		tooManyRows[i] = UserChannelChainItem{Group: fmt.Sprintf("group-%d", i), Models: []string{"model-x"}}
	}
	tests := []struct {
		name  string
		items []UserChannelChainItem
		want  string
	}{
		{name: "empty rows", items: nil, want: "at least one group row"},
		{name: "empty group", items: []UserChannelChainItem{{Group: "", Models: []string{"model-x"}}}, want: "pricing group name cannot be empty"},
		{name: "row without models", items: []UserChannelChainItem{{Group: "vip"}}, want: "at least one model"},
		{name: "duplicate group", items: []UserChannelChainItem{{Group: "vip", Models: []string{"model-x"}}, {Group: "vip", Models: []string{"model-y"}}}, want: "duplicated"},
		{name: "nested chain prefix", items: []UserChannelChainItem{{Group: "chain:", Models: []string{"model-x"}}}, want: "cannot be another channel chain"},
		{name: "nested chain id", items: []UserChannelChainItem{{Group: "chain:abc", Models: []string{"model-x"}}}, want: "cannot be another channel chain"},
		{name: "blank model", items: []UserChannelChainItem{{Group: "vip", Models: []string{"model-x", " "}}}, want: "cannot be empty"},
		{name: "model with space", items: []UserChannelChainItem{{Group: "vip", Models: []string{"model x"}}}, want: "whitespace or control characters"},
		{name: "model with control", items: []UserChannelChainItem{{Group: "vip", Models: []string{"model\x00x"}}}, want: "whitespace or control characters"},
		{name: "model too long", items: []UserChannelChainItem{{Group: "vip", Models: []string{longName}}}, want: "exceeds"},
		{name: "duplicate model", items: []UserChannelChainItem{{Group: "vip", Models: []string{"model-x", "model-x"}}}, want: "duplicated"},
		{name: "too many models", items: []UserChannelChainItem{{Group: "vip", Models: tooManyModels}}, want: "at most 100 models"},
		{name: "too many rows", items: tooManyRows, want: "at most 10 group rows"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chain := &UserChannelChain{Type: UserChannelChainTypeModels}
			err := chain.SetItems(test.items)
			require.ErrorContains(t, err, test.want)
			assert.Empty(t, chain.Items)
		})
	}

	atLimitModels := tooManyModels[:MaxModelsPerUserChannelChainItem]
	chain := &UserChannelChain{Type: UserChannelChainTypeModels}
	require.NoError(t, chain.SetItems([]UserChannelChainItem{{Group: "vip", Models: atLimitModels}}))
	assert.Equal(t, []string{"vip"}, chain.GetGroupList())

	atLimitModelName := strings.Repeat("m", setting.MaxTokenModelNameLength)
	require.NoError(t, chain.SetItems([]UserChannelChainItem{{Group: "vip", Models: []string{atLimitModelName}}}))
}

func TestUserChannelChainModelsSetItemsEnforcesBytesLimit(t *testing.T) {
	items := make([]UserChannelChainItem, 0, 4)
	for row := 0; row < 4; row++ {
		models := make([]string, MaxModelsPerUserChannelChainItem)
		for i := range models {
			models[i] = fmt.Sprintf("%s-%02d-%03d", strings.Repeat("m", 190), row, i)
		}
		items = append(items, UserChannelChainItem{Group: fmt.Sprintf("group-%d", row), Models: models})
	}
	chain := &UserChannelChain{Type: UserChannelChainTypeModels}
	err := chain.SetItems(items)
	require.ErrorContains(t, err, "exceed")
	assert.Empty(t, chain.Items)
	assert.Empty(t, chain.GetGroupList())
}

func TestUserChannelChainModelsGetItemsFailsClosedOnPersistedData(t *testing.T) {
	chain := &UserChannelChain{ChainId: "broken", Type: UserChannelChainTypeModels}
	_, err := chain.GetItems()
	require.ErrorContains(t, err, "rows are empty")

	chain.Items = "{not json"
	_, err = chain.GetItems()
	require.ErrorContains(t, err, "failed to parse")

	chain.Items = "[" + strings.Repeat(" ", MaxUserChannelChainItemsBytes) + "]"
	_, err = chain.GetItems()
	require.ErrorContains(t, err, "exceed")

	// Rows persisted without going through SetItems must still fail closed.
	chain.Items = `[{"group":"vip","models":[]}]`
	_, err = chain.GetItems()
	require.ErrorContains(t, err, "at least one model")
	assert.Nil(t, chain.GetGroupList())

	unknown := &UserChannelChain{ChainId: "unknown", Type: "custom"}
	assert.Equal(t, "custom", unknown.GetType())
	_, err = unknown.GetItems()
	require.ErrorIs(t, err, ErrInvalidChannelChainType)
	assert.Nil(t, unknown.GetGroupList())
}

func TestUserChannelChainWithConfigCRUDKeepsTypeImmutable(t *testing.T) {
	user := setupChannelChainTest(t)
	items := []UserChannelChainItem{
		{Group: "vip", Models: []string{"model-x"}},
		{Group: "default", Models: []string{"model-y"}},
	}
	chain, err := CreateUserChannelChainWithConfig(user.Id, "models-chain", UserChannelChainTypeModels, []string{"unused"}, items)
	require.NoError(t, err)
	assert.Equal(t, UserChannelChainTypeModels, chain.GetType())
	assert.Equal(t, []string{"vip", "default"}, chain.GetGroupList())

	stored, err := GetUserChannelChain(user.Id, chain.ChainId)
	require.NoError(t, err)
	assert.Equal(t, UserChannelChainTypeModels, stored.Type)
	storedItems, err := stored.GetItems()
	require.NoError(t, err)
	assert.Equal(t, items, storedItems)

	require.ErrorIs(t, UpdateUserChannelChainWithConfig(user.Id, chain.ChainId, "renamed", UserChannelChainTypeGroups, []string{"default"}, nil), ErrChannelChainTypeImmutable)
	require.ErrorIs(t, UpdateUserChannelChainWithConfig(user.Id, chain.ChainId, "renamed", "custom", nil, nil), ErrInvalidChannelChainType)
	_, err = CreateUserChannelChainWithConfig(user.Id, "unknown", "custom", nil, items)
	require.ErrorIs(t, err, ErrInvalidChannelChainType)
	_, err = CreateUserChannelChainWithConfig(user.Id, "groups-with-rows", UserChannelChainTypeGroups, []string{"default"}, items)
	require.ErrorContains(t, err, "groups chain cannot contain model rows")

	updatedItems := []UserChannelChainItem{{Group: "default", Models: []string{"model-z"}}}
	require.NoError(t, UpdateUserChannelChainWithConfig(user.Id, chain.ChainId, "renamed", UserChannelChainTypeModels, nil, updatedItems))
	reloaded, err := GetUserChannelChain(user.Id, chain.ChainId)
	require.NoError(t, err)
	assert.Equal(t, "renamed", reloaded.Name)
	assert.Equal(t, []string{"default"}, reloaded.GetGroupList())

	// A rejected update must not persist a partial configuration.
	require.Error(t, UpdateUserChannelChainWithConfig(user.Id, chain.ChainId, "should-not-save", UserChannelChainTypeModels, nil, []UserChannelChainItem{{Group: "vip"}}))
	unchanged, err := GetUserChannelChain(user.Id, chain.ChainId)
	require.NoError(t, err)
	assert.Equal(t, "renamed", unchanged.Name)
	unchangedItems, err := unchanged.GetItems()
	require.NoError(t, err)
	assert.Equal(t, updatedItems, unchangedItems)

	legacy, err := CreateUserChannelChain(user.Id, "legacy", []string{"default"})
	require.NoError(t, err)
	assert.Equal(t, UserChannelChainTypeGroups, legacy.GetType())
	storedLegacy, err := GetUserChannelChain(user.Id, legacy.ChainId)
	require.NoError(t, err)
	assert.Equal(t, UserChannelChainTypeGroups, storedLegacy.Type)
	require.ErrorIs(t, UpdateUserChannelChainWithConfig(user.Id, legacy.ChainId, "legacy", UserChannelChainTypeModels, nil, updatedItems), ErrChannelChainTypeImmutable)
}

func TestDeleteUserChannelChainDisablesFallbackForModelsChainsOnly(t *testing.T) {
	user := setupChannelChainTest(t)
	modelsChain, err := CreateUserChannelChainWithConfig(user.Id, "models", UserChannelChainTypeModels, nil, []UserChannelChainItem{{Group: "vip", Models: []string{"model-x"}}})
	require.NoError(t, err)
	modelsToken := newModelChainToken(t, user, "models-chain-token", modelsChain)
	draft, err := modelsToken.GetFallbackModels()
	require.NoError(t, err)
	require.Len(t, draft, 1)

	reset, err := DeleteUserChannelChain(user.Id, modelsChain.ChainId)
	require.NoError(t, err)
	assert.Equal(t, 1, reset)
	var storedModelsToken Token
	require.NoError(t, DB.First(&storedModelsToken, modelsToken.Id).Error)
	assert.Equal(t, "default", storedModelsToken.Group)
	assert.False(t, storedModelsToken.FallbackEnabled)
	assert.False(t, storedModelsToken.CrossGroupRetry)
	assert.Empty(t, storedModelsToken.AutoGroups)
	storedDraft, err := storedModelsToken.GetFallbackModels()
	require.NoError(t, err)
	assert.Equal(t, draft, storedDraft)

	groupsChain, err := CreateUserChannelChain(user.Id, "groups", []string{"default"})
	require.NoError(t, err)
	groupsToken := newModelChainToken(t, user, "groups-chain-token", groupsChain)
	reset, err = DeleteUserChannelChain(user.Id, groupsChain.ChainId)
	require.NoError(t, err)
	assert.Equal(t, 1, reset)
	var storedGroupsToken Token
	require.NoError(t, DB.First(&storedGroupsToken, groupsToken.Id).Error)
	assert.Equal(t, "default", storedGroupsToken.Group)
	assert.True(t, storedGroupsToken.FallbackEnabled)
	groupsDraft, err := storedGroupsToken.GetFallbackModels()
	require.NoError(t, err)
	assert.Equal(t, draft, groupsDraft)
}

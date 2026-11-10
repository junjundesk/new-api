package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

// requestChannelChainGroups resolves the ordered candidate groups for the
// current request. The chain snapshot loaded by auth is preferred; the legacy
// context key and a direct DB load keep non-auth callers working.
func requestChannelChainGroups(param *RetryParam) ([]string, bool, error) {
	if groups, isChain, err := GetRequestChannelChainGroups(param.Ctx, param.ModelName); isChain {
		return groups, true, err
	}
	if !strings.HasPrefix(param.TokenGroup, "chain:") {
		return nil, false, nil
	}
	chainId, ok := model.ParseUserChannelChain(param.TokenGroup)
	if !ok {
		// A "chain:" prefix with no id is a malformed token group: it must stay a
		// chain request and fail closed instead of selecting an ordinary group.
		return nil, true, fmt.Errorf("invalid channel chain token group %q", param.TokenGroup)
	}
	userId := common.GetContextKeyInt(param.Ctx, constant.ContextKeyUserId)
	if userId == 0 {
		userId = param.Ctx.GetInt("id")
	}
	if userId == 0 {
		return nil, true, fmt.Errorf("failed to load channel chain %s: missing user context", chainId)
	}
	chain, err := model.GetUserChannelChain(userId, chainId)
	if err != nil {
		return nil, true, fmt.Errorf("failed to load channel chain %s: %w", chainId, err)
	}
	// TokenAuth validates before caching a snapshot; a direct DB load (non-auth
	// caller or a request context that lost the snapshot) must not turn an
	// unvalidated row into routing authority.
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
	if userGroup == "" {
		loadedGroup, err := model.GetUserGroup(userId, false)
		if err != nil {
			return nil, true, fmt.Errorf("failed to load user group for chain %s: %w", chainId, err)
		}
		userGroup = loadedGroup
	}
	if err := ValidateUserChannelChain(chain, userGroup, false); err != nil {
		return nil, true, fmt.Errorf("channel chain %s is unavailable: %w", chainId, err)
	}
	SetRequestChannelChain(param.Ctx, chain)
	groups, err := resolveChannelChainGroupsForModel(chain, param.ModelName)
	if err != nil {
		return nil, true, err
	}
	return groups, true, nil
}

func getChannelChainSelection(param *RetryParam) (*model.Channel, string, error, bool) {
	groups, isChain, err := requestChannelChainGroups(param)
	if !isChain {
		return nil, "", nil, false
	}
	if err != nil {
		// A models-type chain that cannot serve this model must fail closed;
		// never fall through to an ordinary group selection.
		return nil, param.TokenGroup, err, true
	}

	used := make(map[int]struct{})
	for _, idStr := range param.Ctx.GetStringSlice("use_channel") {
		if id, err := strconv.Atoi(idStr); err == nil {
			used[id] = struct{}{}
		}
	}

	startGroupIndex := 0
	if index, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
		if value, ok := index.(int); ok {
			startGroupIndex = value
		}
	}

	for i := startGroupIndex; i < len(groups); i++ {
		group := groups[i]
		priorityRetry := param.GetRetry()
		if i > startGroupIndex {
			priorityRetry = 0
		}

		channel, _ := model.GetRandomSatisfiedChannel(group, param.ModelName, priorityRetry, param.RequestPath)
		if channel == nil {
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
			param.SetRetry(0)
			continue
		}
		if _, alreadyUsed := used[channel.Id]; alreadyUsed {
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
			param.SetRetry(0)
			continue
		}

		common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, group)
		if priorityRetry >= common.RetryTimes {
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
			param.SetRetry(0)
			param.ResetRetryNextTry()
		} else {
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
		}
		return channel, group, nil, true
	}
	return nil, param.TokenGroup, nil, true
}

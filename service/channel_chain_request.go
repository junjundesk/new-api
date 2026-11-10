package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// ErrChannelChainModelNotAvailable marks a models-type channel chain that has no
// row providing the requested model. Runtime resolution must fail closed instead
// of falling back to all groups or to the token's ordinary group.
var ErrChannelChainModelNotAvailable = errors.New("channel chain has no group for the requested model")

// SetRequestChannelChain stores the request-scoped channel chain snapshot.
// Auth loads the chain once and every later consumer (selection, retry budget,
// model list, fallback gate) reads this snapshot instead of querying the DB.
func SetRequestChannelChain(c *gin.Context, chain *model.UserChannelChain) {
	if c == nil || chain == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyChannelChainDefinition, chain)
	// The derived group cache and the legacy key belong to the previous
	// snapshot: reset both so a later chain load cannot reuse stale candidates.
	common.SetContextKey(c, constant.ContextKeyChannelChainGroups, map[string][]string(nil))
	if chain.GetType() == model.UserChannelChainTypeGroups {
		groups := chain.GetGroupList()
		if len(groups) > 0 {
			// Keep the legacy groups key for groups-type chains only: existing
			// consumers interpret it as an all-groups list.
			common.SetContextKey(c, constant.ContextKeyChannelChain, groups)
		} else {
			// Fail closed: never leave a stale all-groups list behind.
			common.SetContextKey(c, constant.ContextKeyChannelChain, []string(nil))
		}
		return
	}
	// A models-type chain must never look like an all-groups chain to legacy
	// consumers of the old key.
	common.SetContextKey(c, constant.ContextKeyChannelChain, []string(nil))
}

// GetRequestChannelChain returns the request-scoped chain snapshot loaded by auth.
func GetRequestChannelChain(c *gin.Context) (*model.UserChannelChain, bool) {
	if c == nil {
		return nil, false
	}
	chain, ok := common.GetContextKeyType[*model.UserChannelChain](c, constant.ContextKeyChannelChainDefinition)
	if !ok || chain == nil {
		return nil, false
	}
	return chain, true
}

// IsRequestChannelChain reports whether the current request is routed through a
// channel chain, including request contexts that only carry the legacy groups key.
func IsRequestChannelChain(c *gin.Context) bool {
	if _, ok := GetRequestChannelChain(c); ok {
		return true
	}
	return len(common.GetContextKeyStringSlice(c, constant.ContextKeyChannelChain)) > 0
}

// IsRequestModelChannelChain reports whether the current request is routed
// through a models-type chain.
func IsRequestModelChannelChain(c *gin.Context) bool {
	chain, ok := GetRequestChannelChain(c)
	return ok && chain.GetType() == model.UserChannelChainTypeModels
}

// GetRequestChannelChainGroups resolves the ordered candidate groups for the
// current request model. For models-type chains only rows that currently
// provide the model are candidates; an empty result is an error and must not
// degrade to a normal group selection.
func GetRequestChannelChainGroups(c *gin.Context, modelName string) ([]string, bool, error) {
	if c == nil {
		return nil, false, nil
	}
	chain, ok := GetRequestChannelChain(c)
	if ok {
		if groups, hit := cachedRequestChannelChainGroups(c, modelName); hit {
			return groups, true, nil
		}
		groups, err := resolveChannelChainGroupsForModel(chain, modelName)
		if err != nil {
			return nil, true, err
		}
		cacheRequestChannelChainGroups(c, modelName, groups)
		return groups, true, nil
	}
	if groups := common.GetContextKeyStringSlice(c, constant.ContextKeyChannelChain); len(groups) > 0 {
		return groups, true, nil
	}
	return nil, false, nil
}

func resolveChannelChainGroupsForModel(chain *model.UserChannelChain, modelName string) ([]string, error) {
	if chain == nil {
		return nil, errors.New("channel chain is required")
	}
	switch chain.GetType() {
	case model.UserChannelChainTypeGroups:
		groups := chain.GetGroupList()
		if len(groups) == 0 {
			return nil, fmt.Errorf("channel chain %s has no groups", chain.ChainId)
		}
		return groups, nil
	case model.UserChannelChainTypeModels:
		if strings.TrimSpace(modelName) == "" {
			return nil, fmt.Errorf("%w: request model is empty", ErrChannelChainModelNotAvailable)
		}
		groups, err := GetUserChannelChainModelGroups(chain, modelName)
		if err != nil {
			return nil, err
		}
		if len(groups) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrChannelChainModelNotAvailable, modelName)
		}
		return groups, nil
	default:
		return nil, fmt.Errorf("channel chain %s has unsupported type %q", chain.ChainId, chain.GetType())
	}
}

func cachedRequestChannelChainGroups(c *gin.Context, modelName string) ([]string, bool) {
	cache, ok := common.GetContextKeyType[map[string][]string](c, constant.ContextKeyChannelChainGroups)
	if !ok {
		return nil, false
	}
	groups, ok := cache[modelName]
	return groups, ok
}

func cacheRequestChannelChainGroups(c *gin.Context, modelName string, groups []string) {
	cache, _ := common.GetContextKeyType[map[string][]string](c, constant.ContextKeyChannelChainGroups)
	if cache == nil {
		cache = make(map[string][]string)
		common.SetContextKey(c, constant.ContextKeyChannelChainGroups, cache)
	}
	cache[modelName] = groups
}

package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ValidateUserChannelChainGroups validates a legacy groups-type chain: at least
// one selectable group, at most 10 groups, no duplicates and no nested chains.
func ValidateUserChannelChainGroups(userGroup string, groups []string) error {
	if len(groups) == 0 {
		return errors.New("at least one pricing group is required")
	}
	if len(groups) > model.MaxGroupsPerUserChannelChain {
		return fmt.Errorf("a group chain can contain at most %d groups", model.MaxGroupsPerUserChannelChain)
	}
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if group == "" {
			return errors.New("pricing group name cannot be empty")
		}
		if strings.HasPrefix(group, "chain:") {
			return errors.New("channel chain groups cannot be nested")
		}
		if _, ok := seen[group]; ok {
			return fmt.Errorf("group %s is duplicated", group)
		}
		seen[group] = struct{}{}
		if !IsUserSelectableGroup(userGroup, group) {
			return fmt.Errorf("group %s is unavailable or unauthorized", group)
		}
	}
	return nil
}

// ValidateUserChannelChain validates a chain configuration for the given user
// group. When checkModels is false only the structure and group authorization
// are verified, so runtime lookups do not fail when one selected model is
// temporarily disabled.
func ValidateUserChannelChain(chain *model.UserChannelChain, userGroup string, checkModels bool) error {
	if chain == nil {
		return errors.New("channel chain is required")
	}
	switch chain.GetType() {
	case model.UserChannelChainTypeGroups:
		return ValidateUserChannelChainGroups(userGroup, chain.GetGroupList())
	case model.UserChannelChainTypeModels:
		items, err := chain.GetItems()
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return errors.New("a model chain requires at least one group row")
		}
		// Validate the persisted rows with the same write-path rules applied by
		// SetItems, so manually built or stored rows cannot drift.
		validationChain := &model.UserChannelChain{Type: model.UserChannelChainTypeModels}
		if err := validationChain.SetItems(items); err != nil {
			return err
		}
		if checkModels && model.DB == nil {
			return errors.New("database is not initialized")
		}
		for _, item := range items {
			if !IsUserSelectableGroup(userGroup, item.Group) {
				return fmt.Errorf("group %s is unavailable or unauthorized", item.Group)
			}
			if !checkModels {
				continue
			}
			enabledModels := make(map[string]struct{})
			for _, modelName := range model.GetGroupEnabledModels(item.Group) {
				enabledModels[modelName] = struct{}{}
			}
			for _, modelName := range item.Models {
				if _, ok := enabledModels[modelName]; !ok {
					return fmt.Errorf("model %s is not available in group %s", modelName, item.Group)
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: %q", model.ErrInvalidChannelChainType, chain.GetType())
	}
}

// GetUserChannelChainModels returns the models a chain may currently route to.
// A models-type chain uses the union of each row's selected models intersect
// that group's enabled catalog; a legacy groups-type chain exposes every model
// enabled in its groups. The result is never nil.
func GetUserChannelChainModels(chain *model.UserChannelChain) ([]string, error) {
	if chain == nil {
		return nil, errors.New("channel chain is required")
	}
	if model.DB == nil {
		return nil, errors.New("database is not initialized")
	}
	if chain.GetType() == model.UserChannelChainTypeGroups {
		// Keep the legacy channel-chain model listing stable and sorted; the
		// models-type catalog keeps its ordered row semantics below.
		models := GetGroupsEnabledModels(chain.GetGroupList())
		if models == nil {
			models = []string{}
		}
		sort.Strings(models)
		return models, nil
	}
	items, err := chain.GetItems()
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{})
	models := make([]string, 0)
	for _, item := range items {
		enabledModels := model.GetGroupEnabledModels(item.Group)
		if len(enabledModels) == 0 {
			continue
		}
		enabledSet := make(map[string]struct{}, len(enabledModels))
		for _, modelName := range enabledModels {
			enabledSet[modelName] = struct{}{}
		}
		for _, modelName := range item.Models {
			if _, ok := enabledSet[modelName]; !ok {
				continue
			}
			if _, ok := seen[modelName]; ok {
				continue
			}
			seen[modelName] = struct{}{}
			models = append(models, modelName)
		}
	}
	return models, nil
}

// GetUserChannelChainModelGroups returns the ordered candidate groups for a
// model. A groups-type chain exposes all of its groups; a models-type chain only
// exposes rows that explicitly select the model and have it enabled in that
// group. An empty result is authoritative and must not be widened by callers.
func GetUserChannelChainModelGroups(chain *model.UserChannelChain, modelName string) ([]string, error) {
	if chain == nil {
		return nil, errors.New("channel chain is required")
	}
	if chain.GetType() == model.UserChannelChainTypeGroups {
		return chain.GetGroupList(), nil
	}
	items, err := chain.GetItems()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(modelName) == "" {
		return nil, nil
	}
	if model.DB == nil {
		return nil, errors.New("database is not initialized")
	}
	// A row matches the exact request model, or the request's normalized
	// identifier when that row explicitly selected it AND the identifier is
	// currently enabled in the row's group. Row models are never generalized:
	// there is no prefix or date-suffix expansion. An empty result is
	// authoritative and must not be widened by callers.
	candidates := []string{modelName}
	if normalized := ratio_setting.FormatMatchingModelName(modelName); normalized != modelName {
		candidates = append(candidates, normalized)
	}
	groups := make([]string, 0)
	for _, item := range items {
		enabledModels := model.GetGroupEnabledModels(item.Group)
		for _, candidate := range candidates {
			selected := false
			for _, selectedModel := range item.Models {
				if selectedModel == candidate {
					selected = true
					break
				}
			}
			if !selected {
				continue
			}
			enabled := false
			for _, enabledModel := range enabledModels {
				if enabledModel == candidate {
					enabled = true
					break
				}
			}
			if enabled {
				groups = append(groups, item.Group)
				break
			}
		}
	}
	return groups, nil
}

func ResolveChannelChainGroups(userId int, group string) ([]string, error) {
	chainId, ok := model.ParseUserChannelChain(group)
	if !ok {
		return nil, errors.New("invalid channel chain")
	}
	chain, err := model.GetUserChannelChain(userId, chainId)
	if err != nil {
		return nil, err
	}
	return chain.GetGroupList(), nil
}

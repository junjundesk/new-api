package model

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"gorm.io/gorm"
)

const (
	MaxUserChannelChains             = 10
	MaxGroupsPerUserChannelChain     = 10
	MaxModelsPerUserChannelChainItem = 100
	MaxUserChannelChainItemsBytes    = 60 * 1024

	UserChannelChainTypeGroups = "groups"
	UserChannelChainTypeModels = "models"
)

var (
	ErrChannelChainLimit         = errors.New("channel chain limit reached")
	ErrInvalidChannelChainType   = errors.New("invalid channel chain type")
	ErrChannelChainTypeImmutable = errors.New("channel chain type cannot be changed")
)

// UserChannelChainItem is one row of a models-type chain: an ordered group and
// the models explicitly selected from that group's enabled catalog.
type UserChannelChainItem struct {
	Group  string   `json:"group"`
	Models []string `json:"models"`
}

type UserChannelChain struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId    int    `json:"user_id" gorm:"index"`
	ChainId   string `json:"chain_id" gorm:"type:varchar(32);uniqueIndex"`
	Name      string `json:"name" gorm:"type:varchar(64)"`
	Type      string `json:"type" gorm:"type:varchar(16)"`
	Groups    string `json:"-" gorm:"type:text"`
	Items     string `json:"-" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (UserChannelChain) TableName() string {
	return "user_group_chains"
}

// ParseUserChannelChain extracts a chain id from a token group such as chain:OUwA5H5q.
func ParseUserChannelChain(group string) (string, bool) {
	if !strings.HasPrefix(group, "chain:") || len(group) <= len("chain:") {
		return "", false
	}
	return strings.TrimPrefix(group, "chain:"), true
}

// normalizeUserChannelChainType resolves the legacy empty type to a groups
// chain and rejects unknown type values.
func normalizeUserChannelChainType(chainType string) (string, error) {
	switch chainType {
	case "", UserChannelChainTypeGroups:
		return UserChannelChainTypeGroups, nil
	case UserChannelChainTypeModels:
		return UserChannelChainTypeModels, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidChannelChainType, chainType)
	}
}

// GetType returns the chain type. Rows created before the type column existed
// have an empty value and are legacy groups-type chains.
func (chain *UserChannelChain) GetType() string {
	if chain.Type == "" {
		return UserChannelChainTypeGroups
	}
	return chain.Type
}

// validateUserChannelChainItems checks the structure of models-type rows:
// ordered unique groups, 1..100 unique valid model names per row, bounded row
// count and no nested chains.
func validateUserChannelChainItems(items []UserChannelChainItem) error {
	if len(items) == 0 {
		return errors.New("a model chain requires at least one group row")
	}
	if len(items) > MaxGroupsPerUserChannelChain {
		return fmt.Errorf("a model chain can contain at most %d group rows", MaxGroupsPerUserChannelChain)
	}
	seenGroups := make(map[string]struct{}, len(items))
	for i, item := range items {
		row := i + 1
		if item.Group == "" {
			return fmt.Errorf("row %d: pricing group name cannot be empty", row)
		}
		if strings.HasPrefix(item.Group, "chain:") {
			return fmt.Errorf("row %d: group %s cannot be another channel chain", row, item.Group)
		}
		if _, ok := seenGroups[item.Group]; ok {
			return fmt.Errorf("row %d: group %s is duplicated", row, item.Group)
		}
		seenGroups[item.Group] = struct{}{}
		if len(item.Models) == 0 {
			return fmt.Errorf("row %d: at least one model is required", row)
		}
		if len(item.Models) > MaxModelsPerUserChannelChainItem {
			return fmt.Errorf("row %d: at most %d models are allowed", row, MaxModelsPerUserChannelChainItem)
		}
		seenModels := make(map[string]struct{}, len(item.Models))
		for j, modelName := range item.Models {
			if strings.TrimSpace(modelName) == "" {
				return fmt.Errorf("row %d model %d: model name cannot be empty", row, j+1)
			}
			if utf8.RuneCountInString(modelName) > setting.MaxTokenModelNameLength {
				return fmt.Errorf("row %d model %d: model name exceeds %d characters", row, j+1, setting.MaxTokenModelNameLength)
			}
			for _, r := range modelName {
				if unicode.IsSpace(r) || unicode.IsControl(r) {
					return fmt.Errorf("row %d model %d: model name cannot contain whitespace or control characters", row, j+1)
				}
			}
			if _, ok := seenModels[modelName]; ok {
				return fmt.Errorf("row %d: model %s is duplicated", row, modelName)
			}
			seenModels[modelName] = struct{}{}
		}
	}
	return nil
}

// GetGroupList returns the ordered groups of the chain. Models-type chains
// derive them from their rows so a stale groups column can never widen the
// chain; unparseable rows fail closed with a nil result.
func (chain *UserChannelChain) GetGroupList() []string {
	switch chain.GetType() {
	case UserChannelChainTypeModels:
		items, err := chain.GetItems()
		if err != nil {
			common.SysError(fmt.Sprintf("failed to derive groups for channel chain %s: %v", chain.ChainId, err))
			return nil
		}
		groups := make([]string, 0, len(items))
		for _, item := range items {
			groups = append(groups, item.Group)
		}
		return groups
	case UserChannelChainTypeGroups:
		if chain.Groups == "" {
			return nil
		}
		var groups []string
		if err := common.UnmarshalJsonStr(chain.Groups, &groups); err != nil {
			common.SysError(fmt.Sprintf("failed to parse channel chain %s: %v", chain.ChainId, err))
			return nil
		}
		return groups
	default:
		common.SysError(fmt.Sprintf("failed to parse channel chain %s: invalid chain type %q", chain.ChainId, chain.Type))
		return nil
	}
}

func (chain *UserChannelChain) SetGroupList(groups []string) error {
	if len(groups) == 0 {
		chain.Groups = ""
		return nil
	}
	data, err := common.Marshal(groups)
	if err != nil {
		return err
	}
	chain.Groups = string(data)
	return nil
}

// GetItems returns the ordered rows of a models-type chain. Legacy groups-type
// chains have no rows and return nil.
func (chain *UserChannelChain) GetItems() ([]UserChannelChainItem, error) {
	switch chain.GetType() {
	case UserChannelChainTypeGroups:
		return nil, nil
	case UserChannelChainTypeModels:
		if chain.Items == "" {
			return nil, errors.New("channel chain rows are empty")
		}
		if len(chain.Items) > MaxUserChannelChainItemsBytes {
			return nil, fmt.Errorf("channel chain rows exceed %d bytes", MaxUserChannelChainItemsBytes)
		}
		var items []UserChannelChainItem
		if err := common.UnmarshalJsonStr(chain.Items, &items); err != nil {
			return nil, fmt.Errorf("failed to parse channel chain %s rows: %w", chain.ChainId, err)
		}
		// Persisted rows are validated on read as well so rows written outside
		// SetItems (or before these rules existed) fail closed.
		if err := validateUserChannelChainItems(items); err != nil {
			return nil, fmt.Errorf("invalid channel chain %s rows: %w", chain.ChainId, err)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidChannelChainType, chain.Type)
	}
}

// SetItems validates and stores the ordered rows of a models-type chain and
// derives the legacy group list from them without reordering.
func (chain *UserChannelChain) SetItems(items []UserChannelChainItem) error {
	if err := validateUserChannelChainItems(items); err != nil {
		return err
	}
	data, err := common.Marshal(items)
	if err != nil {
		return err
	}
	if len(data) > MaxUserChannelChainItemsBytes {
		return fmt.Errorf("channel chain rows exceed %d bytes", MaxUserChannelChainItemsBytes)
	}
	groups := make([]string, 0, len(items))
	for _, item := range items {
		groups = append(groups, item.Group)
	}
	if err := chain.SetGroupList(groups); err != nil {
		return err
	}
	chain.Items = string(data)
	return nil
}

func CountUserChannelChains(userId int) (int64, error) {
	var count int64
	err := DB.Model(&UserChannelChain{}).Where("user_id = ?", userId).Count(&count).Error
	return count, err
}

func GetUserChannelChains(userId int) ([]UserChannelChain, error) {
	chains := make([]UserChannelChain, 0)
	err := DB.Where("user_id = ?", userId).Order("id asc").Find(&chains).Error
	return chains, err
}

func GetUserChannelChain(userId int, chainId string) (*UserChannelChain, error) {
	chain := UserChannelChain{}
	err := DB.Where("user_id = ? and chain_id = ?", userId, chainId).First(&chain).Error
	return &chain, err
}

// CreateUserChannelChain creates a legacy groups-type chain.
func CreateUserChannelChain(userId int, name string, groups []string) (*UserChannelChain, error) {
	return CreateUserChannelChainWithConfig(userId, name, UserChannelChainTypeGroups, groups, nil)
}

// CreateUserChannelChainWithConfig creates a chain with an explicit type. The
// type is fixed once persisted.
func CreateUserChannelChainWithConfig(userId int, name string, chainType string, groups []string, items []UserChannelChainItem) (*UserChannelChain, error) {
	normalizedType, err := normalizeUserChannelChainType(chainType)
	if err != nil {
		return nil, err
	}
	count, err := CountUserChannelChains(userId)
	if err != nil {
		return nil, err
	}
	if count >= MaxUserChannelChains {
		return nil, ErrChannelChainLimit
	}

	chain := &UserChannelChain{
		UserId: userId,
		Name:   name,
		Type:   normalizedType,
	}
	for i := 0; i < 10; i++ {
		chainId := common.GetRandomString(8)
		var exists int64
		if err := DB.Model(&UserChannelChain{}).Where("chain_id = ?", chainId).Count(&exists).Error; err != nil {
			return nil, err
		}
		if exists == 0 {
			chain.ChainId = chainId
			break
		}
	}
	if chain.ChainId == "" {
		return nil, errors.New("failed to generate channel chain id")
	}
	if normalizedType == UserChannelChainTypeModels {
		if err := chain.SetItems(items); err != nil {
			return nil, err
		}
	} else {
		if len(items) > 0 {
			return nil, errors.New("a groups chain cannot contain model rows")
		}
		if err := chain.SetGroupList(groups); err != nil {
			return nil, err
		}
	}
	if err := DB.Create(chain).Error; err != nil {
		return nil, err
	}
	return chain, nil
}

// UpdateUserChannelChain updates a legacy groups-type chain.
func UpdateUserChannelChain(userId int, chainId string, name string, groups []string) error {
	return UpdateUserChannelChainWithConfig(userId, chainId, name, UserChannelChainTypeGroups, groups, nil)
}

// UpdateUserChannelChainWithConfig updates a chain configuration. The persisted
// chain type is immutable.
func UpdateUserChannelChainWithConfig(userId int, chainId string, name string, chainType string, groups []string, items []UserChannelChainItem) error {
	chain := UserChannelChain{}
	if err := DB.Where("user_id = ? and chain_id = ?", userId, chainId).First(&chain).Error; err != nil {
		return err
	}
	normalizedType, err := normalizeUserChannelChainType(chainType)
	if err != nil {
		return err
	}
	if normalizedType != chain.GetType() {
		return fmt.Errorf("%w: %s chain cannot become %s", ErrChannelChainTypeImmutable, chain.GetType(), normalizedType)
	}
	chain.Name = name
	if normalizedType == UserChannelChainTypeModels {
		if err := chain.SetItems(items); err != nil {
			return err
		}
	} else {
		if len(items) > 0 {
			return errors.New("a groups chain cannot contain model rows")
		}
		if err := chain.SetGroupList(groups); err != nil {
			return err
		}
	}
	return DB.Save(&chain).Error
}

// DeleteUserChannelChain removes a chain and resets bound tokens to the user
// group. Tokens bound to a models-type chain additionally have fallback
// disabled while their fallback draft models are preserved.
func DeleteUserChannelChain(userId int, chainId string) (int, error) {
	chain, err := GetUserChannelChain(userId, chainId)
	if err != nil {
		return 0, err
	}
	chainGroup := "chain:" + chainId
	var tokens []Token
	if err := DB.Where("user_id = ? and "+commonGroupCol+" = ?", userId, chainGroup).Find(&tokens).Error; err != nil {
		return 0, err
	}
	user, err := GetUserById(userId, false)
	if err != nil {
		return 0, err
	}
	disableFallback := chain.GetType() == UserChannelChainTypeModels
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? and chain_id = ?", userId, chainId).Delete(&UserChannelChain{}).Error; err != nil {
			return err
		}
		for _, token := range tokens {
			updates := map[string]interface{}{
				"group":             user.Group,
				"cross_group_retry": false,
				"auto_groups":       "",
			}
			columns := []string{"group", "cross_group_retry", "auto_groups"}
			if disableFallback {
				updates["fallback_enabled"] = false
				columns = append(columns, "fallback_enabled")
			}
			if err := tx.Model(&Token{}).Where("id = ?", token.Id).
				Select(columns).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if common.RedisEnabled {
		for _, token := range tokens {
			_ = cacheDeleteToken(token.Key)
		}
	}
	return len(tokens), nil
}

func CountUserTokensByChannelChain(userId int, chainId string) (int64, error) {
	var count int64
	group := "chain:" + chainId
	err := DB.Model(&Token{}).Where("user_id = ? and "+commonGroupCol+" = ?", userId, group).Count(&count).Error
	return count, err
}

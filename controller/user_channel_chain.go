package controller

import (
	"errors"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type channelChainRequest struct {
	Name   string                       `json:"name"`
	Type   string                       `json:"type"`
	Groups []string                     `json:"groups"`
	Items  []model.UserChannelChainItem `json:"items"`
}

func getUserChannelChainContext(c *gin.Context) (int, string, bool) {
	userId := c.GetInt("id")
	userGroup, err := model.GetUserGroup(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return 0, "", false
	}
	return userId, userGroup, true
}

func validateChannelChainRequest(c *gin.Context, req channelChainRequest, userGroup string) (*model.UserChannelChain, bool) {
	chain := &model.UserChannelChain{Name: strings.TrimSpace(req.Name), Type: req.Type}
	if chain.Name == "" {
		common.ApiErrorI18n(c, i18n.MsgNameCannotBeEmpty)
		return nil, false
	}
	if len(chain.Name) > 64 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return nil, false
	}
	switch chain.GetType() {
	case model.UserChannelChainTypeGroups:
		if len(req.Items) > 0 {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return nil, false
		}
		if err := chain.SetGroupList(req.Groups); err != nil {
			common.ApiError(c, err)
			return nil, false
		}
	case model.UserChannelChainTypeModels:
		if err := chain.SetItems(req.Items); err != nil {
			common.ApiError(c, err)
			return nil, false
		}
		if req.Groups != nil && !slices.Equal(req.Groups, chain.GetGroupList()) {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return nil, false
		}
	default:
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return nil, false
	}
	if err := service.ValidateUserChannelChain(chain, userGroup, true); err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	return chain, true
}

func GetUserChannelChains(c *gin.Context) {
	userId := c.GetInt("id")
	chains, err := model.GetUserChannelChains(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	chainItems := make([]gin.H, 0, len(chains))
	for _, chain := range chains {
		items, err := chain.GetItems()
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if items == nil {
			items = []model.UserChannelChainItem{}
		}
		chainItems = append(chainItems, gin.H{
			"id":         chain.ChainId,
			"name":       chain.Name,
			"type":       chain.GetType(),
			"items":      items,
			"groups":     chain.GetGroupList(),
			"created_at": chain.CreatedAt,
			"updated_at": chain.UpdatedAt,
		})
	}
	tokenUsage := make(map[string]int64, len(chains))
	for _, chain := range chains {
		count, err := model.CountUserTokensByChannelChain(userId, chain.ChainId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		tokenUsage["chain:"+chain.ChainId] = count
	}
	common.ApiSuccess(c, gin.H{
		"chains":               chainItems,
		"max_chains":           model.MaxUserChannelChains,
		"max_groups_per_chain": model.MaxGroupsPerUserChannelChain,
		"max_models_per_item":  model.MaxModelsPerUserChannelChainItem,
		"max_items_bytes":      model.MaxUserChannelChainItemsBytes,
		"token_usage":          tokenUsage,
	})
}

func CreateUserChannelChain(c *gin.Context) {
	var req channelChainRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId, userGroup, ok := getUserChannelChainContext(c)
	if !ok {
		return
	}
	config, ok := validateChannelChainRequest(c, req, userGroup)
	if !ok {
		return
	}
	chain, err := model.CreateUserChannelChainWithConfig(userId, config.Name, config.GetType(), config.GetGroupList(), req.Items)
	if err != nil {
		if errors.Is(err, model.ErrChannelChainLimit) {
			common.ApiErrorI18n(c, i18n.MsgBatchTooMany, map[string]any{"Max": model.MaxUserChannelChains})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, chain)
}

func UpdateUserChannelChain(c *gin.Context) {
	chainId := c.Param("chain_id")
	if chainId == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	userId := c.GetInt("id")
	if _, err := model.GetUserChannelChain(userId, chainId); err != nil {
		common.ApiErrorI18n(c, i18n.MsgNotFound)
		return
	}
	var req channelChainRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userGroup, err := model.GetUserGroup(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	config, ok := validateChannelChainRequest(c, req, userGroup)
	if !ok {
		return
	}
	if err := model.UpdateUserChannelChainWithConfig(userId, chainId, config.Name, config.GetType(), config.GetGroupList(), req.Items); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"chain_id": chainId})
}

func DeleteUserChannelChain(c *gin.Context) {
	chainId := c.Param("chain_id")
	if chainId == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	userId := c.GetInt("id")
	if _, err := model.GetUserChannelChain(userId, chainId); err != nil {
		common.ApiErrorI18n(c, i18n.MsgNotFound)
		return
	}
	resetTokens, err := model.DeleteUserChannelChain(userId, chainId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"reset_tokens": resetTokens})
}

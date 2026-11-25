package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func redemptionAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrRedemptionName):
		common.ApiErrorI18n(c, i18n.MsgRedemptionNameLength)
	case errors.Is(err, model.ErrRedemptionQuota):
		common.ApiErrorI18n(c, i18n.MsgRedemptionQuotaInvalid)
	case errors.Is(err, model.ErrRedemptionBonusRate):
		common.ApiErrorI18n(c, i18n.MsgRedemptionBonusRateInvalid)
	case errors.Is(err, model.ErrRedemptionBonusRuleConflict):
		common.ApiErrorI18n(c, i18n.MsgRedemptionBonusRuleConflict)
	case errors.Is(err, model.ErrRedemptionCategory):
		common.ApiErrorI18n(c, i18n.MsgRedemptionCategoryMissing)
	case errors.Is(err, model.ErrRedemptionCategoryName):
		common.ApiErrorI18n(c, i18n.MsgRedemptionCategoryDuplicate)
	case errors.Is(err, model.ErrRedemptionCategoryInUse):
		common.ApiErrorI18n(c, i18n.MsgRedemptionCategoryInUse)
	case errors.Is(err, model.ErrRedemptionAlreadyUsed):
		common.ApiErrorI18n(c, i18n.MsgRedemptionUsed)
	default:
		common.ApiError(c, err)
	}
}

func GetRedemptionCategories(c *gin.Context) {
	categories, err := model.GetRedemptionCategories()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, categories)
}

func SaveRedemptionCategory(c *gin.Context) {
	var req struct {
		Name  string `json:"name"`
		Quota int    `json:"quota"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	category := model.RedemptionCategory{Name: req.Name, Quota: req.Quota}
	if raw := c.Param("id"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			common.ApiErrorI18n(c, i18n.MsgRedemptionCategoryMissing)
			return
		}
		category.Id = id
	}
	if err := category.SaveSettings(); err != nil {
		redemptionAdminError(c, err)
		return
	}
	common.ApiSuccess(c, category)
}

func DeleteRedemptionCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCategoryMissing)
		return
	}
	if err := model.DeleteRedemptionCategory(id); err != nil {
		redemptionAdminError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GetRedemptionBonusRules(c *gin.Context) {
	rules, err := model.GetRedemptionBonusRules()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rules)
}

func SaveRedemptionBonusRules(c *gin.Context) {
	var req struct {
		Rules []model.RedemptionBonusRule `json:"rules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	rules, err := model.ReplaceRedemptionBonusRules(req.Rules)
	if err != nil {
		redemptionAdminError(c, err)
		return
	}
	common.ApiSuccess(c, rules)
}

func redemptionCategoryFilter(c *gin.Context) (*int, bool) {
	raw, ok := c.GetQuery("category_id")
	if !ok {
		return nil, true
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id < 0 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCategoryMissing)
		return nil, false
	}
	return &id, true
}

func SearchRedemptions(c *gin.Context) {
	categoryId, ok := redemptionCategoryFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	redemptions, total, err := model.SearchRedemptions(c.Query("keyword"), c.Query("status"), categoryId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(redemptions)
	common.ApiSuccess(c, pageInfo)
}

func GetRedemption(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	redemption, err := model.GetRedemptionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, redemption)
}

func AddRedemption(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return
	}
	var redemption model.Redemption
	if err := c.ShouldBindJSON(&redemption); err != nil {
		common.ApiError(c, err)
		return
	}
	if redemption.Count <= 0 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountPositive)
		return
	}
	if redemption.Count > 100 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountMax)
		return
	}
	if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
		common.ApiErrorI18n(c, i18n.MsgRedemptionExpireTimeInvalid)
		return
	}
	redemption.UserId = c.GetInt("id")
	keys, err := redemption.IssueBatch()
	if err != nil {
		redemptionAdminError(c, err)
		return
	}
	recordManageAudit(c, "redemption.create", map[string]interface{}{
		"name": redemption.Name, "count": len(keys), "quota": logger.LogQuota(redemption.Quota), "category_id": redemption.CategoryId,
	})
	common.ApiSuccess(c, keys)
}

func DeleteRedemption(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteRedemptionById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func UpdateRedemption(c *gin.Context) {
	var req struct {
		Id          int    `json:"id"`
		Name        string `json:"name"`
		Quota       int    `json:"quota"`
		CategoryId  *int   `json:"category_id"`
		Status      int    `json:"status"`
		ExpiredTime int64  `json:"expired_time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	var status *int
	if c.Query("status_only") != "" {
		status = &req.Status
	} else if req.ExpiredTime != 0 && req.ExpiredTime < common.GetTimestamp() {
		common.ApiErrorI18n(c, i18n.MsgRedemptionExpireTimeInvalid)
		return
	}
	redemption, err := model.UpdateRedemptionDetails(req.Id, req.CategoryId, req.Name, req.Quota, req.ExpiredTime, status)
	if err != nil {
		redemptionAdminError(c, err)
		return
	}
	common.ApiSuccess(c, redemption)
}

func DeleteInvalidRedemption(c *gin.Context) {
	categoryId, ok := redemptionCategoryFilter(c)
	if !ok {
		return
	}
	rows, err := model.DeleteInvalidRedemptions(categoryId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

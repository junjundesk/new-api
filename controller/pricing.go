package controller

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetPricing(c *gin.Context) {
	userId := c.GetInt("id")
	var group string
	if userId > 0 {
		user, err := model.GetUserCache(userId)
		if err == nil {
			group = user.Group
		}
	}

	// The model square advertises standard group pricing only. Activity-code
	// grants are personal billing discounts, so they must not leak into either
	// the visible group list or the displayed prices here.
	usableGroup := service.GetUserUsableGroups(group)
	pricing := model.GetPricingForUser(group, usableGroup)
	groupRatio := make(map[string]float64)
	for usingGroup := range ratio_setting.GetGroupRatioCopy() {
		if _, visible := usableGroup[usingGroup]; visible {
			groupRatio[usingGroup] = service.GetUserGroupRatio(group, usingGroup)
		}
	}

	// Plaza-only, per-group model descriptions. Only groups the caller can see
	// are exposed so a private group's copy never leaks through this endpoint.
	groupModelDescriptions := make(map[string]map[string]string)
	for usingGroup := range usableGroup {
		if descriptions := ratio_setting.GetGroupModelDescriptions(usingGroup); len(descriptions) > 0 {
			groupModelDescriptions[usingGroup] = descriptions
		}
	}

	c.JSON(200, gin.H{
		"success":                  true,
		"data":                     pricing,
		"vendors":                  model.GetVendors(),
		"group_ratio":              groupRatio,
		"usable_group":             usableGroup,
		"group_model_descriptions": groupModelDescriptions,
		"supported_endpoint":       model.GetSupportedEndpointMap(),
		"auto_groups":              service.GetUserAutoGroup(group),
		"pricing_version":          "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}

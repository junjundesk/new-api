/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"errors"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func GetRevenueReport(c *gin.Context) {
	report, err := model.GetRevenueReport(c.Request.Context(), c.DefaultQuery("range", "today"), time.Now())
	if errors.Is(err, model.ErrRevenueRange) {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}

func GetRevenueCostGroups(c *gin.Context) {
	settings, err := model.GetRevenueCostSettings(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	choices := []string{}
	for g := range ratio_setting.GetGroupRatioCopy() {
		choices = append(choices, g)
	}
	sort.Strings(choices)
	common.ApiSuccess(c, gin.H{"groups": settings.Groups, "available_groups": choices})
}

func SaveRevenueCostGroups(c *gin.Context) {
	var settings model.RevenueCostSettings
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, model.MaxRevenueSettingsBytes))
	if err != nil || common.Unmarshal(raw, &settings) != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := model.SaveRevenueCostSettings(c.Request.Context(), settings); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, settings)
}

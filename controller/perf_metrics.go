package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

func GetPerfMetricsSummary(c *gin.Context) {
	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	activeGroups := append(lo.Keys(ratio_setting.GetGroupRatioCopy()), "auto")
	result, err := perfmetrics.QuerySummaryAll(hours, activeGroups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func GetPerfMetrics(c *gin.Context) {
	modelName := c.Query("model")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "model is required",
		})
		return
	}

	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	result, err := perfmetrics.Query(perfmetrics.QueryParams{
		Model: modelName,
		Group: c.Query("group"),
		Hours: hours,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result.Groups = filterActiveGroups(result.Groups)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetPerfMetricsGroups returns one aggregate per group that the current
// account can use. The browser can render the complete performance page from
// this single request, while the model list inside each group supports the
// drill-down view without exposing inactive groups.
func GetPerfMetricsGroups(c *gin.Context) {
	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	metadata := getPerformanceGroupMetadata(c)
	var result perfmetrics.GroupsResult
	var err error
	if c.GetInt("role") >= common.RoleAdminUser {
		result, err = perfmetrics.QueryGroupsAll(hours, metadata)
	} else {
		result, err = perfmetrics.QueryGroups(hours, metadata)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func getPerformanceGroupMetadata(c *gin.Context) map[string]perfmetrics.GroupMetadata {
	if c.GetInt("role") >= common.RoleAdminUser {
		return getAllPerformanceGroupMetadata()
	}

	userID := c.GetInt("id")
	if userID > 0 {
		userGroup, _ := model.GetUserGroup(userID, false)
		return getUserPerformanceGroupMetadata(userGroup)
	}
	return getPublicPerformanceGroupMetadata()
}

func getUserPerformanceGroupMetadata(userGroup string) map[string]perfmetrics.GroupMetadata {
	metadata := make(map[string]perfmetrics.GroupMetadata)
	usableGroups := service.GetUserUsableGroups(userGroup)
	for group, description := range usableGroups {
		if group == "auto" {
			metadata[group] = perfmetrics.GroupMetadata{
				Description: setting.GetUsableGroupDescription(group),
				Ratio:       1,
			}
			continue
		}
		if !ratio_setting.ContainsGroupRatio(group) {
			continue
		}
		metadata[group] = perfmetrics.GroupMetadata{
			Description: description,
			Ratio:       service.GetUserGroupRatio(userGroup, group),
		}
	}
	return metadata
}

func getPublicPerformanceGroupMetadata() map[string]perfmetrics.GroupMetadata {
	metadata := make(map[string]perfmetrics.GroupMetadata)
	for group, ratio := range ratio_setting.GetGroupRatioCopy() {
		metadata[group] = perfmetrics.GroupMetadata{
			Description: setting.GetUsableGroupDescription(group),
			Ratio:       ratio,
		}
	}

	// The automatic group is available to users even when it does not have a
	// direct ratio entry.
	if _, ok := metadata["auto"]; !ok {
		metadata["auto"] = perfmetrics.GroupMetadata{
			Description: setting.GetUsableGroupDescription("auto"),
			Ratio:       1,
		}
	}
	return metadata
}

func getAllPerformanceGroupMetadata() map[string]perfmetrics.GroupMetadata {
	metadata := make(map[string]perfmetrics.GroupMetadata)
	for group, ratio := range ratio_setting.GetGroupRatioCopy() {
		metadata[group] = perfmetrics.GroupMetadata{
			Description: setting.GetUsableGroupDescription(group),
			Ratio:       ratio,
		}
	}

	// Keep disabled groups visible to administrators when they remain in the
	// configured usable-group list. A zero ratio marks that the group is not
	// currently enabled without exposing it to ordinary users.
	for group, description := range setting.GetUserUsableGroupsCopy() {
		if _, ok := metadata[group]; ok || group == "auto" {
			continue
		}
		metadata[group] = perfmetrics.GroupMetadata{
			Description: description,
			Ratio:       0,
		}
	}

	metadata["auto"] = perfmetrics.GroupMetadata{
		Description: setting.GetUsableGroupDescription("auto"),
		Ratio:       1,
	}
	return metadata
}

func filterActiveGroups(groups []perfmetrics.GroupResult) []perfmetrics.GroupResult {
	activeRatios := ratio_setting.GetGroupRatioCopy()
	return lo.Filter(groups, func(g perfmetrics.GroupResult, _ int) bool {
		_, ok := activeRatios[g.Group]
		return ok || g.Group == "auto"
	})
}

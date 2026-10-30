package ratio_setting

import (
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"vip": {
		"edit_this": 0.9,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{}

var groupModelDescriptions = types.NewRWMap[string, map[string]string]()

const (
	maxGroupModelDescriptionLength = 1000
	maxGroupModelDescriptionsSize  = 512 * 1024
)

type GroupRatioSetting struct {
	GroupRatio              *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio         *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupSpecialUsableGroup *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
	// GroupModelDescriptions stores plaza-only model descriptions per group:
	// {"<group>": {"<model name>": "<description>"}}. It never affects billing.
	GroupModelDescriptions *types.RWMap[string, map[string]string] `json:"group_model_descriptions"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup: groupSpecialUsableGroup,
		GroupRatio:              groupRatioMap,
		GroupGroupRatio:         groupGroupRatioMap,
		GroupModelDescriptions:  groupModelDescriptions,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	if groupRatioSetting.GroupModelDescriptions == nil {
		groupRatioSetting.GroupModelDescriptions = groupModelDescriptions
	}
	return &groupRatioSetting
}

// GetGroupModelDescriptions returns a copy of the model descriptions configured
// for one group in the model plaza. Blank model names and descriptions are
// treated as not configured.
func GetGroupModelDescriptions(group string) map[string]string {
	descriptions, ok := groupModelDescriptions.Get(group)
	if !ok {
		return nil
	}
	copied := make(map[string]string, len(descriptions))
	for modelName, description := range descriptions {
		if strings.TrimSpace(modelName) == "" || strings.TrimSpace(description) == "" {
			continue
		}
		copied[modelName] = description
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

// CheckGroupModelDescriptions validates the option value shape:
// {"<group>": {"<model name>": "<description>"}}.
func CheckGroupModelDescriptions(jsonStr string) error {
	if len(jsonStr) > maxGroupModelDescriptionsSize {
		return errors.New("分组模型描述内容过大")
	}
	descriptions := make(map[string]map[string]string)
	if err := common.UnmarshalJsonStr(jsonStr, &descriptions); err != nil {
		return errors.New("分组模型描述必须是 {\"分组\": {\"模型\": \"描述\"}} 形式的 JSON 对象")
	}
	for group, entries := range descriptions {
		if strings.TrimSpace(group) == "" {
			return errors.New("分组名称不能为空")
		}
		for modelName, description := range entries {
			if strings.TrimSpace(modelName) == "" {
				return errors.New("模型名称不能为空: " + group)
			}
			if len(description) > maxGroupModelDescriptionLength {
				return errors.New("模型描述过长（最多 1000 字符）: " + group + " / " + modelName)
			}
		}
	}
	return nil
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

// ActivityRatioOverride is an activity-code grant's ratio overrides for one group.
// ModelRatios covers the named billing models and outranks GroupRatio, which covers
// every other model in the group. Both empty means the grant inherits the group
// pricing; the non-nil value still marks an active grant, which outranks a
// channel-model override.
type ActivityRatioOverride struct {
	ModelRatios map[string]float64
	GroupRatio  *float64
	// Token 套餐：免费价来自需要计量的授权时，记录本次请求应扣减的授权 id。
	// 不限量免费权益不设置这两个字段，因此不会扣减任何套餐。
	GroupTokenGrantId  int
	ModelTokenGrantIds map[string]int
}

// IsValidModelGroupRatio reports whether a channel model-specific billing
// multiplier is usable: a finite number inside [0, MaxChannelModelGroupRatio].
// Values loaded from a hand-edited row are ignored instead of poisoning the
// charge.
func IsValidModelGroupRatio(ratio *float64) bool {
	return ratio != nil && !math.IsNaN(*ratio) && !math.IsInf(*ratio, 0) &&
		*ratio >= 0 && *ratio <= dto.MaxChannelModelGroupRatio
}

// ResolveModelGroupRatio resolves the final billing multiplier, independently of
// routing and authorization. modelName is the exact token-mapped, pre-channel-
// mapping billing name. The returned override is copied to freeze its value.
//
// An active activity-code grant is the highest priority: when activity is
// non-nil, its model-specific ratio (or group-level ratio, or the group's own
// ratio for an inherit-mode grant) is the charge, and a channel-model override
// is ignored. Without a grant, a channel-model override outranks the user-group
// matrix, which outranks the group default.
func ResolveModelGroupRatio(userGroup, usingGroup, modelName string, overrides map[string]*float64, activity *ActivityRatioOverride) types.GroupRatioInfo {
	info := types.GroupRatioInfo{GroupSpecialRatio: -1, Source: types.GroupRatioSourceGroup}

	var channelRatio *float64
	if ratio, ok := overrides[modelName]; ok && IsValidModelGroupRatio(ratio) {
		value := *ratio
		channelRatio = &value
	}

	var activityRatio *float64
	activityModelRatio := false
	if activity != nil {
		if ratio, ok := activity.ModelRatios[modelName]; ok && IsValidModelGroupRatio(&ratio) {
			activityRatio = &ratio
			activityModelRatio = true
		} else if IsValidModelGroupRatio(activity.GroupRatio) {
			activityRatio = activity.GroupRatio
		}
	}

	if activity == nil && channelRatio != nil {
		value := *channelRatio
		info.GroupRatio = value
		info.ChannelModelRatio = &value
		info.Source = types.GroupRatioSourceChannelModel
		return info
	}

	var baseRatio float64
	baseSpecial := false
	if ratio, ok := GetGroupGroupRatio(userGroup, usingGroup); ok {
		baseRatio = ratio
		baseSpecial = true
	} else {
		baseRatio = GetGroupRatio(usingGroup)
	}

	if activity != nil {
		value := baseRatio
		if activityRatio != nil {
			value = *activityRatio
		}
		info.GroupRatio = value
		info.GroupSpecialRatio = value
		info.HasSpecialRatio = true
		info.Source = types.GroupRatioSourceActivity
		// Token 套餐：记录本次请求应扣减的授权，结算时按实际 token 累加。
		if activityModelRatio {
			if grantId, ok := activity.ModelTokenGrantIds[modelName]; ok {
				info.ActivityGrantId = grantId
			}
		} else if activity.GroupTokenGrantId != 0 {
			info.ActivityGrantId = activity.GroupTokenGrantId
		}
		// Freeze the price the request would have had without the grant, so the
		// consume log can show it struck through next to the discounted charge.
		if channelRatio != nil {
			original := *channelRatio
			info.ActivityOriginalRatio = &original
		} else if activityRatio != nil {
			original := baseRatio
			info.ActivityOriginalRatio = &original
		}
		return info
	}

	info.GroupRatio = baseRatio
	if baseSpecial {
		info.GroupSpecialRatio = baseRatio
		info.HasSpecialRatio = true
		info.Source = types.GroupRatioSourceUserGroup
	}
	return info
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}

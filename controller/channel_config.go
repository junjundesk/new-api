package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	channelConfigVersion    = 1
	maxChannelConfigEntries = 100
	maxChannelConfigBytes   = 8 << 20
)

type channelConfig struct {
	Version  int                  `json:"version"`
	Channels []channelConfigEntry `json:"channels"`
}

// Only portable settings belong here. IDs, balances, usage, test history and
// polling cursors are owned by the destination, never by an import document.
type channelConfigEntry struct {
	Name               string                `json:"name"`
	Type               int                   `json:"type"`
	Key                string                `json:"key"`
	Models             string                `json:"models"`
	Group              string                `json:"group"`
	Status             int                   `json:"status,omitempty"`
	BaseURL            *string               `json:"base_url,omitempty"`
	BaseURLs           []string              `json:"base_urls,omitempty"`
	OpenAIOrganization *string               `json:"openai_organization,omitempty"`
	TestModel          *string               `json:"test_model,omitempty"`
	Priority           *int64                `json:"priority,omitempty"`
	Weight             *uint                 `json:"weight,omitempty"`
	AutoBan            *int                  `json:"auto_ban,omitempty"`
	Tag                *string               `json:"tag,omitempty"`
	Remark             *string               `json:"remark,omitempty"`
	ModelMapping       *string               `json:"model_mapping,omitempty"`
	StatusCodeMapping  *string               `json:"status_code_mapping,omitempty"`
	ParamOverride      *string               `json:"param_override,omitempty"`
	HeaderOverride     *string               `json:"header_override,omitempty"`
	Other              string                `json:"other,omitempty"`
	Setting            *string               `json:"setting,omitempty"`
	Settings           string                `json:"settings,omitempty"`
	MultiKeyMode       constant.MultiKeyMode `json:"multi_key_mode,omitempty"`
	MultiKeyStatusList map[int]int           `json:"multi_key_status_list,omitempty"`
}

// Keep provider settings (including unknown extension fields), but never replay
// the source instance's pending model additions/removals in the destination.
func portableChannelSettings(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	var settings map[string]json.RawMessage
	if err := common.UnmarshalJsonStr(raw, &settings); err != nil {
		return "", errors.New("settings")
	}
	originalSize := len(settings)
	delete(settings, "upstream_model_update_last_check_time")
	delete(settings, "upstream_model_update_last_detected_models")
	delete(settings, "upstream_model_update_last_removed_models")
	if len(settings) == originalSize {
		return raw, nil
	}
	data, err := common.Marshal(settings)
	return string(data), err
}

func ExportChannelConfig(c *gin.Context) {
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := common.DecodeJsonStrict(http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10), &req); err != nil || len(req.IDs) == 0 || len(req.IDs) > maxChannelConfigEntries {
		common.ApiErrorI18n(c, i18n.MsgChannelConfigSelection, map[string]any{"Max": maxChannelConfigEntries})
		return
	}
	channels, err := model.GetChannelsByIds(req.IDs)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelQueryFailed)
		return
	}
	// IN returns unique rows. Fail the entire export for stale or repeated IDs
	// instead of handing the operator an incomplete backup.
	if len(channels) != len(req.IDs) {
		common.ApiErrorI18n(c, i18n.MsgChannelConfigSelection, map[string]any{"Max": maxChannelConfigEntries})
		return
	}
	byID := make(map[int]*model.Channel, len(channels))
	for _, ch := range channels {
		byID[ch.Id] = ch
	}
	config := channelConfig{Version: channelConfigVersion, Channels: make([]channelConfigEntry, 0, len(channels))}
	for i, id := range req.IDs {
		ch := byID[id]
		settings, err := portableChannelSettings(ch.OtherSettings)
		if err != nil {
			common.ApiErrorI18n(c, i18n.MsgChannelConfigEntryInvalid, map[string]any{"Index": i + 1, "Detail": err.Error()})
			return
		}
		entry := channelConfigEntry{
			Name: ch.Name, Type: ch.Type, Key: ch.Key, Models: ch.Models, Group: ch.Group, Status: ch.Status,
			BaseURL: ch.BaseURL, BaseURLs: ch.BaseURLs, OpenAIOrganization: ch.OpenAIOrganization, TestModel: ch.TestModel,
			Priority: ch.Priority, Weight: ch.Weight, AutoBan: ch.AutoBan, Tag: ch.Tag, Remark: ch.Remark,
			ModelMapping: ch.ModelMapping, StatusCodeMapping: ch.StatusCodeMapping, ParamOverride: ch.ParamOverride,
			HeaderOverride: ch.HeaderOverride, Other: ch.Other, Setting: ch.Setting, Settings: settings,
		}
		if ch.ChannelInfo.IsMultiKey {
			entry.MultiKeyMode = ch.ChannelInfo.MultiKeyMode
			if entry.MultiKeyMode == "" {
				entry.MultiKeyMode = constant.MultiKeyModeRandom
			}
			entry.MultiKeyStatusList = ch.ChannelInfo.MultiKeyStatusList
		}
		config.Channels = append(config.Channels, entry)
	}
	data, err := common.Marshal(config)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelQueryFailed)
		return
	}
	if len(data) > maxChannelConfigBytes {
		common.ApiErrorI18n(c, i18n.MsgChannelConfigTooLarge)
		return
	}
	recordManageAudit(c, "channel.export", map[string]any{"ids": req.IDs, "count": len(channels)})
	common.ApiSuccess(c, json.RawMessage(data))
}

func (entry *channelConfigEntry) newChannel() (*model.Channel, error) {
	if _, ok := constant.ChannelTypeNames[entry.Type]; !ok || entry.Type == constant.ChannelTypeUnknown {
		return nil, errors.New("type")
	}
	if strings.TrimSpace(entry.Name) == "" {
		return nil, errors.New("name")
	}
	if strings.TrimSpace(entry.Key) == "" {
		return nil, errors.New("key")
	}
	for _, name := range strings.Split(entry.Models, ",") {
		if strings.TrimSpace(name) == "" {
			return nil, errors.New("models")
		}
	}
	if entry.Group == "" {
		entry.Group = "default"
	}
	if utf8.RuneCountInString(entry.Group) > 64 {
		return nil, errors.New("group")
	}
	for _, group := range strings.Split(entry.Group, ",") {
		if strings.TrimSpace(group) == "" {
			return nil, errors.New("group")
		}
	}
	if entry.Status != 0 && !isManageableChannelStatus(entry.Status) && entry.Status != common.ChannelStatusAutoDisabled {
		return nil, errors.New("status")
	}
	for field, value := range map[string]*string{"header_override": entry.HeaderOverride, "param_override": entry.ParamOverride, "status_code_mapping": entry.StatusCodeMapping} {
		if value == nil || *value == "" {
			continue
		}
		var object map[string]json.RawMessage
		if err := common.UnmarshalJsonStr(*value, &object); err != nil || object == nil {
			return nil, errors.New(field)
		}
	}
	settings, err := portableChannelSettings(entry.Settings)
	if err != nil {
		return nil, err
	}
	ch := &model.Channel{
		Name: entry.Name, Type: entry.Type, Key: entry.Key, Models: entry.Models, Group: entry.Group, Status: entry.Status,
		OpenAIOrganization: entry.OpenAIOrganization, TestModel: entry.TestModel,
		Priority: entry.Priority, Weight: entry.Weight, AutoBan: entry.AutoBan, Tag: entry.Tag, Remark: entry.Remark,
		ModelMapping: entry.ModelMapping, StatusCodeMapping: entry.StatusCodeMapping, ParamOverride: entry.ParamOverride,
		HeaderOverride: entry.HeaderOverride, Other: entry.Other, Setting: entry.Setting, OtherSettings: settings,
		CreatedTime: common.GetTimestamp(),
	}
	if err := applyChannelBaseURLs(ch, entry.BaseURLs, entry.BaseURL); err != nil {
		return nil, errors.New("base_url / base_urls")
	}
	if err := validateChannel(ch, true); err != nil {
		return nil, err
	}
	if entry.MultiKeyMode == "" && len(entry.MultiKeyStatusList) == 0 {
		return ch, nil
	}
	if entry.MultiKeyMode != constant.MultiKeyModeRandom && entry.MultiKeyMode != constant.MultiKeyModePolling {
		return nil, errors.New("multi_key_mode")
	}
	keys := ch.GetKeys()
	if len(keys) == 0 {
		return nil, errors.New("key")
	}
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			return nil, errors.New("key")
		}
	}
	for index, status := range entry.MultiKeyStatusList {
		if index < 0 || index >= len(keys) || (!isManageableChannelStatus(status) && status != common.ChannelStatusAutoDisabled) {
			return nil, errors.New("multi_key_status_list")
		}
	}
	ch.ChannelInfo = model.ChannelInfo{IsMultiKey: true, MultiKeySize: len(keys), MultiKeyMode: entry.MultiKeyMode, MultiKeyStatusList: entry.MultiKeyStatusList}
	return ch, nil
}

func ImportChannelConfig(c *gin.Context) {
	var config channelConfig
	err := common.DecodeJsonStrict(http.MaxBytesReader(c.Writer, c.Request.Body, maxChannelConfigBytes), &config)
	if common.IsRequestBodyTooLargeError(err) {
		common.ApiErrorI18n(c, i18n.MsgChannelConfigTooLarge)
		return
	}
	if err != nil || config.Version != channelConfigVersion || len(config.Channels) == 0 || len(config.Channels) > maxChannelConfigEntries {
		common.ApiErrorI18n(c, i18n.MsgChannelConfigInvalid, map[string]any{"Max": maxChannelConfigEntries})
		return
	}
	channels := make([]model.Channel, 0, len(config.Channels))
	for i := range config.Channels {
		ch, err := config.Channels[i].newChannel()
		if err != nil {
			common.ApiErrorI18n(c, i18n.MsgChannelConfigEntryInvalid, map[string]any{"Index": i + 1, "Detail": err.Error()})
			return
		}
		channels = append(channels, *ch)
	}
	if err := model.BatchInsertChannels(channels); err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelConfigImportFailed)
		return
	}
	model.InitChannelCache()
	recordManageAudit(c, "channel.import", map[string]any{"count": len(channels)})
	common.ApiSuccess(c, len(channels))
}

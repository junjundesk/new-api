package setting

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const (
	MaxTokenModelMappings            = 100
	MaxTokenModelMappingPresets      = 100
	MaxTokenModelNameLength          = 255
	MaxTokenReasoningEffortBytes     = 64
	MaxTokenModelMappingPresetName   = 100
	MaxTokenModelMappingPresetDesc   = 500
	MaxTokenModelMappingPresetIdSize = 64
)

// TokenModelMapping routes one exact client model name to another model on a
// single API key, optionally overriding the reasoning effort sent upstream.
type TokenModelMapping struct {
	SourceModel     string `json:"source_model"`
	TargetModel     string `json:"target_model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// TokenModelMappingPreset is an admin-defined template that users copy into a
// key. Saved keys keep their own copy and never follow later preset edits.
type TokenModelMappingPreset struct {
	Id          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Recommended bool                `json:"recommended"`
	Mappings    []TokenModelMapping `json:"mappings"`
}

var (
	tokenReasoningEffortPattern = regexp.MustCompile(`^[A-Za-z0-9._-]*$`)
	tokenPresetIdPattern        = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

	tokenModelMappingPresets      = []TokenModelMappingPreset{}
	tokenModelMappingPresetsMutex sync.RWMutex
)

func isValidTokenModelName(name string) bool {
	if utf8.RuneCountInString(name) > MaxTokenModelNameLength {
		return false
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// NormalizeTokenModelMappings trims every field and validates the list with the
// same rules as the API key drawer. The returned slice is safe to persist.
func NormalizeTokenModelMappings(mappings []TokenModelMapping) ([]TokenModelMapping, error) {
	if len(mappings) > MaxTokenModelMappings {
		return nil, fmt.Errorf("use at most %d model mappings", MaxTokenModelMappings)
	}
	normalized := make([]TokenModelMapping, 0, len(mappings))
	seen := make(map[string]struct{}, len(mappings))
	for i, mapping := range mappings {
		item := TokenModelMapping{
			SourceModel:     strings.TrimSpace(mapping.SourceModel),
			TargetModel:     strings.TrimSpace(mapping.TargetModel),
			ReasoningEffort: strings.TrimSpace(mapping.ReasoningEffort),
		}
		if item.SourceModel == "" {
			return nil, fmt.Errorf("mapping %d: source model is required", i+1)
		}
		if item.TargetModel == "" {
			return nil, fmt.Errorf("mapping %d: target model is required", i+1)
		}
		if !isValidTokenModelName(item.SourceModel) || !isValidTokenModelName(item.TargetModel) {
			return nil, fmt.Errorf("mapping %d: model names must be at most %d characters and contain no spaces or control characters", i+1, MaxTokenModelNameLength)
		}
		if len(item.ReasoningEffort) > MaxTokenReasoningEffortBytes || !tokenReasoningEffortPattern.MatchString(item.ReasoningEffort) {
			return nil, fmt.Errorf("mapping %d: reasoning effort must be up to %d letters, numbers, dots, underscores or hyphens", i+1, MaxTokenReasoningEffortBytes)
		}
		if _, ok := seen[item.SourceModel]; ok {
			return nil, fmt.Errorf("mapping %d: source model %s is duplicated", i+1, item.SourceModel)
		}
		seen[item.SourceModel] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func parseTokenModelMappingPresets(jsonStr string) ([]TokenModelMappingPreset, error) {
	presets := make([]TokenModelMappingPreset, 0)
	if strings.TrimSpace(jsonStr) == "" {
		return presets, nil
	}
	if err := common.UnmarshalJsonStr(jsonStr, &presets); err != nil {
		return nil, err
	}
	if len(presets) > MaxTokenModelMappingPresets {
		return nil, fmt.Errorf("use at most %d presets", MaxTokenModelMappingPresets)
	}
	seen := make(map[string]struct{}, len(presets))
	for i := range presets {
		preset := &presets[i]
		preset.Id = strings.TrimSpace(preset.Id)
		preset.Name = strings.TrimSpace(preset.Name)
		preset.Description = strings.TrimSpace(preset.Description)
		if preset.Id == "" || len(preset.Id) > MaxTokenModelMappingPresetIdSize || !tokenPresetIdPattern.MatchString(preset.Id) {
			return nil, fmt.Errorf("preset %d: invalid id", i+1)
		}
		if _, ok := seen[preset.Id]; ok {
			return nil, fmt.Errorf("preset %d: id %s is duplicated", i+1, preset.Id)
		}
		seen[preset.Id] = struct{}{}
		if preset.Name == "" || utf8.RuneCountInString(preset.Name) > MaxTokenModelMappingPresetName {
			return nil, fmt.Errorf("preset %d: name must be 1-%d characters", i+1, MaxTokenModelMappingPresetName)
		}
		if utf8.RuneCountInString(preset.Description) > MaxTokenModelMappingPresetDesc {
			return nil, fmt.Errorf("preset %d: description must be at most %d characters", i+1, MaxTokenModelMappingPresetDesc)
		}
		if len(preset.Mappings) == 0 {
			return nil, fmt.Errorf("preset %d: add at least one mapping", i+1)
		}
		mappings, err := NormalizeTokenModelMappings(preset.Mappings)
		if err != nil {
			return nil, fmt.Errorf("preset %d: %w", i+1, err)
		}
		preset.Mappings = mappings
	}
	return presets, nil
}

func CheckTokenModelMappingPresets(jsonStr string) error {
	_, err := parseTokenModelMappingPresets(jsonStr)
	return err
}

func UpdateTokenModelMappingPresetsByJSONString(jsonStr string) error {
	presets, err := parseTokenModelMappingPresets(jsonStr)
	if err != nil {
		return err
	}
	tokenModelMappingPresetsMutex.Lock()
	defer tokenModelMappingPresetsMutex.Unlock()
	tokenModelMappingPresets = presets
	return nil
}

func TokenModelMappingPresets2JSONString() string {
	tokenModelMappingPresetsMutex.RLock()
	defer tokenModelMappingPresetsMutex.RUnlock()
	data, err := common.Marshal(tokenModelMappingPresets)
	if err != nil {
		common.SysError("failed to marshal token model mapping presets: " + err.Error())
		return "[]"
	}
	return string(data)
}

func GetTokenModelMappingPresetsCopy() []TokenModelMappingPreset {
	tokenModelMappingPresetsMutex.RLock()
	defer tokenModelMappingPresetsMutex.RUnlock()
	presets := make([]TokenModelMappingPreset, len(tokenModelMappingPresets))
	for i, preset := range tokenModelMappingPresets {
		preset.Mappings = append([]TokenModelMapping(nil), preset.Mappings...)
		presets[i] = preset
	}
	return presets
}

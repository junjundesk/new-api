package setting

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxTokenFallbackModels     = 5
	MaxTokenFallbackNameLength = 255
)

// TokenFallbackModel routes one model to an ordered list of fallback targets.
// SourceModel is optional: an empty value makes the entry apply to every
// requested model, while a non-empty value scopes it to that exact model.
type TokenFallbackModel struct {
	SourceModel string `json:"source_model,omitempty"`
	Group       string `json:"group"`
	Model       string `json:"model"`
}

func isValidTokenFallbackName(name string) bool {
	if utf8.RuneCountInString(name) > MaxTokenFallbackNameLength {
		return false
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// NormalizeDisabledTokenFallbackModels trims and bounds the fallback list kept
// for a key whose fallback is disabled. That list is an inert draft: auth never
// loads it and the edit form keeps unfinished rows, so empty fields are allowed
// and only field shape limits apply. Enabling fallback runs the full validation
// again.
func NormalizeDisabledTokenFallbackModels(items []TokenFallbackModel) ([]TokenFallbackModel, error) {
	if len(items) > MaxTokenFallbackModels {
		return nil, fmt.Errorf("use at most %d fallback models", MaxTokenFallbackModels)
	}
	normalized := make([]TokenFallbackModel, 0, len(items))
	for i, item := range items {
		entry := TokenFallbackModel{
			SourceModel: strings.TrimSpace(item.SourceModel),
			Group:       strings.TrimSpace(item.Group),
			Model:       strings.TrimSpace(item.Model),
		}
		if entry.SourceModel != "" && !isValidTokenFallbackName(entry.SourceModel) {
			return nil, fmt.Errorf("fallback %d: source model is invalid", i+1)
		}
		if entry.Group != "" && !isValidTokenFallbackName(entry.Group) {
			return nil, fmt.Errorf("fallback %d: group is invalid", i+1)
		}
		if entry.Model != "" && !isValidTokenFallbackName(entry.Model) {
			return nil, fmt.Errorf("fallback %d: model is invalid", i+1)
		}
		normalized = append(normalized, entry)
	}
	return normalized, nil
}

// NormalizeTokenFallbackModels trims every field and validates the list shape.
// Permission, group membership and pricing checks belong to the controller:
// this package must not depend on service or relay helpers.
func NormalizeTokenFallbackModels(items []TokenFallbackModel) ([]TokenFallbackModel, error) {
	if len(items) > MaxTokenFallbackModels {
		return nil, fmt.Errorf("use at most %d fallback models", MaxTokenFallbackModels)
	}
	normalized := make([]TokenFallbackModel, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		entry := TokenFallbackModel{
			SourceModel: strings.TrimSpace(item.SourceModel),
			Group:       strings.TrimSpace(item.Group),
			Model:       strings.TrimSpace(item.Model),
		}
		if entry.Group == "" {
			return nil, fmt.Errorf("fallback %d: group is required", i+1)
		}
		if entry.Model == "" {
			return nil, fmt.Errorf("fallback %d: model is required", i+1)
		}
		if entry.SourceModel != "" && !isValidTokenFallbackName(entry.SourceModel) {
			return nil, fmt.Errorf("fallback %d: source model is invalid", i+1)
		}
		if !isValidTokenFallbackName(entry.Group) || !isValidTokenFallbackName(entry.Model) {
			return nil, fmt.Errorf("fallback %d: group or model is invalid", i+1)
		}
		key := entry.SourceModel + "\x00" + entry.Group + "\x00" + entry.Model
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("fallback %d: duplicate fallback target", i+1)
		}
		seen[key] = struct{}{}
		normalized = append(normalized, entry)
	}
	return normalized, nil
}

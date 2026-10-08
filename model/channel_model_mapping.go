package model

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

// Upper bounds for a channel model mapping entry. The candidate list is
// searched in order, so an unbounded list would let one channel spend an
// arbitrary number of upstream requests on a single client call.
const (
	MaxChannelModelMappingCandidates = 32
	MaxChannelModelMappingNameLength = 255
)

// ChannelModelMapping maps a client-facing model name to the ordered list of
// upstream model names to try. A single candidate is the legacy
// `{"model": "upstream"}` form; two or more use the same object shape with a
// JSON array as the value, and the relay then tries them in order when the
// upstream reports the model as unavailable.
type ChannelModelMapping map[string][]string

// ParseChannelModelMapping decodes a stored model_mapping payload. It accepts
// both the legacy string form and the array form. Empty, "{}" and payloads
// without any usable entry yield a nil mapping instead of an error, so a
// channel without a usable mapping behaves as if none was configured.
func ParseChannelModelMapping(raw string) (ChannelModelMapping, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return nil, nil
	}

	rawEntries := make(map[string]any)
	if err := common.UnmarshalJsonStr(trimmed, &rawEntries); err != nil {
		return nil, fmt.Errorf("model_mapping is not a JSON object: %w", err)
	}

	mapping := make(ChannelModelMapping, len(rawEntries))
	for source, rawTarget := range rawEntries {
		normalizedSource := strings.TrimSpace(source)
		if normalizedSource == "" {
			continue
		}
		candidates, err := normalizeModelMappingCandidates(rawTarget)
		if err != nil {
			return nil, fmt.Errorf("model_mapping entry %q: %w", normalizedSource, err)
		}
		if len(candidates) == 0 {
			continue
		}
		mapping[normalizedSource] = candidates
	}
	if len(mapping) == 0 {
		return nil, nil
	}
	return mapping, nil
}

// NormalizeChannelModelMappingString validates a model_mapping payload sent by
// the channel admin UI and returns the canonical JSON string to persist. A
// single candidate is stored as a JSON string so existing tooling keeps
// working, two or more are stored as a JSON array, and an empty mapping is
// stored as "". It rejects model names that could not be sent upstream.
func NormalizeChannelModelMappingString(raw string) (string, error) {
	mapping, err := ParseChannelModelMapping(raw)
	if err != nil {
		return "", err
	}
	if len(mapping) == 0 {
		return "", nil
	}

	payload := make(map[string]any, len(mapping))
	for source, candidates := range mapping {
		if !isValidChannelModelMappingName(source) {
			return "", fmt.Errorf("source model %q must be 1-%d characters without spaces or control characters", source, MaxChannelModelMappingNameLength)
		}
		if len(candidates) > MaxChannelModelMappingCandidates {
			return "", fmt.Errorf("source model %q accepts at most %d candidate models", source, MaxChannelModelMappingCandidates)
		}
		for _, candidate := range candidates {
			if !isValidChannelModelMappingName(candidate) {
				return "", fmt.Errorf("candidate model %q must be 1-%d characters without spaces or control characters", candidate, MaxChannelModelMappingNameLength)
			}
		}
		if len(candidates) == 1 {
			payload[source] = candidates[0]
			continue
		}
		payload[source] = candidates
	}

	data, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// normalizeModelMappingCandidates converts one raw mapping value into a
// trimmed, de-duplicated candidate list.
func normalizeModelMappingCandidates(rawTarget any) ([]string, error) {
	var rawCandidates []string
	switch value := rawTarget.(type) {
	case string:
		rawCandidates = []string{value}
	case []any:
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("candidates must be model names")
			}
			rawCandidates = append(rawCandidates, text)
		}
	default:
		return nil, fmt.Errorf("value must be a model name or an array of model names")
	}

	seen := make(map[string]struct{}, len(rawCandidates))
	candidates := make([]string, 0, len(rawCandidates))
	for _, rawCandidate := range rawCandidates {
		candidate := strings.TrimSpace(rawCandidate)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func isValidChannelModelMappingName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > MaxChannelModelMappingNameLength {
		return false
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseChannelModelMapping(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected ChannelModelMapping
	}{
		{
			name:     "empty payload",
			raw:      "",
			expected: nil,
		},
		{
			name:     "blank payload",
			raw:      "   ",
			expected: nil,
		},
		{
			name:     "empty object",
			raw:      "{}",
			expected: nil,
		},
		{
			name: "legacy single value",
			raw:  `{"gpt-4o":"gpt-4o-2024-08-06"}`,
			expected: ChannelModelMapping{
				"gpt-4o": {"gpt-4o-2024-08-06"},
			},
		},
		{
			name: "array candidates",
			raw:  `{"claude-3-5-sonnet":["claude-3-5-sonnet-20241022","claude-3-5-sonnet-latest"]}`,
			expected: ChannelModelMapping{
				"claude-3-5-sonnet": {"claude-3-5-sonnet-20241022", "claude-3-5-sonnet-latest"},
			},
		},
		{
			name: "mixed single and multiple candidates",
			raw:  `{"a":"upstream-a","b":["upstream-b1","upstream-b2"]}`,
			expected: ChannelModelMapping{
				"a": {"upstream-a"},
				"b": {"upstream-b1", "upstream-b2"},
			},
		},
		{
			name: "trims source and candidates, keeps order, drops duplicates and blanks",
			raw:  `{" alias ":[" target-a ","target-b","target-a",""]}`,
			expected: ChannelModelMapping{
				"alias": {"target-a", "target-b"},
			},
		},
		{
			name:     "entries without usable candidates are dropped",
			raw:      `{"empty":"","blank":["  "],"valid":"upstream"}`,
			expected: ChannelModelMapping{"valid": {"upstream"}},
		},
		{
			name:     "blank source is dropped",
			raw:      `{"":"upstream","valid":"upstream-2"}`,
			expected: ChannelModelMapping{"valid": {"upstream-2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseChannelModelMapping(tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseChannelModelMappingRejectsInvalidPayloads(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "invalid json", raw: `{"a":`},
		{name: "array root", raw: `["a"]`},
		{name: "numeric value", raw: `{"a":1}`},
		{name: "object value", raw: `{"a":{"b":"c"}}`},
		{name: "null value", raw: `{"a":null}`},
		{name: "array with non string item", raw: `{"a":["ok",1]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseChannelModelMapping(tt.raw)
			require.Error(t, err)
		})
	}
}

func TestNormalizeChannelModelMappingString(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		normalized string
	}{
		{
			name:       "single candidate stored as json string",
			raw:        `{"alias":" upstream "}`,
			normalized: `{"alias":"upstream"}`,
		},
		{
			name:       "multiple candidates stored as json array",
			raw:        `{"alias":["a","b"]}`,
			normalized: `{"alias":["a","b"]}`,
		},
		{
			name:       "duplicates collapse to a single value",
			raw:        `{"alias":["a","a"]}`,
			normalized: `{"alias":"a"}`,
		},
		{
			name:       "empty mapping stored as empty string",
			raw:        `{}`,
			normalized: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := NormalizeChannelModelMappingString(tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.normalized, normalized)
		})
	}

	t.Run("normalized payload round trips through the parser", func(t *testing.T) {
		normalized, err := NormalizeChannelModelMappingString(`{"a":"upstream-a","b":["upstream-b1","upstream-b2"]}`)
		require.NoError(t, err)
		parsed, err := ParseChannelModelMapping(normalized)
		require.NoError(t, err)
		require.Equal(t, ChannelModelMapping{
			"a": {"upstream-a"},
			"b": {"upstream-b1", "upstream-b2"},
		}, parsed)
	})
}

func TestNormalizeChannelModelMappingStringRejectsUnusableNames(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "space in candidate", raw: `{"a":"bad model"}`},
		{name: "space in source", raw: `{"bad model":"upstream"}`},
		{name: "too many candidates", raw: `{"a":["1","2","3","4","5","6","7","8","9","10","11","12","13","14","15","16","17","18","19","20","21","22","23","24","25","26","27","28","29","30","31","32","33"]}`},
		{name: "overlong candidate", raw: `{"a":"` + longModelName(MaxChannelModelMappingNameLength+1) + `"}`},
		{name: "invalid json", raw: `{"a":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeChannelModelMappingString(tt.raw)
			require.Error(t, err)
		})
	}
}

func longModelName(length int) string {
	name := make([]byte, length)
	for i := range name {
		name[i] = 'a'
	}
	return string(name)
}

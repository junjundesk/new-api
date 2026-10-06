package setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTokenModelMappings(t *testing.T) {
	tests := []struct {
		name    string
		input   []TokenModelMapping
		want    []TokenModelMapping
		wantErr string
	}{
		{
			name:  "trims fields and drops empty effort",
			input: []TokenModelMapping{{SourceModel: " gpt-6-luna ", TargetModel: " gpt-6-sol ", ReasoningEffort: " "}},
			want:  []TokenModelMapping{{SourceModel: "gpt-6-luna", TargetModel: "gpt-6-sol"}},
		},
		{
			name:  "same-name mapping only overrides effort",
			input: []TokenModelMapping{{SourceModel: "gpt-5.5", TargetModel: "gpt-5.5", ReasoningEffort: "high"}},
			want:  []TokenModelMapping{{SourceModel: "gpt-5.5", TargetModel: "gpt-5.5", ReasoningEffort: "high"}},
		},
		{
			name:    "source required",
			input:   []TokenModelMapping{{SourceModel: " ", TargetModel: "gpt-6-sol"}},
			wantErr: "source model is required",
		},
		{
			name:    "target required",
			input:   []TokenModelMapping{{SourceModel: "a", TargetModel: ""}},
			wantErr: "target model is required",
		},
		{
			name:    "duplicate source rejected",
			input:   []TokenModelMapping{{SourceModel: "a", TargetModel: "b"}, {SourceModel: "a", TargetModel: "c"}},
			wantErr: "duplicated",
		},
		{
			name:    "whitespace inside model name rejected",
			input:   []TokenModelMapping{{SourceModel: "gpt 6", TargetModel: "b"}},
			wantErr: "no spaces or control characters",
		},
		{
			name:    "model name longer than 255 characters rejected",
			input:   []TokenModelMapping{{SourceModel: strings.Repeat("a", 256), TargetModel: "b"}},
			wantErr: "no spaces or control characters",
		},
		{
			name:    "effort with invalid characters rejected",
			input:   []TokenModelMapping{{SourceModel: "a", TargetModel: "b", ReasoningEffort: "hi gh"}},
			wantErr: "reasoning effort",
		},
		{
			name:    "effort longer than 64 bytes rejected",
			input:   []TokenModelMapping{{SourceModel: "a", TargetModel: "b", ReasoningEffort: strings.Repeat("x", 65)}},
			wantErr: "reasoning effort",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTokenModelMappings(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeTokenModelMappingsLimit(t *testing.T) {
	mappings := make([]TokenModelMapping, MaxTokenModelMappings+1)
	for i := range mappings {
		mappings[i] = TokenModelMapping{SourceModel: "s" + strings.Repeat("x", i), TargetModel: "t"}
	}
	_, err := NormalizeTokenModelMappings(mappings)
	require.Error(t, err)
}

func TestCheckTokenModelMappingPresets(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty string is allowed", input: ""},
		{name: "empty list is allowed", input: "[]"},
		{name: "valid preset", input: `[{"id":"codex-luna","name":"Codex / Luna","recommended":true,"mappings":[{"source_model":"gpt-6-luna","target_model":"gpt-6-sol","reasoning_effort":"low"}]}]`},
		{name: "invalid json", input: "{", wantErr: true},
		{name: "missing name", input: `[{"id":"a","name":" ","recommended":false,"mappings":[{"source_model":"a","target_model":"b"}]}]`, wantErr: true},
		{name: "invalid id", input: `[{"id":"a b","name":"x","recommended":false,"mappings":[{"source_model":"a","target_model":"b"}]}]`, wantErr: true},
		{name: "duplicate id", input: `[{"id":"a","name":"x","recommended":false,"mappings":[{"source_model":"a","target_model":"b"}]},{"id":"a","name":"y","recommended":false,"mappings":[{"source_model":"a","target_model":"b"}]}]`, wantErr: true},
		{name: "no mappings", input: `[{"id":"a","name":"x","recommended":false,"mappings":[]}]`, wantErr: true},
		{name: "invalid mapping", input: `[{"id":"a","name":"x","recommended":false,"mappings":[{"source_model":"a","target_model":""}]}]`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckTokenModelMappingPresets(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

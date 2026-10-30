package setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTokenFallbackModelsTrimsAndPreservesOrder(t *testing.T) {
	input := []TokenFallbackModel{
		{SourceModel: " gpt-5 ", Group: " vip ", Model: " claude-sonnet-4-5 "},
		{SourceModel: "   ", Group: "default", Model: "gpt-4.1-mini"},
		{SourceModel: "gpt-5", Group: "azure", Model: "gpt-4.1-mini"},
	}

	got, err := NormalizeTokenFallbackModels(input)
	require.NoError(t, err)
	assert.Equal(t, []TokenFallbackModel{
		{SourceModel: "gpt-5", Group: "vip", Model: "claude-sonnet-4-5"},
		{Group: "default", Model: "gpt-4.1-mini"},
		{SourceModel: "gpt-5", Group: "azure", Model: "gpt-4.1-mini"},
	}, got)
	assert.Equal(t, " gpt-5 ", input[0].SourceModel, "normalization must not mutate the caller's configuration")
}

func TestNormalizeTokenFallbackModelsRejectsInvalidStructure(t *testing.T) {
	valid := TokenFallbackModel{SourceModel: "gpt-5", Group: "vip", Model: "claude-sonnet-4-5"}
	tests := []struct {
		name    string
		mutate  func(*TokenFallbackModel)
		wantErr string
	}{
		{name: "group required", mutate: func(item *TokenFallbackModel) { item.Group = " 	 " }, wantErr: "group is required"},
		{name: "model required", mutate: func(item *TokenFallbackModel) { item.Model = " \n " }, wantErr: "model is required"},
		{name: "source contains whitespace", mutate: func(item *TokenFallbackModel) { item.SourceModel = "gpt 5" }, wantErr: "source model is invalid"},
		{name: "source contains control character", mutate: func(item *TokenFallbackModel) { item.SourceModel = "gpt\t5" }, wantErr: "source model is invalid"},
		{name: "source too long", mutate: func(item *TokenFallbackModel) { item.SourceModel = strings.Repeat("模", MaxTokenFallbackNameLength+1) }, wantErr: "source model is invalid"},
		{name: "group contains whitespace", mutate: func(item *TokenFallbackModel) { item.Group = "v ip" }, wantErr: "group or model is invalid"},
		{name: "model contains newline", mutate: func(item *TokenFallbackModel) { item.Model = "claude\nsonnet" }, wantErr: "group or model is invalid"},
		{name: "target too long", mutate: func(item *TokenFallbackModel) { item.Model = strings.Repeat("m", MaxTokenFallbackNameLength+1) }, wantErr: "group or model is invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := valid
			tt.mutate(&item)
			got, err := NormalizeTokenFallbackModels([]TokenFallbackModel{item})
			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNormalizeTokenFallbackModelsRejectsDuplicateNormalizedTargets(t *testing.T) {
	got, err := NormalizeTokenFallbackModels([]TokenFallbackModel{
		{SourceModel: "gpt-5", Group: "vip", Model: "claude-sonnet-4-5"},
		{SourceModel: " gpt-5 ", Group: " vip ", Model: " claude-sonnet-4-5 "},
	})
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "duplicate fallback target")
}

func TestNormalizeTokenFallbackModelsEnforcesListLimit(t *testing.T) {
	items := make([]TokenFallbackModel, MaxTokenFallbackModels)
	for i := range items {
		items[i] = TokenFallbackModel{Group: "vip", Model: strings.Repeat("m", i+1)}
	}
	got, err := NormalizeTokenFallbackModels(items)
	require.NoError(t, err)
	assert.Len(t, got, MaxTokenFallbackModels)

	items = append(items, TokenFallbackModel{Group: "vip", Model: "one-target-too-many"})
	got, err = NormalizeTokenFallbackModels(items)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "at most")
}

func TestNormalizeTokenFallbackModelsAllowsEmptyList(t *testing.T) {
	got, err := NormalizeTokenFallbackModels(nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestNormalizeDisabledTokenFallbackModelsKeepsIncompleteDraft(t *testing.T) {
	input := []TokenFallbackModel{
		{SourceModel: " gpt-5 ", Group: " vip ", Model: " claude-sonnet-4-5 "},
		{SourceModel: "   ", Group: "  ", Model: " "},
	}

	got, err := NormalizeDisabledTokenFallbackModels(input)
	require.NoError(t, err)
	assert.Equal(t, []TokenFallbackModel{
		{SourceModel: "gpt-5", Group: "vip", Model: "claude-sonnet-4-5"},
		{},
	}, got)
	assert.Equal(t, " gpt-5 ", input[0].SourceModel, "normalization must not mutate the caller's draft")
}

func TestNormalizeDisabledTokenFallbackModelsRejectsInvalidFieldShape(t *testing.T) {
	tests := []struct {
		name    string
		item    TokenFallbackModel
		wantErr string
	}{
		{name: "source contains whitespace", item: TokenFallbackModel{SourceModel: "gpt 5"}, wantErr: "source model is invalid"},
		{name: "group contains control character", item: TokenFallbackModel{Group: "v\tip"}, wantErr: "group is invalid"},
		{name: "model too long", item: TokenFallbackModel{Model: strings.Repeat("m", MaxTokenFallbackNameLength+1)}, wantErr: "model is invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDisabledTokenFallbackModels([]TokenFallbackModel{tt.item})
			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNormalizeDisabledTokenFallbackModelsEnforcesListLimit(t *testing.T) {
	items := make([]TokenFallbackModel, MaxTokenFallbackModels+1)

	got, err := NormalizeDisabledTokenFallbackModels(items)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "at most")
}

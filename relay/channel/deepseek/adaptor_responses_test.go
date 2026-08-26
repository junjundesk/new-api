package deepseek

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDeepSeekResponsesInputConvertsSummaryToReasoningText(t *testing.T) {
	input := json.RawMessage(`[
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]},
		{"type":"reasoning","summary":[{"type":"summary_text","text":"think step by step"}]}
	]`)

	out, err := normalizeDeepSeekResponsesInput(input)
	require.NoError(t, err)

	var items []map[string]any
	require.NoError(t, common.Unmarshal(out, &items))
	require.Len(t, items, 2)

	reasoning, ok := items[1]["content"].([]any)
	require.True(t, ok)
	require.Len(t, reasoning, 1)
	part, ok := reasoning[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "reasoning_text", part["type"])
	assert.Equal(t, "think step by step", part["text"])

	_, hasSummary := items[1]["summary"]
	assert.False(t, hasSummary)
}

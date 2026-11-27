package common

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepCopyResponsesRequestIsolatesRelayAttempts(t *testing.T) {
	original := &dto.OpenAIResponsesRequest{
		Model:           "gpt-test",
		Input:           json.RawMessage(`"hello"`),
		Reasoning:       &dto.Reasoning{Context: json.RawMessage(`{"turn":1}`)},
		Store:           json.RawMessage(`false`),
		MaxOutputTokens: new(uint),
	}
	attempt, err := DeepCopy(original)
	require.NoError(t, err)
	require.Equal(t, original, attempt)

	copy(attempt.Input, `"world"`)
	copy(attempt.Reasoning.Context, `{"turn":2}`)
	*attempt.MaxOutputTokens = 1
	assert.Equal(t, json.RawMessage(`"hello"`), original.Input)
	assert.Equal(t, json.RawMessage(`{"turn":1}`), original.Reasoning.Context)
	assert.Zero(t, *original.MaxOutputTokens)
}

func BenchmarkDeepCopyResponsesRequest(b *testing.B) {
	// A synthetic JSON string exercises the same raw-input copy path as a large conversation.
	input := bytes.Repeat([]byte{'a'}, 22<<20)
	input[0], input[len(input)-1] = '"', '"'
	req := &dto.OpenAIResponsesRequest{Model: "gpt-test", Input: input}
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for b.Loop() {
		_, err := DeepCopy(req)
		require.NoError(b, err)
	}
}

package claudemessages

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	"github.com/stretchr/testify/require"
)

func testMeta(upstreamModel string) convmeta.Meta {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   upstreamModel,
	}
}

func convertClaudeRequest(t *testing.T, raw string, info convmeta.Meta) *dto.GeneralOpenAIRequest {
	t.Helper()
	var request dto.ClaudeRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &request))
	result, err := ClaudeMessagesRequestToOpenAIChat(request, info)
	require.NoError(t, err)
	return result
}

func TestClaudeRequestKeepsTextAlongsideToolUse(t *testing.T) {
	result := convertClaudeRequest(t, `{
		"model": "claude-test",
		"max_tokens": 100,
		"messages": [{
			"role": "assistant",
			"content": [
				{"type": "text", "text": "Let me check the weather."},
				{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": {"city": "Paris"}}
			]
		}],
		"tools": [{"name": "get_weather", "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}}]
	}`, testMeta("gpt-5"))

	require.Len(t, result.Messages, 1)
	message := result.Messages[0]
	// 单个 text part 简化为字符串，同时保留 tool_calls。
	require.Equal(t, "Let me check the weather.", message.Content)
	require.Len(t, message.ParseToolCalls(), 1)
	require.Equal(t, "get_weather", message.ParseToolCalls()[0].Function.Name)
}

func TestClaudeRequestMapsToolChoice(t *testing.T) {
	base := `{
		"model": "claude-test",
		"max_tokens": 100,
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"name": "get_weather", "input_schema": {"type": "object"}}]
	}`

	t.Run("any becomes required", func(t *testing.T) {
		result := convertClaudeRequest(t, withToolChoice(base, `"any"`), testMeta("gpt-5"))
		require.Equal(t, "required", result.ToolChoice)
	})

	t.Run("forced tool becomes function selector", func(t *testing.T) {
		result := convertClaudeRequest(t, withToolChoice(base,
			`{"type":"tool","name":"get_weather","disable_parallel_tool_use":true}`), testMeta("gpt-5"))
		require.Equal(t, map[string]any{
			"type":     "function",
			"function": map[string]any{"name": "get_weather"},
		}, result.ToolChoice)
		require.NotNil(t, result.ParallelTooCalls)
		require.False(t, *result.ParallelTooCalls)
	})

	t.Run("tool choice dropped without tools", func(t *testing.T) {
		request := `{
			"model": "claude-test",
			"max_tokens": 100,
			"messages": [{"role": "user", "content": "hi"}],
			"tool_choice": "any"
		}`
		result := convertClaudeRequest(t, request, testMeta("gpt-5"))
		require.Nil(t, result.ToolChoice)
		require.Nil(t, result.ParallelTooCalls)
	})
}

func TestClaudeRequestCleansTools(t *testing.T) {
	result := convertClaudeRequest(t, `{
		"model": "claude-test",
		"max_tokens": 100,
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [
			{"type": "BatchTool", "name": "batch", "input_schema": {"type": "object"}},
			{"name": "open_url", "input_schema": {"properties": {"url": {"type": "string", "format": "uri"}}}}
		]
	}`, testMeta("gpt-5"))

	require.Len(t, result.Tools, 1)
	tool := result.Tools[0]
	require.Equal(t, "open_url", tool.Function.Name)
	require.Empty(t, tool.Function.Description)
	schema, ok := tool.Function.Parameters.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "object", schema["type"])
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	urlSchema, ok := properties["url"].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, urlSchema, "format")
}

func TestClaudeRequestMovesToolResultMedia(t *testing.T) {
	result := convertClaudeRequest(t, `{
		"model": "claude-test",
		"max_tokens": 100,
		"messages": [
			{"role": "user", "content": "look at the file"},
			{"role": "assistant", "content": [
				{"type": "tool_use", "id": "toolu_img", "name": "read_file", "input": {}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_img", "content": [
					{"type": "text", "text": "see attached"},
					{"type": "image", "source": {"type": "base64", "media_type": "image/jpeg", "data": "Zm9v"}}
				]}
			]}
		]
	}`, testMeta("gpt-5"))

	require.Len(t, result.Messages, 4)
	toolMessage := result.Messages[2]
	require.Equal(t, "tool", toolMessage.Role)
	require.Equal(t, "see attached\n"+toolMediaPlaceholder, toolMessage.Content)

	mediaMessage := result.Messages[3]
	require.Equal(t, "user", mediaMessage.Role)
	mediaParts := mediaMessage.ParseContent()
	require.Len(t, mediaParts, 2)
	require.Equal(t, "text", mediaParts[0].Type)
	require.Equal(t, "[tool output media of toolu_img]", mediaParts[0].Text)
	require.Equal(t, "image_url", mediaParts[1].Type)
	require.Equal(t, "data:image/jpeg;base64,Zm9v", mediaParts[1].GetImageMedia().Url)
}

func TestClaudeRequestImageWithoutSourceIsSkipped(t *testing.T) {
	result := convertClaudeRequest(t, `{
		"model": "claude-test",
		"max_tokens": 100,
		"messages": [{"role": "user", "content": [
			{"type": "text", "text": "hi"},
			{"type": "image"}
		]}]
	}`, testMeta("gpt-5"))

	require.Len(t, result.Messages, 1)
	require.Equal(t, "hi", result.Messages[0].Content)
}

func TestClaudeRequestSystemStripsBillingHeader(t *testing.T) {
	t.Run("array system", func(t *testing.T) {
		result := convertClaudeRequest(t, `{
			"model": "claude-test",
			"max_tokens": 100,
			"system": [
				{"type": "text", "text": "x-anthropic-billing-header: cch=abc\n\nYou are helpful."},
				{"type": "text", "text": "Second part"}
			],
			"messages": [{"role": "user", "content": "hi"}]
		}`, testMeta("gpt-5"))

		require.Equal(t, "system", result.Messages[0].Role)
		require.Equal(t, "You are helpful.\nSecond part", result.Messages[0].Content)
	})

	t.Run("string system", func(t *testing.T) {
		result := convertClaudeRequest(t, `{
			"model": "claude-test",
			"max_tokens": 100,
			"system": "x-anthropic-billing-header: cch=abc\r\n\r\nYou are helpful.",
			"messages": [{"role": "user", "content": "hi"}]
		}`, testMeta("gpt-5"))

		require.Equal(t, "system", result.Messages[0].Role)
		require.Equal(t, "You are helpful.", result.Messages[0].Content)
	})
}

func TestClaudeRequestReasoningEffortFromThinking(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		thinking   string
		output     string
		wantEffort string
	}{
		{name: "small budget", model: "gpt-5", thinking: `{"type":"enabled","budget_tokens":512}`, wantEffort: "low"},
		{name: "medium budget", model: "gpt-5", thinking: `{"type":"enabled","budget_tokens":8000}`, wantEffort: "medium"},
		{name: "large budget", model: "gpt-5", thinking: `{"type":"enabled","budget_tokens":20000}`, wantEffort: "high"},
		{name: "missing budget", model: "gpt-5", thinking: `{"type":"enabled"}`, wantEffort: "high"},
		{name: "adaptive", model: "gpt-5", thinking: `{"type":"adaptive"}`, wantEffort: "xhigh"},
		{name: "explicit effort wins", model: "gpt-5", thinking: `{"type":"enabled","budget_tokens":512}`, output: `{"effort":"high"}`, wantEffort: "high"},
		{name: "max downgraded for model", model: "gpt-5", output: `{"effort":"max"}`, wantEffort: "xhigh"},
		{name: "gpt-5-pro only high", model: "gpt-5-pro", thinking: `{"type":"enabled","budget_tokens":512}`, wantEffort: "high"},
		{name: "other pro raises low", model: "gpt-5.1-pro", thinking: `{"type":"enabled","budget_tokens":512}`, wantEffort: "medium"},
		{name: "thinking disabled drops to low", model: "gpt-5", thinking: `{"type":"disabled"}`, output: `{"effort":"high"}`, wantEffort: "low"},
		{name: "nothing requested", model: "gpt-5", wantEffort: ""},
		{name: "deepseek keeps effort", model: "deepseek-chat", output: `{"effort":"xhigh"}`, wantEffort: "xhigh"},
		{name: "kimi omits effort", model: "kimi-k2", output: `{"effort":"high"}`, wantEffort: ""},
		{name: "unknown model keeps high", model: "some-llm", output: `{"effort":"high"}`, wantEffort: "high"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := map[string]any{
				"model":    "claude-test",
				"messages": []any{map[string]any{"role": "user", "content": "hi"}},
			}
			if tc.thinking != "" {
				request["thinking"] = json.RawMessage(tc.thinking)
			}
			if tc.output != "" {
				request["output_config"] = json.RawMessage(tc.output)
			}
			raw, err := json.Marshal(request)
			require.NoError(t, err)

			result := convertClaudeRequest(t, string(raw), testMeta(tc.model))
			require.Equal(t, tc.wantEffort, result.ReasoningEffort)
		})
	}
}

func TestClaudeRequestOpenRouterReasoning(t *testing.T) {
	info := &convmeta.Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   "anthropic/claude-sonnet-4",
		Options:             &convmeta.Options{OpenRouterDialect: true},
	}

	result := convertClaudeRequest(t, `{
		"model": "claude-test",
		"max_tokens": 100,
		"messages": [{"role": "user", "content": "hi"}],
		"thinking": {"type": "enabled", "budget_tokens": 512}
	}`, info)

	var reasoningConfig map[string]any
	require.NoError(t, json.Unmarshal(result.Reasoning, &reasoningConfig))
	require.Equal(t, true, reasoningConfig["enabled"])
	require.Equal(t, "low", reasoningConfig["effort"])
	require.Equal(t, float64(512), reasoningConfig["max_tokens"])
	// effort 不应再写进 verbosity。
	require.Empty(t, result.Verbosity)
}

func withToolChoice(base, toolChoice string) string {
	return `{
		"model": "claude-test",
		"max_tokens": 100,
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"name": "get_weather", "input_schema": {"type": "object"}}],
		"tool_choice": ` + toolChoice + `
	}`
}

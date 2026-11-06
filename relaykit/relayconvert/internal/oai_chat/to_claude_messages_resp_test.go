package oaichat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseOpenAI2ClaudeToolUseInputIsObject(t *testing.T) {
	tests := []struct {
		name string
		args string
		want map[string]interface{}
	}{
		{name: "object", args: `{"q":"x"}`, want: map[string]interface{}{"q": "x"}},
		{name: "empty", args: "", want: map[string]interface{}{}},
		{name: "invalid", args: "{", want: map[string]interface{}{}},
		{name: "null", args: "null", want: map[string]interface{}{}},
		{name: "array", args: `["x"]`, want: map[string]interface{}{}},
		{name: "string", args: `"x"`, want: map[string]interface{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := dto.Message{Role: "assistant"}
			msg.SetToolCalls([]dto.ToolCallRequest{
				{
					ID:   "call_1",
					Type: "function",
					Function: dto.FunctionRequest{
						Name:      "lookup",
						Arguments: tt.args,
					},
				},
			})

			resp := ResponseOpenAI2Claude(&dto.OpenAITextResponse{
				Id:    "chatcmpl_1",
				Model: "gpt-test",
				Choices: []dto.OpenAITextResponseChoice{
					{Message: msg, FinishReason: "tool_calls"},
				},
			}, nil)

			require.Len(t, resp.Content, 1)
			assert.Equal(t, "tool_use", resp.Content[0].Type)
			assert.Equal(t, tt.want, resp.Content[0].Input)
		})
	}
}

func TestResponseOpenAI2ClaudeUsageCarriesOpenAIBillingUsage(t *testing.T) {
	resp := ResponseOpenAI2Claude(&dto.OpenAITextResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{Message: dto.Message{Role: "assistant", Content: "hello"}, FinishReason: "stop"},
		},
		Usage: dto.Usage{
			PromptTokens:     11,
			CompletionTokens: 5,
			TotalTokens:      16,
		},
	}, nil)

	require.NotNil(t, resp.Usage)
	assert.Equal(t, 11, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
	require.NotNil(t, resp.Usage.BillingUsage)
	require.NotNil(t, resp.Usage.BillingUsage.OpenAIUsage)
	assert.Equal(t, dto.BillingUsageSourceOAIChat, resp.Usage.BillingUsage.Source)
	assert.Equal(t, dto.BillingUsageSemanticOpenAI, resp.Usage.BillingUsage.Semantic)
	assert.Equal(t, 11, resp.Usage.BillingUsage.OpenAIUsage.PromptTokens)
	assert.Equal(t, 5, resp.Usage.BillingUsage.OpenAIUsage.CompletionTokens)
	assert.Equal(t, 16, resp.Usage.BillingUsage.OpenAIUsage.TotalTokens)
	assert.Nil(t, resp.Usage.BillingUsage.OpenAIUsage.BillingUsage)
}

func TestBuildClaudeUsageFromOpenAICacheWriteUsage(t *testing.T) {
	usage := buildClaudeUsageFromOpenAIUsage(&dto.Usage{
		PromptTokens:     3619,
		CompletionTokens: 36,
		TotalTokens:      3655,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:     2921,
			CacheWriteTokens: 3616,
		},
	})

	require.NotNil(t, usage)
	// Claude semantics reports input_tokens excluding cache read/write; the
	// overlapping unadjusted prefixes drive the remainder negative, clamp to 0.
	assert.Equal(t, 0, usage.InputTokens)
	assert.Equal(t, 2921, usage.CacheReadInputTokens)
	assert.Equal(t, 3616, usage.CacheCreationInputTokens)
	assert.Equal(t, 36, usage.OutputTokens)
	require.NotNil(t, usage.BillingUsage)
	require.NotNil(t, usage.BillingUsage.OpenAIUsage)
	assert.Equal(t, dto.BillingUsageSemanticOpenAI, usage.BillingUsage.Semantic)
	assert.Equal(t, 3616, usage.BillingUsage.OpenAIUsage.PromptTokensDetails.CacheWriteTokens)
}

func TestStreamResponseOpenAI2ClaudeClosesTextThinkingAndToolBlocks(t *testing.T) {
	info := &convmeta.Values{
		ClaudeConvertInfo: &convmeta.ClaudeConvertInfo{
			LastMessagesType: convmeta.LastMessageTypeNone,
		},
	}

	info.SendResponseCount = 1
	textResponses := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: ptr("hello"),
				},
			},
		},
	}, info)
	require.Len(t, textResponses, 3)
	assert.Equal(t, "message_start", textResponses[0].Type)
	assert.Equal(t, "content_block_start", textResponses[1].Type)
	assert.Equal(t, 0, textResponses[1].GetIndex())
	assert.Equal(t, "content_block_delta", textResponses[2].Type)

	info.SendResponseCount = 2
	thinkingResponses := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ReasoningContent: ptr("thinking"),
				},
			},
		},
	}, info)
	require.Len(t, thinkingResponses, 3)
	assert.Equal(t, "content_block_stop", thinkingResponses[0].Type)
	assert.Equal(t, 0, thinkingResponses[0].GetIndex())
	assert.Equal(t, "content_block_start", thinkingResponses[1].Type)
	assert.Equal(t, 1, thinkingResponses[1].GetIndex())
	assert.Equal(t, "thinking", thinkingResponses[1].ContentBlock.Type)
	assert.Equal(t, "content_block_delta", thinkingResponses[2].Type)

	info.SendResponseCount = 3
	toolResponses := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: ptr(0),
							ID:    "call_1",
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      "lookup",
								Arguments: `{"q":"x"}`,
							},
						},
					},
				},
			},
		},
	}, info)
	require.Len(t, toolResponses, 3)
	assert.Equal(t, "content_block_stop", toolResponses[0].Type)
	assert.Equal(t, 1, toolResponses[0].GetIndex())
	assert.Equal(t, "content_block_start", toolResponses[1].Type)
	assert.Equal(t, 2, toolResponses[1].GetIndex())
	assert.Equal(t, "tool_use", toolResponses[1].ContentBlock.Type)
	assert.Equal(t, "content_block_delta", toolResponses[2].Type)

	info.SendResponseCount = 4
	finishResponses := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{FinishReason: ptr("tool_calls")},
		},
		Usage: &dto.Usage{
			PromptTokens:     7,
			CompletionTokens: 3,
			TotalTokens:      10,
		},
	}, info)
	require.Len(t, finishResponses, 3)
	assert.Equal(t, "content_block_stop", finishResponses[0].Type)
	assert.Equal(t, 2, finishResponses[0].GetIndex())
	assert.Equal(t, "message_delta", finishResponses[1].Type)
	assert.Equal(t, "tool_use", *finishResponses[1].Delta.StopReason)
	require.NotNil(t, finishResponses[1].Usage)
	require.NotNil(t, finishResponses[1].Usage.BillingUsage)
	require.NotNil(t, finishResponses[1].Usage.BillingUsage.OpenAIUsage)
	assert.Equal(t, 7, finishResponses[1].Usage.BillingUsage.OpenAIUsage.PromptTokens)
	assert.Equal(t, 3, finishResponses[1].Usage.BillingUsage.OpenAIUsage.CompletionTokens)
	assert.Equal(t, "message_stop", finishResponses[2].Type)
}

func TestNormalizeCacheCreationSplit(t *testing.T) {
	cache5m, cache1h := NormalizeCacheCreationSplit(10, 3, 2)
	assert.Equal(t, 8, cache5m)
	assert.Equal(t, 2, cache1h)

	cache5m, cache1h = NormalizeCacheCreationSplit(3, 5, 1)
	assert.Equal(t, 5, cache5m)
	assert.Equal(t, 1, cache1h)
}

// ---------------------------------------------------------------------------
// 流式状态机
// ---------------------------------------------------------------------------

func newStreamInfo() *convmeta.Values {
	return &convmeta.Values{
		ClaudeConvertInfo: &convmeta.ClaudeConvertInfo{
			LastMessagesType: convmeta.LastMessageTypeNone,
		},
	}
}

func streamChunk(choices []dto.ChatCompletionsStreamResponseChoice, usage *dto.Usage) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Model:   "gpt-test",
		Choices: choices,
		Usage:   usage,
	}
}

func contentChoice(content string) dto.ChatCompletionsStreamResponseChoice {
	return dto.ChatCompletionsStreamResponseChoice{
		Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: ptr(content)},
	}
}

func toolChoice(toolCall dto.ToolCallResponse) dto.ChatCompletionsStreamResponseChoice {
	return dto.ChatCompletionsStreamResponseChoice{
		Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{toolCall}},
	}
}

func finishChoice(reason string) dto.ChatCompletionsStreamResponseChoice {
	return dto.ChatCompletionsStreamResponseChoice{FinishReason: ptr(reason)}
}

func streamEventTypes(responses []*dto.ClaudeResponse) []string {
	types := make([]string, 0, len(responses))
	for _, response := range responses {
		types = append(types, response.Type)
	}
	return types
}

func streamChunkTypes(t *testing.T, info *convmeta.Values, chunk *dto.ChatCompletionsStreamResponse) []string {
	t.Helper()
	return streamEventTypes(StreamResponseOpenAI2Claude(chunk, info))
}

func TestStreamResponseOpenAI2ClaudeSingleChunkHasMessageStart(t *testing.T) {
	info := newStreamInfo()
	// 单 chunk 流：正文、finish_reason 与 usage 都在同一片里。
	responses := StreamResponseOpenAI2Claude(streamChunk(
		[]dto.ChatCompletionsStreamResponseChoice{{
			Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{Content: ptr("hi")},
			FinishReason: ptr("stop"),
		}},
		&dto.Usage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6},
	), info)

	require.Equal(t, []string{
		"message_start", "content_block_start", "content_block_delta",
		"content_block_stop", "message_delta", "message_stop",
	}, streamEventTypes(responses))
	// 单 chunk 流也能拿到 message_start，且 input_tokens 不再恒为估算值。
	require.NotNil(t, responses[0].Message)
	require.NotNil(t, responses[0].Message.Usage)
	assert.Equal(t, 4, responses[0].Message.Usage.InputTokens)
	assert.Equal(t, "end_turn", *responses[4].Delta.StopReason)
}

func TestStreamResponseOpenAI2ClaudeFinishWithoutUsageWaitsForFinalize(t *testing.T) {
	info := newStreamInfo()
	StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{contentChoice("hello")}, nil), info)

	// finish chunk 不带 usage：此时不能直接丢弃终态，也不能提前发 message_delta。
	deferred := streamChunkTypes(t, info, streamChunk([]dto.ChatCompletionsStreamResponseChoice{finishChoice("stop")}, nil))
	assert.Empty(t, deferred)

	finalized := FinalizeStreamResponseOpenAI2Claude(info)
	require.Equal(t, []string{
		"content_block_stop", "message_delta", "message_stop",
	}, streamEventTypes(finalized))
	require.NotNil(t, finalized[1].Usage)
	assert.Equal(t, 0, finalized[1].Usage.InputTokens)
	assert.Equal(t, "end_turn", *finalized[1].Delta.StopReason)

	// 幂等：Done 后反复调用不再发事件。
	assert.Empty(t, FinalizeStreamResponseOpenAI2Claude(info))
}

func TestStreamResponseOpenAI2ClaudeFinishThenUsageChunkEmitsTerminal(t *testing.T) {
	info := newStreamInfo()
	StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{contentChoice("hello")}, nil), info)

	StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{finishChoice("stop")}, nil), info)

	usage := &dto.Usage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6}
	terminal := StreamResponseOpenAI2Claude(streamChunk(nil, usage), info)
	require.Equal(t, []string{
		"content_block_stop", "message_delta", "message_stop",
	}, streamEventTypes(terminal))
	require.NotNil(t, terminal[1].Usage)
	assert.Equal(t, 4, terminal[1].Usage.InputTokens)
	assert.Equal(t, 2, terminal[1].Usage.OutputTokens)
}

func TestStreamResponseOpenAI2ClaudeRepeatedFinishKeepsSingleDelta(t *testing.T) {
	info := newStreamInfo()
	usage := &dto.Usage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6}
	first := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{finishChoice("stop")}, usage), info)
	require.Equal(t, []string{"message_start", "message_delta", "message_stop"}, streamEventTypes(first))

	// 重复 finish chunk 只更新 usage，不重复发 message_delta / message_stop。
	repeated := StreamResponseOpenAI2Claude(streamChunk(
		[]dto.ChatCompletionsStreamResponseChoice{finishChoice("stop")},
		&dto.Usage{PromptTokens: 8, CompletionTokens: 3, TotalTokens: 11},
	), info)
	assert.Empty(t, repeated)
	assert.Empty(t, FinalizeStreamResponseOpenAI2Claude(info))
}

func TestStreamResponseOpenAI2ClaudeToolStartsAfterIDAndName(t *testing.T) {
	info := newStreamInfo()

	// 参数先到：id/name 未就绪，不能开块。
	early := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(0), Function: dto.FunctionResponse{Arguments: `{"a":`}}),
	}, nil), info)
	require.Equal(t, []string{"message_start"}, streamEventTypes(early))

	late := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{
			Index: ptr(0),
			ID:    "call_1",
			Function: dto.FunctionResponse{
				Name:      "lookup",
				Arguments: `1}`,
			},
		}),
	}, nil), info)
	require.Equal(t, []string{"content_block_start", "content_block_delta"}, streamEventTypes(late))
	require.NotNil(t, late[0].ContentBlock)
	assert.Equal(t, "tool_use", late[0].ContentBlock.Type)
	assert.Equal(t, "call_1", late[0].ContentBlock.Id)
	assert.Equal(t, "lookup", late[0].ContentBlock.Name)
	assert.Equal(t, map[string]interface{}{}, late[0].ContentBlock.Input)
	assert.Equal(t, 0, late[0].GetIndex())
	// 启动时把缓存参数一次性补发。
	assert.Equal(t, `{"a":1}`, *late[1].Delta.PartialJson)

	more := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(0), Function: dto.FunctionResponse{Arguments: ` `}}),
	}, nil), info)
	require.Equal(t, []string{"content_block_delta"}, streamEventTypes(more))
	assert.Equal(t, " ", *more[0].Delta.PartialJson)

	terminal := StreamResponseOpenAI2Claude(streamChunk(
		[]dto.ChatCompletionsStreamResponseChoice{finishChoice("tool_calls")},
		&dto.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10},
	), info)
	require.Equal(t, []string{"content_block_stop", "message_delta", "message_stop"}, streamEventTypes(terminal))
	assert.Equal(t, 0, terminal[0].GetIndex())
	assert.Equal(t, "tool_use", *terminal[1].Delta.StopReason)
}

func TestStreamResponseOpenAI2ClaudeParallelToolInterleaved(t *testing.T) {
	info := newStreamInfo()

	// 工具 0 先出现（参数就绪但缺 id/name），占用 block 0。
	StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(0), Function: dto.FunctionResponse{Arguments: `{"a":1}`}}),
	}, nil), info)

	// 工具 1 后出现，按首次出现顺序分配 block 1。
	start1 := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{
			Index: ptr(1),
			ID:    "call_1",
			Function: dto.FunctionResponse{
				Name:      "second",
				Arguments: `{"b":2}`,
			},
		}),
	}, nil), info)
	require.Equal(t, []string{"content_block_start", "content_block_delta"}, streamEventTypes(start1))
	assert.Equal(t, 1, start1[0].GetIndex())

	start0 := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{
			Index:    ptr(0),
			ID:       "call_0",
			Function: dto.FunctionResponse{Name: "first"},
		}),
	}, nil), info)
	require.Equal(t, []string{"content_block_start", "content_block_delta"}, streamEventTypes(start0))
	assert.Equal(t, 0, start0[0].GetIndex())
	assert.Equal(t, `{"a":1}`, *start0[1].Delta.PartialJson)

	// 关闭按 index 升序，且只关真正打开的块。
	terminal := StreamResponseOpenAI2Claude(streamChunk(
		[]dto.ChatCompletionsStreamResponseChoice{finishChoice("tool_calls")},
		&dto.Usage{PromptTokens: 5, CompletionTokens: 1, TotalTokens: 6},
	), info)
	require.Equal(t, []string{"content_block_stop", "content_block_stop", "message_delta", "message_stop"}, streamEventTypes(terminal))
	assert.Equal(t, 0, terminal[0].GetIndex())
	assert.Equal(t, 1, terminal[1].GetIndex())
}

func TestStreamResponseOpenAI2ClaudeNonContiguousToolIndex(t *testing.T) {
	info := newStreamInfo()
	// 上游只发 index 0 与 2：不能给从未打开的 index 1 发 content_block_stop。
	StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(0), ID: "call_0", Function: dto.FunctionResponse{Name: "a"}}),
	}, nil), info)
	start2 := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(2), ID: "call_2", Function: dto.FunctionResponse{Name: "b"}}),
	}, nil), info)
	require.Len(t, start2, 1)
	assert.Equal(t, 1, start2[0].GetIndex())

	terminal := StreamResponseOpenAI2Claude(streamChunk(
		[]dto.ChatCompletionsStreamResponseChoice{finishChoice("tool_calls")},
		&dto.Usage{PromptTokens: 5, CompletionTokens: 1, TotalTokens: 6},
	), info)
	require.Equal(t, []string{"content_block_stop", "content_block_stop", "message_delta", "message_stop"}, streamEventTypes(terminal))
	assert.Equal(t, 0, terminal[0].GetIndex())
	assert.Equal(t, 1, terminal[1].GetIndex())
}

func TestStreamResponseOpenAI2ClaudeReasoningAndContentSameChunk(t *testing.T) {
	info := newStreamInfo()
	responses := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
			ReasoningContent: ptr("think first"),
			Content:          ptr("answer"),
		}},
	}, nil), info)

	require.Equal(t, []string{
		"message_start",
		"content_block_start", "content_block_delta",
		"content_block_stop",
		"content_block_start", "content_block_delta",
	}, streamEventTypes(responses))
	assert.Equal(t, "thinking", responses[1].ContentBlock.Type)
	assert.Equal(t, 0, responses[1].GetIndex())
	assert.Equal(t, "think first", *responses[2].Delta.Thinking)
	assert.Equal(t, 0, responses[3].GetIndex())
	assert.Equal(t, "text", responses[4].ContentBlock.Type)
	assert.Equal(t, 1, responses[4].GetIndex())
	assert.Equal(t, "answer", *responses[5].Delta.Text)
}

func TestStreamResponseOpenAI2ClaudeInlineThinkAcrossChunks(t *testing.T) {
	info := newStreamInfo()
	first := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{contentChoice("<thi")}, nil), info)
	require.Equal(t, []string{"message_start"}, streamEventTypes(first))

	second := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{contentChoice("nk>hello</thi")}, nil), info)
	require.Equal(t, []string{"content_block_start", "content_block_delta"}, streamEventTypes(second))
	assert.Equal(t, "thinking", second[0].ContentBlock.Type)
	assert.Equal(t, "hello", *second[1].Delta.Thinking)

	third := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{contentChoice("nk>\nworld")}, nil), info)
	require.Equal(t, []string{
		"content_block_stop", "content_block_start", "content_block_delta",
	}, streamEventTypes(third))
	assert.Equal(t, 0, third[0].GetIndex())
	assert.Equal(t, "text", third[1].ContentBlock.Type)
	// 闭合标签后的一个换行被吞掉。
	assert.Equal(t, "world", *third[2].Delta.Text)
}

func TestStreamResponseOpenAI2ClaudeFinalizeFlushesTruncatedThink(t *testing.T) {
	info := newStreamInfo()
	StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{contentChoice("<think>abc</thi")}, nil), info)

	// 流被截断（没有 finish_reason）：finalize 先把扣住的增量冲刷出去再收尾。
	finalized := FinalizeStreamResponseOpenAI2Claude(info)
	require.Equal(t, []string{
		"content_block_delta", "content_block_stop", "message_delta", "message_stop",
	}, streamEventTypes(finalized))
	assert.Equal(t, "</thi", *finalized[0].Delta.Thinking)
	assert.Equal(t, "end_turn", *finalized[2].Delta.StopReason)
}

func TestStreamResponseOpenAI2ClaudeToolBoundaryFlushesInlineThink(t *testing.T) {
	info := newStreamInfo()
	StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{contentChoice("<think>abc</thi")}, nil), info)

	// 工具边界：先把扣住的思考增量冲刷出去并关块，再开工具块。
	responses := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(0), ID: "call_0", Function: dto.FunctionResponse{Name: "lookup"}}),
	}, nil), info)
	require.Equal(t, []string{"content_block_delta", "content_block_stop", "content_block_start"}, streamEventTypes(responses))
	assert.Equal(t, "</thi", *responses[0].Delta.Thinking)
	assert.Equal(t, 0, responses[1].GetIndex())
	assert.Equal(t, "tool_use", responses[2].ContentBlock.Type)
	assert.Equal(t, 1, responses[2].GetIndex())
}

func TestStreamResponseOpenAI2ClaudeAbortsInfiniteBlankArguments(t *testing.T) {
	info := newStreamInfo()
	StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(0), ID: "call_0", Function: dto.FunctionResponse{Name: "lookup"}}),
	}, nil), info)

	blank := strings.Repeat(" ", 400)
	StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(0), Function: dto.FunctionResponse{Arguments: blank}}),
	}, nil), info)
	// 累计超过 500 后中止该工具，后续同 index 片段忽略。
	aborting := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(0), Function: dto.FunctionResponse{Arguments: blank}}),
	}, nil), info)
	assert.Empty(t, aborting)

	ignored := StreamResponseOpenAI2Claude(streamChunk([]dto.ChatCompletionsStreamResponseChoice{
		toolChoice(dto.ToolCallResponse{Index: ptr(0), Function: dto.FunctionResponse{Arguments: `{"a":1}`}}),
	}, nil), info)
	assert.Empty(t, ignored)
}

// ---------------------------------------------------------------------------
// 非流式
// ---------------------------------------------------------------------------

func TestResponseOpenAI2ClaudeThinkingAndInlineThink(t *testing.T) {
	resp := ResponseOpenAI2Claude(&dto.OpenAITextResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{{
			Message:      dto.Message{Role: "assistant", Content: "<think>inline</think>\nbody"},
			FinishReason: "stop",
		}},
	}, nil)

	require.Len(t, resp.Content, 2)
	assert.Equal(t, "thinking", resp.Content[0].Type)
	assert.Equal(t, "inline", *resp.Content[0].Thinking)
	assert.Equal(t, "text", resp.Content[1].Type)
	assert.Equal(t, "body", *resp.Content[1].Text)
	assert.Equal(t, "end_turn", resp.StopReason)
}

func TestResponseOpenAI2ClaudeLegacyFunctionCall(t *testing.T) {
	tests := []struct {
		name      string
		function  string
		wantInput any
	}{
		{name: "string arguments", function: `{"id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"}`, wantInput: map[string]interface{}{"q": "x"}},
		{name: "object arguments", function: `{"id":"call_1","name":"lookup","arguments":{"q":"x"}}`, wantInput: map[string]interface{}{"q": "x"}},
		{name: "array arguments", function: `{"id":"call_1","name":"lookup","arguments":["x"]}`, wantInput: []any{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := dto.Message{Role: "assistant"}
			require.NoError(t, json.Unmarshal([]byte(tt.function), &message.FunctionCall))

			resp := ResponseOpenAI2Claude(&dto.OpenAITextResponse{
				Id:    "chatcmpl_1",
				Model: "gpt-test",
				Choices: []dto.OpenAITextResponseChoice{{
					Message:      message,
					FinishReason: "function_call",
				}},
			}, nil)

			require.Len(t, resp.Content, 1)
			assert.Equal(t, "tool_use", resp.Content[0].Type)
			assert.Equal(t, "call_1", resp.Content[0].Id)
			assert.Equal(t, "lookup", resp.Content[0].Name)
			assert.Equal(t, tt.wantInput, resp.Content[0].Input)
			assert.Equal(t, "tool_use", resp.StopReason)
		})
	}
}

func TestResponseOpenAI2ClaudeStopReasonFallback(t *testing.T) {
	// finish_reason 缺失但产出了 tool_use 块：回填 tool_use。
	message := dto.Message{Role: "assistant"}
	message.SetToolCalls([]dto.ToolCallRequest{{
		ID:       "call_1",
		Type:     "function",
		Function: dto.FunctionRequest{Name: "lookup", Arguments: `{}`},
	}})
	resp := ResponseOpenAI2Claude(&dto.OpenAITextResponse{
		Id:      "chatcmpl_1",
		Choices: []dto.OpenAITextResponseChoice{{Message: message}},
	}, nil)
	assert.Equal(t, "tool_use", resp.StopReason)

	// 普通文本且 finish_reason 缺失：兜底 end_turn，字段不被 omitempty 吃掉。
	resp = ResponseOpenAI2Claude(&dto.OpenAITextResponse{
		Id:      "chatcmpl_2",
		Choices: []dto.OpenAITextResponseChoice{{Message: dto.Message{Role: "assistant", Content: "hi"}}},
	}, nil)
	assert.Equal(t, "end_turn", resp.StopReason)

	// 未知 finish_reason 记日志后按 end_turn 处理。
	resp = ResponseOpenAI2Claude(&dto.OpenAITextResponse{
		Id:      "chatcmpl_3",
		Choices: []dto.OpenAITextResponseChoice{{Message: dto.Message{Role: "assistant", Content: "hi"}, FinishReason: "weird"}},
	}, nil)
	assert.Equal(t, "end_turn", resp.StopReason)
}

func TestResponseOpenAI2ClaudeKeepsContentFilterRefusal(t *testing.T) {
	resp := ResponseOpenAI2Claude(&dto.OpenAITextResponse{
		Id:      "chatcmpl_1",
		Choices: []dto.OpenAITextResponseChoice{{Message: dto.Message{Role: "assistant", Content: "hi"}, FinishReason: "content_filter"}},
	}, nil)
	// newapi 的 content_filter → refusal 语义优于 cc-switch 的 end_turn，保持不变。
	assert.Equal(t, "refusal", resp.StopReason)
}

// ---------------------------------------------------------------------------
// 使用量
// ---------------------------------------------------------------------------

func TestBuildClaudeUsageCacheFallbackKeepsIdentity(t *testing.T) {
	tests := []struct {
		name             string
		usage            *dto.Usage
		wantInput        int
		wantCacheRead    int
		wantCacheCreated int
	}{
		{
			name: "prompt details cached",
			usage: &dto.Usage{
				PromptTokens:        100,
				CompletionTokens:    5,
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 30},
			},
			wantInput:     70,
			wantCacheRead: 30,
		},
		{
			name: "input details fallback",
			usage: &dto.Usage{
				PromptTokens:       100,
				CompletionTokens:   5,
				InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 40, CacheWriteTokens: 10},
			},
			wantInput:        50,
			wantCacheRead:    40,
			wantCacheCreated: 10,
		},
		{
			name: "prompt cache hit tokens fallback",
			usage: &dto.Usage{
				PromptTokens:         100,
				CompletionTokens:     5,
				PromptCacheHitTokens: 70,
			},
			wantInput:     30,
			wantCacheRead: 70,
		},
		{
			name: "zero values keep looking",
			usage: &dto.Usage{
				PromptTokens:         100,
				CompletionTokens:     5,
				PromptTokensDetails:  dto.InputTokenDetails{CachedTokens: 0},
				InputTokensDetails:   &dto.InputTokenDetails{CachedTokens: 0},
				PromptCacheHitTokens: 25,
			},
			wantInput:     75,
			wantCacheRead: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := buildClaudeUsageFromOpenAIUsage(tt.usage)
			require.NotNil(t, usage)
			assert.Equal(t, tt.wantInput, usage.InputTokens)
			assert.Equal(t, tt.wantCacheRead, usage.CacheReadInputTokens)
			assert.Equal(t, tt.wantCacheCreated, usage.CacheCreationInputTokens)
			// 恒等式：input + cache_read + cache_creation == prompt_tokens
			assert.Equal(t, tt.usage.PromptTokens,
				usage.InputTokens+usage.CacheReadInputTokens+usage.CacheCreationInputTokens)
		})
	}
}

func TestSplitLeadingThinkBlock(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		wantThinking string
		wantAnswer   string
		wantOK       bool
	}{
		{name: "think", text: "<think>a</think>b", wantThinking: "a", wantAnswer: "b", wantOK: true},
		{
			name:         "thinking with leading whitespace and newline",
			text:         "  <thinking>\n a \n</thinking>\n\nb",
			wantThinking: " a ",
			wantAnswer:   "b",
			wantOK:       true,
		},
		{name: "no tag", text: "plain", wantAnswer: "plain"},
		{name: "unclosed", text: "<think>a", wantAnswer: "<think>a"},
		{name: "tag not leading", text: "a<think>b</think>", wantAnswer: "a<think>b</think>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thinking, answer, ok := splitLeadingThinkBlock(tt.text)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantThinking, thinking)
			assert.Equal(t, tt.wantAnswer, answer)
		})
	}
}

func ptr[T any](value T) *T {
	return &value
}

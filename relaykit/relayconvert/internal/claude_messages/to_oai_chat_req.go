package claudemessages

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/reasoning"
)

const (
	webSearchMaxUsesLow    = 1
	webSearchMaxUsesMedium = 5
	webSearchMaxUsesHigh   = 10

	// anthropicBillingHeaderPrefix 是 Claude Code 写在 system 首行的计费头，
	// 每轮都会带不同的滚动值，保留会让上游前缀缓存失效。
	anthropicBillingHeaderPrefix = "x-anthropic-billing-header:"
)

type openRouterRequestReasoning struct {
	Enabled   bool   `json:"enabled"`
	Effort    string `json:"effort,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	Exclude   bool   `json:"exclude,omitempty"`
}

func ClaudeMessagesRequestToOpenAIChat(claudeRequest dto.ClaudeRequest, info convmeta.Meta) (*dto.GeneralOpenAIRequest, error) {
	openAIRequest := dto.GeneralOpenAIRequest{
		Model:       claudeRequest.Model,
		Temperature: claudeRequest.Temperature,
	}
	if claudeRequest.MaxTokens != nil {
		openAIRequest.MaxTokens = kitutil.GetPointer(*claudeRequest.MaxTokens)
	}
	if claudeRequest.TopP != nil {
		openAIRequest.TopP = kitutil.GetPointer(*claudeRequest.TopP)
	}
	if claudeRequest.TopK != nil {
		openAIRequest.TopK = kitutil.GetPointer(*claudeRequest.TopK)
	}
	if claudeRequest.Stream != nil {
		openAIRequest.Stream = kitutil.GetPointer(*claudeRequest.Stream)
	}

	isOpenRouter := convmeta.OptionsOf(info).OpenRouterDialect
	outputEffort := claudeRequest.GetEfforts()
	if isOpenRouter {
		if err := applyOpenRouterReasoning(&openAIRequest, claudeRequest.Thinking, outputEffort); err != nil {
			return nil, err
		}
	} else {
		// 模型名的 -thinking 后缀由入口/映射层处理，转换层不再按客户端原始模型名反推，
		// 否则映射到 gpt-5 等上游时会出现不存在的模型名。
		upstreamModel := convmeta.UpstreamModelName(info)
		if upstreamModel == "" {
			upstreamModel = claudeRequest.Model
		}
		if effort := reasoning.EffortForModel(claudeRequest.Thinking, outputEffort, upstreamModel); effort != "" {
			openAIRequest.ReasoningEffort = effort
			if info != nil {
				info.SetReasoningEffort(effort)
			}
		}
	}

	if len(claudeRequest.StopSequences) == 1 {
		openAIRequest.Stop = claudeRequest.StopSequences[0]
	} else if len(claudeRequest.StopSequences) > 1 {
		openAIRequest.Stop = claudeRequest.StopSequences
	}

	claudeTools, err := kitutil.Any2Type[[]map[string]any](claudeRequest.Tools)
	if err != nil {
		// 单条工具解析失败时只记录日志，不要整组丢弃。
		kitutil.LogError(fmt.Sprintf("claude: failed to parse tools: %v", err))
	}
	openAITools := make([]dto.ToolCallRequest, 0, len(claudeTools))
	for _, claudeTool := range claudeTools {
		if toolType, _ := claudeTool["type"].(string); toolType == "BatchTool" {
			continue
		}
		name, _ := claudeTool["name"].(string)
		if name == "" {
			continue
		}
		openAITool := dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:       name,
				Parameters: cleanSchema(claudeTool["input_schema"]),
			},
		}
		if description, _ := claudeTool["description"].(string); description != "" {
			openAITool.Function.Description = description
		}
		openAITools = append(openAITools, openAITool)
	}
	if len(openAITools) > 0 {
		openAIRequest.Tools = openAITools
		// tool_choice 只在真正存在 tools 时下发，避免上游 400。
		if toolChoice, disableParallel := mapToolChoice(claudeRequest.ToolChoice); toolChoice != nil {
			openAIRequest.ToolChoice = toolChoice
			if disableParallel {
				openAIRequest.ParallelTooCalls = kitutil.GetPointer(false)
			}
		} else if disableParallel {
			openAIRequest.ParallelTooCalls = kitutil.GetPointer(false)
		}
	}

	openAIMessages := make([]dto.Message, 0)
	if claudeRequest.System != nil {
		if claudeRequest.IsStringSystem() {
			systemStr := stripAnthropicBillingHeader(claudeRequest.GetStringSystem())
			if systemStr != "" {
				openAIMessage := dto.Message{Role: "system"}
				openAIMessage.SetStringContent(systemStr)
				openAIMessages = append(openAIMessages, openAIMessage)
			}
		} else {
			systems := claudeRequest.ParseSystem()
			if len(systems) > 0 {
				isOpenRouterClaude := isOpenRouter && strings.HasPrefix(convmeta.UpstreamModelName(info), "anthropic/claude")
				if isOpenRouterClaude {
					systemMediaMessages := make([]dto.MediaContent, 0, len(systems))
					for _, system := range systems {
						text := stripAnthropicBillingHeader(system.GetText())
						if text == "" {
							continue
						}
						systemMediaMessages = append(systemMediaMessages, dto.MediaContent{
							Type:         "text",
							Text:         text,
							CacheControl: system.CacheControl,
						})
					}
					if len(systemMediaMessages) > 0 {
						openAIMessage := dto.Message{Role: "system"}
						openAIMessage.SetMediaContent(systemMediaMessages)
						openAIMessages = append(openAIMessages, openAIMessage)
					}
				} else {
					parts := make([]string, 0, len(systems))
					for _, system := range systems {
						text := stripAnthropicBillingHeader(system.GetText())
						if text != "" {
							parts = append(parts, text)
						}
					}
					if len(parts) > 0 {
						openAIMessage := dto.Message{Role: "system"}
						openAIMessage.SetStringContent(strings.Join(parts, "\n"))
						openAIMessages = append(openAIMessages, openAIMessage)
					}
				}
			}
		}
	}

	preserveReasoning := shouldPreserveReasoningHistory(info, claudeRequest.Model)
	for _, claudeMessage := range claudeRequest.Messages {
		role := claudeMessage.Role
		if role == "" {
			role = "user"
		}
		openAIMessage := dto.Message{Role: role}
		if claudeMessage.IsStringContent() {
			openAIMessage.SetStringContent(claudeMessage.GetStringContent())
		} else if claudeMessage.Content != nil {
			content, err := claudeMessage.ParseContent()
			if err != nil {
				// content 为对象/未知形状时不让整条请求失败，退化成 JSON 文本。
				kitutil.LogError(fmt.Sprintf("claude: message content is not a block array: %v", err))
				openAIMessage.SetStringContent(marshalUnknown(claudeMessage.Content))
				content = nil
			}
			var toolCalls []dto.ToolCallRequest
			var reasoningParts []string
			mediaMessages := make([]dto.MediaContent, 0, len(content))
			toolMediaParts := make([]dto.MediaContent, 0)

			for _, mediaMsg := range content {
				switch mediaMsg.Type {
				case "text", "input_text":
					message := dto.MediaContent{
						Type: "text",
						Text: mediaMsg.GetText(),
					}
					// cache_control 只有 OpenRouter 能识别，其余渠道透传会被严格上游 400。
					if isOpenRouter {
						message.CacheControl = mediaMsg.CacheControl
					}
					mediaMessages = append(mediaMessages, message)
				case "image":
					if mediaMsg.Source == nil {
						kitutil.LogError("claude: image block without source skipped")
						continue
					}
					mediaType := mediaMsg.Source.MediaType
					data := kitutil.Interface2String(mediaMsg.Source.Data)
					if media := buildImageMedia(mediaMsg.Source.Url, mediaType, data, ""); media != nil {
						mediaMessages = append(mediaMessages, *media)
					}
				case "tool_use":
					toolCall := dto.ToolCallRequest{
						ID:   mediaMsg.Id,
						Type: "function",
						Function: dto.FunctionRequest{
							Name:      mediaMsg.Name,
							Arguments: requestToJSONString(mediaMsg.Input),
						},
					}
					toolCalls = append(toolCalls, toolCall)
				case "tool_result":
					toolName := mediaMsg.Name
					if toolName == "" {
						toolName = claudeRequest.SearchToolNameByToolCallId(mediaMsg.ToolUseId)
					}
					oaiToolMessage := dto.Message{
						Role:       "tool",
						Name:       &toolName,
						ToolCallId: mediaMsg.ToolUseId,
					}
					toolContent, mediaParts := planToolResultMedia(mediaMsg.Content)
					oaiToolMessage.SetStringContent(toolContent)
					if len(mediaParts) > 0 {
						toolMediaParts = append(toolMediaParts, dto.MediaContent{
							Type: "text",
							Text: fmt.Sprintf("[tool output media of %s]", mediaMsg.ToolUseId),
						})
						toolMediaParts = append(toolMediaParts, mediaParts...)
					}
					openAIMessages = append(openAIMessages, oaiToolMessage)
				case "thinking":
					if mediaMsg.Thinking != nil && *mediaMsg.Thinking != "" {
						reasoningParts = append(reasoningParts, *mediaMsg.Thinking)
					}
				case "redacted_thinking":
					reasoningParts = append(reasoningParts, "[redacted thinking]")
				case "document", "file":
					label := ""
					if mediaMsg.Source != nil {
						label = mediaMsg.Source.MediaType
					}
					if label == "" {
						label = mediaMsg.Name
					}
					if label == "" {
						label = "document"
					}
					mediaMessages = append(mediaMessages, dto.MediaContent{
						Type: "text",
						Text: "[document: " + label + "]",
					})
				case "audio", "input_audio":
					label := ""
					if mediaMsg.Source != nil {
						label = mediaMsg.Source.MediaType
					}
					if label == "" {
						label = "audio"
					}
					mediaMessages = append(mediaMessages, dto.MediaContent{
						Type: "text",
						Text: "[audio: " + label + "]",
					})
				default:
					kitutil.LogError(fmt.Sprintf("claude: unsupported content block type %q skipped", mediaMsg.Type))
				}
			}

			// tool 消息只能承载文本，抽取出的媒体统一放到主消息之前的一条 user 消息里。
			if len(toolMediaParts) > 0 {
				mediaUserMessage := dto.Message{Role: "user"}
				mediaUserMessage.SetMediaContent(toolMediaParts)
				openAIMessages = append(openAIMessages, mediaUserMessage)
			}

			// 仅 DeepSeek/MiMo 类上游需要在带工具调用的 assistant 消息里回放 reasoning_content。
			if preserveReasoning && role == "assistant" && len(toolCalls) > 0 {
				reasoningText := strings.Join(reasoningParts, "\n")
				if reasoningText == "" {
					reasoningText = "tool call"
				}
				openAIMessage.ReasoningContent = &reasoningText
			}

			if len(toolCalls) > 0 {
				openAIMessage.SetToolCalls(toolCalls)
			}
			if len(mediaMessages) > 0 {
				// 单个 text part 简化为字符串，减少与上游的差异；
				// 带 cache_control 时保持数组，避免丢失 OpenRouter 的缓存断点。
				if len(mediaMessages) == 1 && mediaMessages[0].Type == "text" &&
					mediaMessages[0].ImageUrl == nil && len(mediaMessages[0].CacheControl) == 0 {
					openAIMessage.SetStringContent(mediaMessages[0].Text)
				} else {
					openAIMessage.SetMediaContent(mediaMessages)
				}
			}
		}
		if openAIMessage.Content != nil || len(openAIMessage.ToolCalls) > 0 {
			openAIMessages = append(openAIMessages, openAIMessage)
		}
	}

	openAIRequest.Messages = openAIMessages
	return &openAIRequest, nil
}

// applyOpenRouterReasoning 按 OpenRouter 方言构造 reasoning 配置。
// effort 必须写进 reasoning.effort；顶层 verbosity 与 reasoning 语义无关，不能复用。
func applyOpenRouterReasoning(req *dto.GeneralOpenAIRequest, thinking *dto.Thinking, outputEffort string) error {
	if thinking != nil && thinking.Type == "disabled" {
		reasoningJSON, err := kitutil.Marshal(openRouterRequestReasoning{Enabled: false})
		if err != nil {
			return fmt.Errorf("failed to marshal reasoning: %w", err)
		}
		req.Reasoning = reasoningJSON
		return nil
	}

	effort := reasoning.NormalizeEffortForVendor(reasoning.ResolveEffort(thinking, outputEffort), reasoning.VendorModeOpenRouter)
	if effort == "" && (thinking == nil || (thinking.Type != "enabled" && thinking.Type != "adaptive")) {
		return nil
	}

	reasoningConfig := openRouterRequestReasoning{
		Enabled: true,
		Effort:  effort,
	}
	if thinking != nil && thinking.Type == "enabled" {
		reasoningConfig.MaxTokens = thinking.GetBudgetTokens()
	}
	reasoningJSON, err := kitutil.Marshal(reasoningConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal reasoning: %w", err)
	}
	req.Reasoning = reasoningJSON
	return nil
}

// stripAnthropicBillingHeader 去掉 system 开头的 x-anthropic-billing-header 行。
// 只匹配精确小写的前缀；行尾后最多再跨过一个空行。
func stripAnthropicBillingHeader(text string) string {
	if !strings.HasPrefix(text, anthropicBillingHeaderPrefix) {
		return text
	}
	lineEnd := strings.IndexAny(text, "\r\n")
	if lineEnd < 0 {
		return ""
	}
	rest := text[lineEnd:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else {
		rest = rest[1:]
	}
	// 最多再跳过一行空行。
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, "\n") || strings.HasPrefix(rest, "\r") {
		rest = rest[1:]
	}
	return rest
}

// cleanSchema 清洗工具 JSON Schema：根节点补 type=object/properties={}，
// 并递归 properties/items 删除 format:"uri"（anyOf/oneOf 内的不递归）。
func cleanSchema(schema any) any {
	root, ok := schema.(map[string]any)
	if !ok || root == nil {
		root = map[string]any{}
	}
	if _, ok := root["type"]; !ok {
		root["type"] = "object"
	}
	if _, ok := root["properties"]; !ok {
		root["properties"] = map[string]any{}
	}
	cleanSchemaNode(root)
	return root
}

func cleanSchemaNode(node map[string]any) {
	if format, ok := node["format"].(string); ok && format == "uri" {
		delete(node, "format")
	}
	if properties, ok := node["properties"].(map[string]any); ok {
		for key, value := range properties {
			if child, ok := value.(map[string]any); ok {
				cleanSchemaNode(child)
				properties[key] = child
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		cleanSchemaNode(items)
	}
}

// mapToolChoice 把 Anthropic 的 tool_choice 映射成 OpenAI 形式。
// 返回 (新的 tool_choice, 是否禁用并行工具调用)；无法识别时返回 nil。
func mapToolChoice(choice any) (any, bool) {
	switch value := choice.(type) {
	case string:
		switch value {
		case "any":
			return "required", false
		case "auto", "none":
			return value, false
		}
	case map[string]any:
		typeName, _ := value["type"].(string)
		disableParallel, _ := value["disable_parallel_tool_use"].(bool)
		return toolChoiceByType(typeName, kitutil.Interface2String(value["name"])), disableParallel
	case dto.ClaudeToolChoice:
		return toolChoiceByType(value.Type, value.Name), value.DisableParallelToolUse
	case *dto.ClaudeToolChoice:
		if value != nil {
			return toolChoiceByType(value.Type, value.Name), value.DisableParallelToolUse
		}
	}
	return nil, false
}

func toolChoiceByType(typeName, name string) any {
	switch typeName {
	case "any":
		return "required"
	case "auto":
		return "auto"
	case "none":
		return "none"
	case "tool":
		if name == "" {
			return nil
		}
		return map[string]any{
			"type":     "function",
			"function": map[string]any{"name": name},
		}
	}
	return nil
}

// shouldPreserveReasoningHistory 判断上游是否属于需要回放历史思考的 DeepSeek/MiMo 类。
// convmeta.Meta 暂未暴露 base_url，这里用上游/原始/客户端模型名做判定。
func shouldPreserveReasoningHistory(info convmeta.Meta, model string) bool {
	names := []string{model, convmeta.UpstreamModelName(info)}
	if info != nil {
		names = append(names, info.GetOriginModelName())
	}
	for _, name := range names {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "deepseek") || strings.Contains(lower, "mimo") {
			return true
		}
	}
	return false
}

func requestToJSONString(v interface{}) string {
	b, err := kitutil.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

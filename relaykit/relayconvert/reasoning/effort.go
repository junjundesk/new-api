package reasoning

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/dto"
)

// VendorMode 描述非白名单模型对顶层 reasoning_effort 的处理方式。
type VendorMode int

const (
	// VendorModeConservative 认不出厂商的模型：只发送 low/medium/high。
	VendorModeConservative VendorMode = iota
	// VendorModeDeepSeek DeepSeek 类：档位原样发送。
	VendorModeDeepSeek
	// VendorModeOpenRouter OpenRouter：xhigh/max 统一收敛为 xhigh。
	VendorModeOpenRouter
	// VendorModeOmit 明确不支持该参数的厂商：不下发 reasoning_effort。
	VendorModeOmit
)

// validEfforts 是 OpenAI 兼容上游可能接受的档位集合。
var validEfforts = map[string]struct{}{
	"minimal": {},
	"low":     {},
	"medium":  {},
	"high":    {},
	"xhigh":   {},
	"max":     {},
}

func isValidEffort(effort string) bool {
	_, ok := validEfforts[effort]
	return ok
}

// normalizeModel 去掉厂商前缀（如 openai/gpt-5）并转小写，便于按裸模型名匹配。
func normalizeModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	if i := strings.Index(m, ":"); i >= 0 {
		m = m[:i]
	}
	return m
}

// SupportsReasoningEffort 判断模型是否在白名单内：o 系、gpt-5 及以上、
// grok-4.x（次版本 >= 5）以及 grok-build-*。
func SupportsReasoningEffort(model string) bool {
	m := normalizeModel(model)
	if m == "" {
		return false
	}
	if len(m) >= 2 && m[0] == 'o' && m[1] >= '0' && m[1] <= '9' {
		return true
	}
	if major, ok := gptMajorVersion(m); ok && major >= 5 {
		return true
	}
	if minor, ok := grokMinorVersion(m); ok && minor >= 5 {
		return true
	}
	return strings.HasPrefix(m, "grok-build-")
}

// gptMajorVersion 解析 gpt-<数字> 的主版本号。
func gptMajorVersion(model string) (int, bool) {
	if !strings.HasPrefix(model, "gpt-") {
		return 0, false
	}
	rest := model[len("gpt-"):]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	major, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0, false
	}
	return major, true
}

// grokMinorVersion 解析 grok-4.<次版本> 的次版本号。
func grokMinorVersion(model string) (int, bool) {
	const prefix = "grok-4."
	if !strings.HasPrefix(model, prefix) {
		return 0, false
	}
	rest := model[len(prefix):]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	minor, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0, false
	}
	return minor, true
}

// supportsMaxEffort 判断模型是否接受 "max" 档：gpt-5.6+/gpt-6 裸名，
// 以及 sol/terra/luna/astra 变体（与 cc-switch 的判定一致）。
func supportsMaxEffort(model string) bool {
	m := normalizeModel(model)
	for _, variant := range []string{"sol", "terra", "luna", "astra"} {
		if strings.Contains(m, variant) {
			return true
		}
	}
	if !strings.HasPrefix(m, "gpt-") {
		return false
	}
	main := m[len("gpt-"):]
	if i := strings.Index(main, "-"); i >= 0 {
		main = main[:i]
	}
	parts := strings.SplitN(main, ".", 2)
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	if major >= 6 {
		return true
	}
	if major == 5 && len(parts) == 2 {
		minor, err := strconv.Atoi(parts[1])
		return err == nil && minor >= 6
	}
	return false
}

func isProModel(model string) bool {
	return strings.HasSuffix(model, "-pro") || strings.Contains(model, "-pro-")
}

// ClampEffortForModel 按白名单模型的特性钳制档位：
// gpt-5-pro 只收 high；其他 *-pro 把 low 抬到 medium；不支持 max 的模型把 max 降为 xhigh。
func ClampEffortForModel(effort, model string) string {
	m := normalizeModel(model)
	if strings.HasPrefix(m, "gpt-5-pro") {
		return "high"
	}
	if isProModel(m) && (effort == "low" || effort == "minimal") {
		effort = "medium"
	}
	if effort == "max" && !supportsMaxEffort(m) {
		effort = "xhigh"
	}
	return effort
}

// InferVendorMode 按模型名（可含厂商前缀）推断厂商的 effort 处理方式。
func InferVendorMode(model string) VendorMode {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(m, "openrouter"):
		return VendorModeOpenRouter
	case strings.Contains(m, "deepseek"):
		return VendorModeDeepSeek
	}
	for _, keyword := range []string{
		"kimi", "moonshot", "glm", "zhipu", "qwen", "qwq", "minimax",
		"mimo", "mistral", "siliconflow", "modelscope",
	} {
		if strings.Contains(m, keyword) {
			return VendorModeOmit
		}
	}
	return VendorModeConservative
}

// NormalizeEffortForVendor 把档位收敛到厂商可接受的范围。
func NormalizeEffortForVendor(effort string, mode VendorMode) string {
	if effort == "" {
		return ""
	}
	switch mode {
	case VendorModeOmit:
		return ""
	case VendorModeOpenRouter:
		switch effort {
		case "max", "xhigh":
			return "xhigh"
		default:
			return effort
		}
	case VendorModeDeepSeek:
		if effort == "minimal" {
			return "low"
		}
		return effort
	default:
		switch effort {
		case "xhigh", "max":
			return "high"
		case "minimal":
			return "low"
		default:
			return effort
		}
	}
}

// ResolveEffort 把 Claude 的 output_config.effort / thinking 折算成 OpenAI 档位。
// 返回空串表示不需要下发 reasoning_effort。
func ResolveEffort(thinking *dto.Thinking, outputEffort string) string {
	effort := strings.ToLower(strings.TrimSpace(outputEffort))
	if effort != "" && !isValidEffort(effort) {
		// 未知档位既不下发也不回落到 thinking，避免把上游不认识的取值发出去。
		return ""
	}
	if thinking == nil {
		// thinking 缺失时，显式 effort 仍然有效；没有 effort 则不写。
		return effort
	}
	if thinking.Type == "disabled" {
		if effort == "" {
			return ""
		}
		// 显式要求了 effort 但同时关闭 thinking 时降到 low。
		return "low"
	}
	if effort != "" {
		return effort
	}
	switch thinking.Type {
	case "adaptive":
		return "xhigh"
	case "enabled":
		budget := thinking.GetBudgetTokens()
		switch {
		case budget <= 0:
			return "high"
		case budget < 4000:
			return "low"
		case budget < 16000:
			return "medium"
		default:
			return "high"
		}
	}
	return ""
}

// EffortForModel 综合 thinking/output_config 与上游模型，返回需要写入顶层
// reasoning_effort 的档位；空串表示不下发。
func EffortForModel(thinking *dto.Thinking, outputEffort, upstreamModel string) string {
	effort := ResolveEffort(thinking, outputEffort)
	if effort == "" {
		return ""
	}
	if SupportsReasoningEffort(upstreamModel) {
		return ClampEffortForModel(effort, upstreamModel)
	}
	return NormalizeEffortForVendor(effort, InferVendorMode(upstreamModel))
}

// Package convmeta defines the conversion-context contract between format
// converters (future relaykit) and the hosting application. Converters read
// protocol state and per-request options exclusively through the Meta
// interface; the host's RelayInfo implements it.
package convmeta

import (
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// Meta is the only view of the relay session that format converters may use.
// It is satisfied by *relaycommon.RelayInfo on the host side; other embedders
// (tests, external relaykit users) can use *Values.
// Implementations backed by pointer types must make every method safe on a nil
// receiver: a typed-nil pointer stored in Meta is still a non-nil interface,
// and relaykit deliberately does not use reflection to detect that case.
type Meta interface {
	GetOriginModelName() string
	GetUpstreamModelName() string
	// HasChannelMeta reports whether upstream channel information is attached;
	// converters use it to decide if GetUpstreamModelName is meaningful.
	HasChannelMeta() bool
	GetChannelID() int
	GetChannelType() int
	GetIsStream() bool
	GetReasoningEffort() string
	// SetReasoningEffort records the effort level a converter derived from a
	// model-name suffix so downstream billing/logging can see it.
	SetReasoningEffort(effort string)
	GetEstimatePromptTokens() int

	// EnsureClaudeConvertInfo lazily creates and returns the mutable
	// OpenAI→Claude stream conversion state. For non-nil receivers, the same
	// instance must be returned for the lifetime of one streaming session; a
	// nil receiver may return a temporary initialized state.
	EnsureClaudeConvertInfo() *ClaudeConvertInfo

	// GetSendResponseCount / IncrSendResponseCount expose the shared
	// downstream-chunk counter (the host may also increment it).
	GetSendResponseCount() int
	IncrSendResponseCount()

	// AppendRequestConversion records a hop in the request format chain.
	AppendRequestConversion(format types.RelayFormat)

	// ConvOptions returns the request-scoped conversion options snapshot.
	// Must never return nil.
	ConvOptions() *Options
}

// ToolBlockState 记录一个 OpenAI tool_call 在 Claude 流中的块状态。
// 以 OpenAI tool_call.index 为键维护，AnthropicIndex 按工具首次出现顺序分配，
// 与 tool_call.index 的数值无关。
type ToolBlockState struct {
	AnthropicIndex int
	ID             string
	Name           string
	// Started 表示已经发出过 content_block_start。
	Started bool
	// Closed 表示该块已经发出过 content_block_stop，后续同 index 的增量忽略。
	Closed bool
	// Aborted 表示该工具因上游异常（如无限空白参数）被中止。
	Aborted bool
	// PendingArgs 缓存块启动前到达的参数片段，启动时一次性补发。
	PendingArgs string
	// BlankArgsLen 记录连续空白参数字符的累计长度，用于识别上游无限输出空白的 bug。
	BlankArgsLen int
}

// 流首内联 think 拆分器的三态。
const (
	InlineThinkPhaseDetecting = iota
	InlineThinkPhaseReasoning
	InlineThinkPhaseText
)

// ClaudeConvertInfo carries mutable state for OpenAI chat → Claude Messages
// stream conversion. Moved here from relay/common (which keeps an alias).
type ClaudeConvertInfo struct {
	LastMessagesType string
	Index            int
	Usage            *dto.Usage
	FinishReason     string
	Done             bool

	// LastChunkUsage 记录转换器在 chunk 中看到的上游 usage。
	// Usage 由宿主/状态式管线维护（聚合后的口径），转换器不覆盖它。
	LastChunkUsage *dto.Usage

	// ToolCallBaseIndex / ToolCallMaxIndexOffset 为旧版按区间推导块 index 的字段，
	// 新状态机改用 ToolBlocks + NextBlockIndex，保留字段仅为兼容。
	ToolCallBaseIndex      int
	ToolCallMaxIndexOffset int

	// HasSentMessageStart 标记 message_start 是否已发出，finalize 路径依据它补发。
	HasSentMessageStart bool
	// MessageId / Model 记录上游最后上报的 id 与 model，供 finalize 补发 message_start。
	MessageId string
	Model     string
	// FinishSeen 表示已收到 finish_reason，但终态事件尚未发出（等待 usage）。
	FinishSeen bool
	// NextBlockIndex 是下一个可用的 content block index。
	NextBlockIndex int
	// ToolBlocks 按 OpenAI tool_call.index 维护工具块状态。
	ToolBlocks map[int]*ToolBlockState

	// InlineThinkPhase / InlineThinkBuffer / InlineThinkStripSeparator 是跨 chunk
	// 内联 think 拆分器的状态。
	InlineThinkPhase          int
	InlineThinkBuffer         string
	InlineThinkStripSeparator bool
}

const (
	LastMessageTypeNone     = "none"
	LastMessageTypeText     = "text"
	LastMessageTypeTools    = "tools"
	LastMessageTypeThinking = "thinking"
)

// Values is a plain-struct Meta implementation for tests and non-RelayInfo
// hosts (the relaykit-native entry point).
type Values struct {
	OriginModelName      string
	UpstreamModelName    string
	ChannelMetaAttached  bool
	ChannelID            int
	ChannelType          int
	IsStream             bool
	ReasoningEffort      string
	EstimatePromptTokens int

	ClaudeConvertInfo *ClaudeConvertInfo
	SendResponseCount int
	ConversionChain   []types.RelayFormat

	Options *Options
}

var _ Meta = (*Values)(nil)

func (v *Values) GetOriginModelName() string {
	if v == nil {
		return ""
	}
	return v.OriginModelName
}

func (v *Values) GetUpstreamModelName() string {
	if v == nil {
		return ""
	}
	return v.UpstreamModelName
}

func (v *Values) HasChannelMeta() bool {
	return v != nil && v.ChannelMetaAttached
}

func (v *Values) GetChannelID() int {
	if v == nil {
		return 0
	}
	return v.ChannelID
}

func (v *Values) GetChannelType() int {
	if v == nil {
		return 0
	}
	return v.ChannelType
}

func (v *Values) GetIsStream() bool {
	return v != nil && v.IsStream
}

func (v *Values) GetReasoningEffort() string {
	if v == nil {
		return ""
	}
	return v.ReasoningEffort
}

func (v *Values) SetReasoningEffort(effort string) {
	if v != nil {
		v.ReasoningEffort = effort
	}
}

func (v *Values) GetEstimatePromptTokens() int {
	if v == nil {
		return 0
	}
	return v.EstimatePromptTokens
}

func (v *Values) EnsureClaudeConvertInfo() *ClaudeConvertInfo {
	if v == nil {
		return &ClaudeConvertInfo{LastMessagesType: LastMessageTypeNone}
	}
	if v.ClaudeConvertInfo == nil {
		v.ClaudeConvertInfo = &ClaudeConvertInfo{LastMessagesType: LastMessageTypeNone}
	}
	return v.ClaudeConvertInfo
}

func (v *Values) GetSendResponseCount() int {
	if v == nil {
		return 0
	}
	return v.SendResponseCount
}

func (v *Values) IncrSendResponseCount() {
	if v != nil {
		v.SendResponseCount++
	}
}

func (v *Values) AppendRequestConversion(format types.RelayFormat) {
	if v == nil || format == "" {
		return
	}
	if n := len(v.ConversionChain); n > 0 && v.ConversionChain[n-1] == format {
		return
	}
	v.ConversionChain = append(v.ConversionChain, format)
}

func (v *Values) ConvOptions() *Options {
	if v == nil {
		return &Options{}
	}
	if v.Options == nil {
		v.Options = &Options{}
	}
	return v.Options
}

// UpstreamModelName / ChannelTypeOf are nil-safe accessors for optional Meta
// values (converters are often called with a nil Meta in tests and compat
// shims).
func UpstreamModelName(m Meta) string {
	if m == nil || !m.HasChannelMeta() {
		return ""
	}
	return m.GetUpstreamModelName()
}

func ChannelTypeOf(m Meta) int {
	if m == nil || !m.HasChannelMeta() {
		return 0
	}
	return m.GetChannelType()
}

// OptionsOf returns m's conversion options, or empty defaults when m is nil.
func OptionsOf(m Meta) *Options {
	if m == nil {
		return &Options{}
	}
	return m.ConvOptions()
}

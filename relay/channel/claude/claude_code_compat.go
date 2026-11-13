package claude

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
)

// claudeCodeCompatSystemPrompt 是 Claude Code 客户端固定的身份声明，仅限
// Claude Code 的上游会校验 system 中是否包含该文本。
const claudeCodeCompatSystemPrompt = "You are Claude Code, Anthropic's official CLI for Claude."

// claudeCodeCompatUserIDPattern 匹配 Claude Code 的 metadata.user_id 格式：
// user_<64 位十六进制>_account_<UUID>_session_<UUID>。
var claudeCodeCompatUserIDPattern = regexp.MustCompile(
	`^user_[0-9a-f]{64}_account_[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}` +
		`_session_[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
)

// applyClaudeCodeCompat 为仅接受 Claude Code 流量的上游补齐请求指纹：
// metadata.user_id 与包含 Claude Code 身份声明的 system 数组。
func applyClaudeCodeCompat(request *dto.ClaudeRequest, info *relaycommon.RelayInfo) {
	if request == nil {
		return
	}
	applyClaudeCodeCompatSystem(request)
	applyClaudeCodeCompatMetadata(request, info)
}

func applyClaudeCodeCompatSystem(request *dto.ClaudeRequest) {
	if text, ok := request.System.(string); ok {
		request.System = []dto.ClaudeMediaMessage{claudeCodeCompatSystemBlock(), {
			Type: "text",
			Text: common.GetPointer(text),
		}}
		return
	}
	if request.System == nil {
		request.System = []dto.ClaudeMediaMessage{claudeCodeCompatSystemBlock()}
		return
	}
	blocks, err := kitutil.Any2Type[[]dto.ClaudeMediaMessage](request.System)
	if err != nil || len(blocks) == 0 {
		request.System = []dto.ClaudeMediaMessage{claudeCodeCompatSystemBlock()}
		return
	}
	for _, block := range blocks {
		if block.Type == "text" && block.Text != nil && strings.Contains(*block.Text, claudeCodeCompatSystemPrompt) {
			return
		}
	}
	request.System = append([]dto.ClaudeMediaMessage{claudeCodeCompatSystemBlock()}, blocks...)
}

func applyClaudeCodeCompatMetadata(request *dto.ClaudeRequest, info *relaycommon.RelayInfo) {
	var metadata struct {
		UserID string `json:"user_id"`
	}
	if err := common.Unmarshal(request.Metadata, &metadata); err == nil &&
		claudeCodeCompatUserIDPattern.MatchString(metadata.UserID) {
		return
	}

	encoded, err := common.Marshal(map[string]string{"user_id": claudeCodeCompatUserID(info)})
	if err != nil {
		return
	}
	request.Metadata = encoded
}

func claudeCodeCompatSystemBlock() dto.ClaudeMediaMessage {
	return dto.ClaudeMediaMessage{
		Type:         "text",
		Text:         common.GetPointer(claudeCodeCompatSystemPrompt),
		CacheControl: []byte(`{"type":"ephemeral"}`),
	}
}

// claudeCodeCompatUserID 生成与用户/令牌绑定的稳定 Claude Code user_id，
// 让上游看到的会话身份在多次请求之间保持一致。
func claudeCodeCompatUserID(info *relaycommon.RelayInfo) string {
	userID, tokenID := 0, 0
	if info != nil {
		userID = info.UserId
		tokenID = info.TokenId
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("new-api:claude-code:%d:%d", userID, tokenID)))
	return "user_" + hex.EncodeToString(sum[:]) +
		"_account_" + formatUUIDv4(sum[0:16]) +
		"_session_" + formatUUIDv4(sum[16:32])
}

func formatUUIDv4(raw []byte) string {
	buf := make([]byte, 16)
	copy(buf, raw)
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

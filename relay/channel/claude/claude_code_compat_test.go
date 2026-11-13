package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyClaudeCodeCompatSystem(t *testing.T) {
	tests := []struct {
		name       string
		system     any
		wantLength int
		wantFirst  string
		wantSecond string
	}{
		{
			name:       "missing system gets claude code identity",
			wantLength: 1,
			wantFirst:  claudeCodeCompatSystemPrompt,
		},
		{
			name:       "plain string system is wrapped into text blocks",
			system:     "custom system prompt",
			wantLength: 2,
			wantFirst:  claudeCodeCompatSystemPrompt,
			wantSecond: "custom system prompt",
		},
		{
			name: "existing text block keeps claude code identity first",
			system: []dto.ClaudeMediaMessage{
				{Type: "text", Text: common.GetPointer("custom system prompt")},
			},
			wantLength: 2,
			wantFirst:  claudeCodeCompatSystemPrompt,
			wantSecond: "custom system prompt",
		},
		{
			name: "system already carrying claude code identity is untouched",
			system: []dto.ClaudeMediaMessage{
				{Type: "text", Text: common.GetPointer(claudeCodeCompatSystemPrompt)},
				{Type: "text", Text: common.GetPointer("custom system prompt")},
			},
			wantLength: 2,
			wantFirst:  claudeCodeCompatSystemPrompt,
			wantSecond: "custom system prompt",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := &dto.ClaudeRequest{System: test.system}
			applyClaudeCodeCompatSystem(request)

			encoded, err := common.Marshal(request.System)
			require.NoError(t, err)
			var blocks []dto.ClaudeMediaMessage
			require.NoError(t, common.Unmarshal(encoded, &blocks))
			require.Len(t, blocks, test.wantLength)
			require.NotNil(t, blocks[0].Text)
			assert.Equal(t, test.wantFirst, *blocks[0].Text)
			if test.wantSecond != "" {
				require.NotNil(t, blocks[1].Text)
				assert.Equal(t, test.wantSecond, *blocks[1].Text)
			}
		})
	}
}

func TestApplyClaudeCodeCompatMetadata(t *testing.T) {
	info := &relaycommon.RelayInfo{UserId: 7, TokenId: 11}

	t.Run("generates stable claude code user id", func(t *testing.T) {
		first := &dto.ClaudeRequest{}
		applyClaudeCodeCompatMetadata(first, info)
		second := &dto.ClaudeRequest{}
		applyClaudeCodeCompatMetadata(second, info)

		var firstMetadata, secondMetadata struct {
			UserID string `json:"user_id"`
		}
		require.NoError(t, common.Unmarshal(first.Metadata, &firstMetadata))
		require.NoError(t, common.Unmarshal(second.Metadata, &secondMetadata))
		assert.Regexp(t, claudeCodeCompatUserIDPattern, firstMetadata.UserID)
		assert.Equal(t, firstMetadata.UserID, secondMetadata.UserID)
	})

	t.Run("different tokens get different user ids", func(t *testing.T) {
		otherInfo := &relaycommon.RelayInfo{UserId: 7, TokenId: 12}
		first := &dto.ClaudeRequest{}
		applyClaudeCodeCompatMetadata(first, info)
		second := &dto.ClaudeRequest{}
		applyClaudeCodeCompatMetadata(second, otherInfo)
		assert.NotEqual(t, string(first.Metadata), string(second.Metadata))
	})

	t.Run("valid client user id is preserved", func(t *testing.T) {
		const clientUserID = "user_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef_account_0123abcd-4567-489a-8bcd-ef0123456789_session_fedcba98-7654-4321-8fed-cba987654321"
		request := &dto.ClaudeRequest{}
		metadata, err := common.Marshal(map[string]string{"user_id": clientUserID})
		require.NoError(t, err)
		request.Metadata = metadata

		applyClaudeCodeCompatMetadata(request, info)

		var got struct {
			UserID string `json:"user_id"`
		}
		require.NoError(t, common.Unmarshal(request.Metadata, &got))
		assert.Equal(t, clientUserID, got.UserID)
	})

	t.Run("invalid client user id is replaced", func(t *testing.T) {
		request := &dto.ClaudeRequest{Metadata: []byte(`{"user_id":"pi-agent"}`)}

		applyClaudeCodeCompatMetadata(request, info)

		var got struct {
			UserID string `json:"user_id"`
		}
		require.NoError(t, common.Unmarshal(request.Metadata, &got))
		assert.Regexp(t, claudeCodeCompatUserIDPattern, got.UserID)
	})
}

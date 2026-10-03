package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsNonStreamingForbidden(t *testing.T) {
	require.NoError(t, UpdateTopupGroupRatioByJSONString(`{"blocked":0,"normal":1,"svip":2}`))
	t.Cleanup(func() {
		require.NoError(t, UpdateTopupGroupRatioByJSONString(`{"default":1,"vip":1,"svip":1}`))
	})

	assert.True(t, IsNonStreamingForbidden("blocked"))
	assert.False(t, IsNonStreamingForbidden("normal"))
	assert.False(t, IsNonStreamingForbidden("svip"))
	assert.False(t, IsNonStreamingForbidden("unknown-group"))
}

func TestGetTopupGroupRatioKeepsStoredValue(t *testing.T) {
	require.NoError(t, UpdateTopupGroupRatioByJSONString(`{"blocked":0,"normal":1,"svip":2}`))
	t.Cleanup(func() {
		require.NoError(t, UpdateTopupGroupRatioByJSONString(`{"default":1,"vip":1,"svip":1}`))
	})

	assert.Equal(t, float64(0), GetTopupGroupRatio("blocked"))
	assert.Equal(t, float64(1), GetTopupGroupRatio("normal"))
	assert.Equal(t, float64(2), GetTopupGroupRatio("svip"))
	// 未配置的分组回退为 1，与支付侧历史行为保持一致
	assert.Equal(t, float64(1), GetTopupGroupRatio("unknown-group"))
}

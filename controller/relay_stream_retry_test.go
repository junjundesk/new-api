package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseURLFailoverOnlyBeforeStreamOutput(t *testing.T) {
	prior := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true // pinning reads the channel cache, not a database
	t.Cleanup(func() { common.MemoryCacheEnabled = prior })
	ch := &model.Channel{Id: 920117, BaseURLs: []string{"https://first.example", "https://backup.example"}}
	t.Cleanup(func() { service.InvalidateChannelBaseURLs(ch.Id) })
	current := service.SelectChannelBaseURL(ch)
	require.Contains(t, ch.BaseURLs, current)
	for _, tc := range []struct {
		name                         string
		received, written, cancelled bool
	}{
		{"upstream already emitted data", true, false, false},
		{"downstream already flushed", false, true, false},
		{"client left before first frame", false, false, true},
		{"failure before any output", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: current}}
			if tc.received {
				info.ReceivedResponseCount = 1
			}
			if tc.written {
				c.Writer.Flush()
			}
			if tc.cancelled {
				ctx, cancel := context.WithCancel(c.Request.Context())
				c.Request = c.Request.WithContext(ctx)
				cancel()
			}
			apiErr := types.NewError(errors.New("upstream timeout"), types.ErrorCodeChannelResponseTimeExceeded)
			param := &service.RetryParam{Ctx: c, Retry: common.GetPointer(0)}
			retry := tryFailoverChannelBaseURL(c, info, ch, apiErr, param)
			assert.Equal(t, !tc.received && !tc.written && !tc.cancelled, retry)
		})
	}
}

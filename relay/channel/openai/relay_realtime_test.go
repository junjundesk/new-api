package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dialTestWebsocketPair 建立一对可用的 websocket 连接：返回测试侧连接与服务侧连接。
func dialTestWebsocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	serverConnCh := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnCh <- conn
	}))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientConn.Close() })
	serverConn := <-serverConnCh
	t.Cleanup(func() { _ = serverConn.Close() })
	return clientConn, serverConn
}

// TestOpenaiRealtimeHandlerReturnsOnRequestCancel 验证长流会话的生命周期绑定在
// 请求 context 上：客户端断开/失租取消请求后 handler 必须结束并完成收尾，
// 不依赖 gin.Context.Done()（未开启 ContextWithFallback 时它永远是 nil channel）。
func TestOpenaiRealtimeHandlerReturnsOnRequestCancel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clientWs, clientPeer := dialTestWebsocketPair(t)
	targetWs, targetPeer := dialTestWebsocketPair(t)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	c.Request = req.WithContext(ctx)

	// gin 未开启 ContextWithFallback 时 c.Done() 恒为 nil：这正是历史上
	// realtime 会话不会随请求结束而退出的原因。
	assert.Nil(t, c.Done(), "前提：本测试覆盖的正是 c.Done() 不可用的场景")
	require.NotNil(t, c.Request.Context().Done())

	info := &relaycommon.RelayInfo{ClientWs: clientWs, TargetWs: targetWs}
	done := make(chan struct{})
	go func() {
		defer close(done)
		apiErr, usage := OpenaiRealtimeHandler(c, info)
		assert.Nil(t, apiErr)
		assert.NotNil(t, usage)
	}()

	// 会话进行中：取消请求 context（客户端断开/并发槽失租都走这条路）。
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("请求取消后 realtime handler 必须返回")
	}

	// 收尾必须关闭两端连接：对端读取立刻失败，不残留仍在读写的半开连接。
	require.NoError(t, clientPeer.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err := clientPeer.ReadMessage()
	assert.Error(t, err, "handler 收尾必须关闭客户端连接")
	require.NoError(t, targetPeer.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err = targetPeer.ReadMessage()
	assert.Error(t, err, "handler 收尾必须关闭上游连接")
}

package common_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamTimingSeparatesAttemptsAndVisibleText(t *testing.T) {
	old := constant.StreamTimingSamplePercent
	constant.StreamTimingSamplePercent = 100
	t.Cleanup(func() { constant.StreamTimingSamplePercent = old })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(upstream.Close)
	for _, tc := range []struct {
		format types.RelayFormat
		prefix []string
		text   string
	}{
		{types.RelayFormatOpenAI, []string{`{"choices":[{"delta":{"role":"assistant","content":""}}]}`, `{"choices":[{"delta":{"tool_calls":[{}],"reasoning_content":"thinking"}}]}`}, `{"choices":[{"delta":{"content":"private-output"}}]}`},
		{types.RelayFormatOpenAIResponses, []string{`{"type":"response.created"}`, `{"type":"response.reasoning_summary_text.delta","delta":"thinking"}`, `{"type":"response.function_call_arguments.delta","delta":"{}"}`}, `{"type":"response.output_text.delta","delta":"private-output"}`},
		{types.RelayFormatClaude, []string{`{"type":"message_start"}`, `{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"thinking"}}`}, `{"type":"content_block_delta","delta":{"type":"text_delta","text":"private-output"}}`},
		{types.RelayFormatGemini, []string{`{"candidates":[{"content":{"parts":[{"thought":true,"text":"thinking"},{"functionCall":{"name":"f"}}]}}]}`}, `{"candidates":[{"content":{"parts":[{"text":"private-output"}]}}]}`},
	} {
		t.Run(string(tc.format), func(t *testing.T) {
			router := gin.New()
			router.Use(relaycommon.SampleStreamIngress(100))
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				first := relaycommon.BeginStreamAttempt(c, types.RelayFormatOpenAI)
				require.NotNil(t, first)
				req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, upstream.URL, nil)
				require.NoError(t, err)
				client := &http.Client{Transport: &http.Transport{}}
				t.Cleanup(client.CloseIdleConnections)
				resp, err := client.Do(first.TraceDispatch(req))
				require.NoError(t, err)
				first.ResponseHeaders()
				_, err = io.Copy(io.Discard, resp.Body)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
				for _, data := range tc.prefix {
					first.ObserveReceived(tc.format, data)
				}
				first.ObserveFlushed(`{"choices":[{"delta":{"content":""}}]}`)
				noOutput := first.Summary(7, 502)
				assert.Contains(t, noOutput, "attempt=1 channel=7")
				assert.Regexp(t, `dispatch_to_conn_us=\d+ conn_reused=0 dispatch_to_headers_us=\d+`, noOutput)
				assert.Contains(t, noOutput, "dispatch_to_text_us=-1 text_to_flush_us=-1 ingress_to_flush_us=-1")

				second := relaycommon.BeginStreamAttempt(c, types.RelayFormatOpenAI)
				require.NotNil(t, second)
				resp, err = client.Do(second.TraceDispatch(req))
				require.NoError(t, err)
				second.ResponseHeaders()
				require.NoError(t, resp.Body.Close())
				second.ObserveReceived(tc.format, tc.text)
				pending := second.Summary(8, 200)
				assert.Regexp(t, `dispatch_to_text_us=\d+ text_to_flush_us=-1 ingress_to_flush_us=-1`, pending)
				second.ObserveFlushed(`{"choices":[{"delta":{"content":"private-output"}}]}`)
				finished := second.Summary(8, 200)
				assert.Contains(t, finished, "attempt=2 channel=8")
				assert.Contains(t, finished, "conn_reused=1")
				assert.Regexp(t, `text_to_flush_us=\d+ ingress_to_flush_us=\d+`, finished)
				assert.NotContains(t, finished, "private-output")
				second.ObserveReceived(tc.format, tc.text)
				second.ObserveFlushed(`{"choices":[{"delta":{"content":"later"}}]}`)
				assert.Equal(t, finished, second.Summary(8, 200), "later chunks cannot change TTFT")
				assert.Equal(t, noOutput, first.Summary(7, 502))
			})
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
		})
	}
}

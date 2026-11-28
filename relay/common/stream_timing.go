package common

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const streamIngressKey = "relay_stream_ingress_time"
const streamTimingKey = "relay_stream_attempt_timing"
const streamStagesKey = "relay_stream_ingress_stages"

// Pre-dispatch stages, measured from ingress and shared by every attempt.
const (
	IngressStageParsed = iota
	IngressStageTokens
	IngressStageBilled
	ingressStageCount
)

type ingressStages [ingressStageCount]atomic.Int64

// MarkIngressStage records the first time a sampled request reaches a stage.
// Unsampled requests return immediately.
func MarkIngressStage(c *gin.Context, stage int) {
	if constant.StreamTimingSamplePercent == 0 || c == nil {
		return
	}
	ingress := c.GetTime(streamIngressKey)
	if ingress.IsZero() {
		return
	}
	value, _ := c.Get(streamStagesKey)
	stages, _ := value.(*ingressStages)
	if stages == nil {
		stages = &ingressStages{}
		for i := range stages {
			stages[i].Store(-1)
		}
		c.Set(streamStagesKey, stages)
	}
	stages[stage].CompareAndSwap(-1, int64(time.Since(ingress)))
}

// SampleStreamIngress runs before body decoding, auth and distribution. Do not
// move the legacy request_start_time: it also feeds historical metrics/routing.
func SampleStreamIngress(percent int) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.FullPath() {
		case "/v1/chat/completions", "/pg/chat/completions", "/v1/responses", "/v1/messages", "/v1/models/*path", "/v1beta/models/*path":
			if c.Request.Method == http.MethodPost && (percent == 100 || rand.IntN(100) < percent) {
				c.Set(streamIngressKey, time.Now())
			}
		}
		c.Next()
	}
}

// One instance per attempt (including candidate, base URL and group fallbacks).
// All timestamps are monotonic offsets from the same ingress clock. -1 denotes
// an unobserved event; the scanner and downstream writer run independently.
type StreamAttemptTiming struct {
	ingress    time.Time
	attempt    int
	downstream types.RelayFormat
	start      time.Duration
	dispatch   time.Duration
	headers    time.Duration
	gotConn    atomic.Int64
	reused     atomic.Int32
	received   atomic.Int64
	flushed    atomic.Int64
	stages     *ingressStages
	done       time.Duration
}

func BeginStreamAttempt(c *gin.Context, format types.RelayFormat) *StreamAttemptTiming {
	ingress := c.GetTime(streamIngressKey)
	if ingress.IsZero() {
		return nil
	}
	attempt := 1
	if prev := CurrentStreamAttempt(c); prev != nil {
		attempt = prev.attempt + 1
	}
	t := &StreamAttemptTiming{ingress: ingress, attempt: attempt, downstream: format, start: time.Since(ingress), dispatch: -1, headers: -1, done: -1}
	t.gotConn.Store(-1)
	t.received.Store(-1)
	t.flushed.Store(-1)
	t.reused.Store(-1)
	stagesValue, _ := c.Get(streamStagesKey)
	t.stages, _ = stagesValue.(*ingressStages)
	c.Set(streamTimingKey, t)
	return t
}

func CurrentStreamAttempt(c *gin.Context) *StreamAttemptTiming {
	if constant.StreamTimingSamplePercent == 0 || c == nil {
		return nil
	}
	value, _ := c.Get(streamTimingKey)
	t, _ := value.(*StreamAttemptTiming)
	return t
}

func (t *StreamAttemptTiming) TraceDispatch(req *http.Request) *http.Request {
	t.dispatch = time.Since(t.ingress)
	trace := &httptrace.ClientTrace{GotConn: func(conn httptrace.GotConnInfo) {
		// A transport retry/redirect can acquire another connection. Report the
		// initial acquisition; it is the one the dispatch duration starts at.
		if t.gotConn.CompareAndSwap(-1, int64(time.Since(t.ingress))) {
			if conn.Reused {
				t.reused.Store(1)
			} else {
				t.reused.Store(0)
			}
		}
	}}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
}

func (t *StreamAttemptTiming) ResponseHeaders() { t.headers = time.Since(t.ingress) }

// Finish marks the end of one relay attempt. For non-stream requests, the span
// from response headers to here covers body read, parsing, conversion and write.
func (t *StreamAttemptTiming) Finish() { t.done = time.Since(t.ingress) }

func (t *StreamAttemptTiming) ObserveReceived(format types.RelayFormat, data string) {
	t.observeText(&t.received, format, data)
}

func (t *StreamAttemptTiming) ObserveFlushed(data string) {
	t.observeText(&t.flushed, t.downstream, data)
}

func (t *StreamAttemptTiming) observeText(mark *atomic.Int64, format types.RelayFormat, data string) {
	if mark.Load() >= 0 {
		return
	}
	at := time.Since(t.ingress)
	if streamHasText(format, data) {
		mark.CompareAndSwap(-1, int64(at))
	}
}

// Classify the wire event, not the first HTTP byte or the first SSE data line.
// Whitespace is text; lifecycle, reasoning, tool and usage-only events are not.
func streamHasText(format types.RelayFormat, data string) bool {
	if !gjson.Valid(data) {
		return false
	}
	event := gjson.Parse(data)
	switch format {
	case types.RelayFormatOpenAI:
		for _, choice := range event.Get("choices").Array() {
			if choice.Get("delta.content").Str != "" {
				return true
			}
		}
	case types.RelayFormatOpenAIResponses:
		return event.Get("type").Str == "response.output_text.delta" && event.Get("delta").Str != ""
	case types.RelayFormatClaude:
		switch event.Get("type").Str {
		case "content_block_delta":
			return event.Get("delta.type").Str == "text_delta" && event.Get("delta.text").Str != ""
		case "content_block_start":
			return event.Get("content_block.type").Str == "text" && event.Get("content_block.text").Str != ""
		}
	case types.RelayFormatGemini:
		for _, candidate := range event.Get("candidates").Array() {
			for _, part := range candidate.Get("content.parts").Array() {
				if !part.Get("thought").Bool() && part.Get("text").Str != "" {
					return true
				}
			}
		}
	}
	return false
}

// Missing stages must never look like fast (zero-duration) responses.
func streamStageUS(start, end time.Duration) int64 {
	if start < 0 || end < start {
		return -1
	}
	return (end - start).Microseconds()
}

func (t *StreamAttemptTiming) Summary(channelID, status int) string {
	received, flushed := time.Duration(t.received.Load()), time.Duration(t.flushed.Load())
	parsed, tokens, billed := int64(-1), int64(-1), int64(-1)
	if s := t.stages; s != nil {
		parsedAt, tokensAt, billedAt := time.Duration(s[IngressStageParsed].Load()), time.Duration(s[IngressStageTokens].Load()), time.Duration(s[IngressStageBilled].Load())
		parsed, tokens, billed = streamStageUS(0, parsedAt), streamStageUS(parsedAt, tokensAt), streamStageUS(tokensAt, billedAt)
	}
	return fmt.Sprintf("relay_stream_timing attempt=%d channel=%d format=%s status=%d ingress_to_dispatch_us=%d attempt_to_dispatch_us=%d dispatch_to_conn_us=%d conn_reused=%d dispatch_to_headers_us=%d dispatch_to_text_us=%d text_to_flush_us=%d ingress_to_flush_us=%d ingress_to_parsed_us=%d parsed_to_tokens_us=%d tokens_to_billed_us=%d headers_to_done_us=%d dispatch_to_done_us=%d",
		t.attempt, channelID, t.downstream, status, streamStageUS(0, t.dispatch), streamStageUS(t.start, t.dispatch),
		streamStageUS(t.dispatch, time.Duration(t.gotConn.Load())), t.reused.Load(), streamStageUS(t.dispatch, t.headers),
		streamStageUS(t.dispatch, received), streamStageUS(received, flushed), streamStageUS(0, flushed),
		parsed, tokens, billed, streamStageUS(t.headers, t.done), streamStageUS(t.dispatch, t.done))
}

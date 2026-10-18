package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const maxResponseBytes = 256 << 10

// Invoke calls a catalog route through the real Gin engine in-process. It never
// opens a network connection and never bypasses route middleware.
func Invoke(ctx context.Context, actor Actor, op Operation, pathParams, query map[string]any, body any) Result {
	if engine == nil {
		return Result{Success: false, Message: "assistant operation engine is not initialized"}
	}
	path, err := renderPath(op, pathParams)
	if err != nil {
		return Result{Success: false, Message: err.Error()}
	}
	target := path
	if len(query) > 0 {
		values := url.Values{}
		for key, value := range query {
			values.Set(key, fmt.Sprint(value))
		}
		target += "?" + values.Encode()
	}
	var bodyReader io.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			return Result{Success: false, Message: "invalid operation body"}
		}
		bodyReader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, op.Method, target, bodyReader)
	if err != nil {
		return Result{Success: false, Message: "invalid operation request"}
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if actor.IP != "" {
		request.RemoteAddr = actor.IP
	}
	request.Header.Set("X-Assistant-Internal", "true")
	request = request.WithContext(WithActor(request.Context(), actor))
	operationCtx, cancel := context.WithTimeout(request.Context(), 30*time.Second)
	defer cancel()
	request = request.WithContext(operationCtx)

	recorder := newCappedRecorder(maxResponseBytes)
	engine.ServeHTTP(recorder, request)
	return normalizeResult(recorder)
}

func renderPath(op Operation, params map[string]any) (string, error) {
	path := op.Path
	for _, name := range op.PathParams {
		value, ok := params[name]
		if !ok || value == nil || fmt.Sprint(value) == "" {
			return "", fmt.Errorf("missing path parameter: %s", name)
		}
		text := fmt.Sprint(value)
		if text == "." || text == ".." || strings.ContainsAny(text, "/?#") {
			return "", fmt.Errorf("path parameter %s contains invalid characters", name)
		}
		path = strings.ReplaceAll(path, ":"+name, url.PathEscape(text))
	}
	return path, nil
}

func normalizeResult(recorder *cappedRecorder) Result {
	body := recorder.Bytes()
	status := recorder.StatusCode()
	result := Result{StatusCode: status, Success: status >= 200 && status < 300, Truncated: recorder.Truncated()}
	envelope := struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}{}
	if err := common.Unmarshal(body, &envelope); err == nil && (envelope.Success || envelope.Message != "" || len(envelope.Data) > 0) {
		result.Success = envelope.Success
		result.Message = envelope.Message
		if len(envelope.Data) > 0 {
			var data any
			if err := common.Unmarshal(envelope.Data, &data); err == nil {
				result.Data = data
			}
		}
		return result
	}
	var data any
	if err := common.Unmarshal(body, &data); err == nil {
		result.Data = data
		if status >= 200 && status < 300 {
			result.Success = true
		}
		return result
	}
	result.Raw = string(body)
	return result
}

type cappedRecorder struct {
	header    http.Header
	buf       bytes.Buffer
	status    int
	truncated bool
	limit     int
}

func newCappedRecorder(limit int) *cappedRecorder {
	return &cappedRecorder{header: make(http.Header), status: http.StatusOK, limit: limit}
}

func (r *cappedRecorder) Header() http.Header { return r.header }
func (r *cappedRecorder) WriteHeader(status int) {
	if r.status == http.StatusOK {
		r.status = status
	}
}
func (r *cappedRecorder) Write(payload []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	remaining := r.limit - r.buf.Len()
	if remaining <= 0 {
		r.truncated = true
		return len(payload), nil
	}
	if len(payload) > remaining {
		r.truncated = true
		payload = payload[:remaining]
	}
	_, _ = r.buf.Write(payload)
	return len(payload), nil
}
func (r *cappedRecorder) Flush()          {}
func (r *cappedRecorder) Bytes() []byte   { return r.buf.Bytes() }
func (r *cappedRecorder) StatusCode() int { return r.status }
func (r *cappedRecorder) Truncated() bool { return r.truncated }

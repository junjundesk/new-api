/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package common

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRequestSource(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		forwarded  string
		origin     string
		referer    string
		wantHost   string
		wantOrigin string
	}{
		{
			name:     "host header only",
			host:     "api.example.com",
			wantHost: "api.example.com",
		},
		{
			name:      "forwarded host wins over host and keeps first hop",
			host:      "internal:3000",
			forwarded: "Public.Example.com:8443, edge.internal",
			wantHost:  "public.example.com:8443",
		},
		{
			name:       "origin differs from host",
			host:       "api.example.com",
			origin:     "https://chat.example.com",
			wantHost:   "api.example.com",
			wantOrigin: "chat.example.com",
		},
		{
			name:     "origin equal to host is dropped",
			host:     "api.example.com",
			origin:   "https://API.example.com",
			wantHost: "api.example.com",
		},
		{
			name:       "referer used when origin missing",
			host:       "api.example.com",
			referer:    "https://chat.example.com/conversation/42?x=1",
			wantHost:   "api.example.com",
			wantOrigin: "chat.example.com",
		},
		{
			name:     "opaque origin is ignored",
			host:     "api.example.com",
			origin:   "null",
			wantHost: "api.example.com",
		},
		{
			name:     "malformed host is rejected",
			host:     "<script>alert(1)</script>",
			wantHost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
			req.Host = tt.host
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-Host", tt.forwarded)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}

			info := ResolveRequestSource(req)
			assert.Equal(t, tt.wantHost, info.Host)
			assert.Equal(t, tt.wantOrigin, info.Origin)
		})
	}
}

func TestAttachRequestSourceToOther(t *testing.T) {
	t.Run("writes structured source", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Host = "api.example.com"
		other := map[string]interface{}{}

		AttachRequestSourceToOther(other, req)

		source, ok := other["request_source"].(RequestSourceInfo)
		require.True(t, ok, "request_source must be a RequestSourceInfo")
		assert.Equal(t, "api.example.com", source.Host)
	})

	t.Run("keeps the first recorded value", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Host = "api.example.com"
		other := map[string]interface{}{
			"request_source": RequestSourceInfo{Host: "first.example.com"},
		}

		AttachRequestSourceToOther(other, req)

		source, ok := other["request_source"].(RequestSourceInfo)
		require.True(t, ok)
		assert.Equal(t, "first.example.com", source.Host)
	})

	t.Run("skips nil request and empty host", func(t *testing.T) {
		other := map[string]interface{}{}
		AttachRequestSourceToOther(other, nil)
		assert.NotContains(t, other, "request_source")

		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Host = ""
		AttachRequestSourceToOther(other, req)
		assert.NotContains(t, other, "request_source")
	})

	t.Run("no key for nil map", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Host = "api.example.com"
		assert.NotPanics(t, func() { AttachRequestSourceToOther(nil, req) })
	})
}

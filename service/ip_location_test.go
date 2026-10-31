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
package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatIPLocation(t *testing.T) {
	tests := []struct {
		name    string
		country string
		local   string
		want    string
	}{
		{
			name:    "dash separated region becomes spaces",
			country: "中国–广东–广州",
			local:   "联通",
			want:    "中国 广东 广州 联通",
		},
		{
			name:    "tabs and repeated spaces collapse",
			country: "美国–加利福尼亚州–圣克拉拉",
			local:   "谷歌公司DNS服务器\t\t骨干网",
			want:    "美国 加利福尼亚州 圣克拉拉 谷歌公司DNS服务器 骨干网",
		},
		{
			name:    "empty local keeps country only",
			country: "中国",
			local:   "",
			want:    "中国",
		},
		{
			name: "both empty",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatIPLocation(tt.country, tt.local))
		})
	}
}

func TestGetIPLocationSkipsNonPublicIP(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"code":0,"data":{"ip":{"query":"1.1.1.1"},"country":"a","local":"b"}}`))
	}))
	defer server.Close()
	t.Setenv("IP_LOCATION_API", server.URL+"/?ip=%s")

	for _, ip := range []string{"", "not-an-ip", "127.0.0.1", "10.1.2.3", "192.168.1.1", "::1", "fe80::1"} {
		assert.Equal(t, "", GetIPLocation(ip), "ip %q must not resolve", ip)
	}
	assert.Equal(t, int32(0), hits.Load(), "local addresses must not hit the upstream API")
}

func TestGetIPLocationFetchesAndCaches(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		assert.Equal(t, "203.0.113.9", r.URL.Query().Get("ip"))
		_, _ = w.Write([]byte(`{"code":0,"data":{"ip":{"query":"203.0.113.9"},"country":"中国–广东–广州","local":"联通"}}`))
	}))
	defer server.Close()
	t.Setenv("IP_LOCATION_API", server.URL+"/?ip=%s")

	assert.Equal(t, "中国 广东 广州 联通", GetIPLocation("203.0.113.9"))
	assert.Equal(t, "中国 广东 广州 联通", GetIPLocation("203.0.113.9"))
	assert.Equal(t, int32(1), hits.Load(), "second lookup must be served from cache")
}

func TestGetIPLocationTreatsErrorPayloadAsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":{"ip":{"query":""},"location":"错误的或不完整的IP地址: 203.0.113.10","country":"","local":""}}`))
	}))
	defer server.Close()
	t.Setenv("IP_LOCATION_API", server.URL+"/?ip=%s")

	assert.Equal(t, "", GetIPLocation("203.0.113.10"))
}

func TestGetIPLocationDisabledByEmptyAPI(t *testing.T) {
	t.Setenv("IP_LOCATION_API", " ")

	assert.Equal(t, "", GetIPLocation("203.0.113.11"))
	assert.False(t, strings.Contains(GetIPLocation("203.0.113.11"), "中国"))
}

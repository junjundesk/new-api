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
	"net/http"
	"net/url"
	"strings"
)

// RequestSourceInfo 描述请求从哪个 API 站点进入网关。同一套网关常被多个域名
// 暴露，记录客户端实际访问的域名即可区分用户来源站点（例如 api.a.com 与
// api.b.com 都指向本实例）。以 JSON 存入日志 other 的 "request_source" 键。
type RequestSourceInfo struct {
	// Host 是客户端实际访问的站点域名（小写，可能带端口，来自
	// X-Forwarded-Host 或 Host 头）。
	Host string `json:"host"`
	// Origin 是浏览器跨站调用时的页面来源站点（取自 Origin，回退 Referer）；
	// 与 Host 相同或客户端未发送时为空。
	Origin string `json:"origin,omitempty"`
}

// maxRequestSourceHostLength 限制写入日志的域名长度（DNS 上限 253，留出端口余量）。
const maxRequestSourceHostLength = 255

// AttachRequestSourceToOther 把请求来源站点写入日志 other["request_source"]。
// 已存在的值不覆盖：同一次请求可能被多处补写，以最先记录的为准。
func AttachRequestSourceToOther(other map[string]interface{}, r *http.Request) {
	if other == nil || r == nil {
		return
	}
	if _, exists := other["request_source"]; exists {
		return
	}
	info := ResolveRequestSource(r)
	if info.Host == "" && info.Origin == "" {
		return
	}
	other["request_source"] = info
}

// ResolveRequestSource 解析请求的来源站点。域名优先取 X-Forwarded-Host
// （反向代理 / CDN 转发原始域名时放这里），回退到 Host 头。
// 两个头都由客户端可控，只用于日志归属展示，不参与任何鉴权判断。
func ResolveRequestSource(r *http.Request) RequestSourceInfo {
	if r == nil {
		return RequestSourceInfo{}
	}
	host := sanitizeRequestSourceHost(firstHeaderHost(r.Header.Get("X-Forwarded-Host")))
	if host == "" {
		host = sanitizeRequestSourceHost(r.Host)
	}
	origin := sanitizeRequestSourceHost(requestOriginHost(r))
	if origin == host {
		origin = ""
	}
	return RequestSourceInfo{Host: host, Origin: origin}
}

// firstHeaderHost 取逗号分隔代理头中的第一个值（最靠近客户端的一跳）。
func firstHeaderHost(header string) string {
	host, _, _ := strings.Cut(header, ",")
	return host
}

// requestOriginHost 取浏览器跨站请求的来源站点：优先 Origin，回退 Referer。
// 服务端 SDK 不发送这两个头，此时返回空。
func requestOriginHost(r *http.Request) string {
	for _, name := range []string{"Origin", "Referer"} {
		raw := strings.TrimSpace(r.Header.Get(name))
		if raw == "" || strings.EqualFold(raw, "null") {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		return u.Host
	}
	return ""
}

// sanitizeRequestSourceHost 归一化域名：小写、去空白，拒绝含非法字符或超长的
// 值，避免客户端用畸形 Host 污染日志内容。
func sanitizeRequestSourceHost(raw string) string {
	host := strings.ToLower(strings.TrimSpace(raw))
	if host == "" || len(host) > maxRequestSourceHostLength {
		return ""
	}
	for _, ch := range host {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
		case ch == '.' || ch == '-' || ch == '_' || ch == ':' || ch == '[' || ch == ']':
		default:
			return ""
		}
	}
	return host
}

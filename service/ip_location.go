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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
)

const (
	// defaultIPLocationAPI 是纯真 IP 库的在线查询接口，%s 处填入待查 IP。
	// 可用环境变量 IP_LOCATION_API 指向自建接口；置空即关闭归属地查询。
	defaultIPLocationAPI = "https://ip.zxinc.org/api.php?type=json&ip=%s"

	ipLocationCacheNamespace = "ip_location:v1"
	ipLocationCacheCapacity  = 30000
	ipLocationCacheTTL       = 24 * time.Hour
	ipLocationFailureTTL     = 5 * time.Minute
	ipLocationRequestTimeout = 3 * time.Second
	ipLocationMaxConcurrency = 8
	ipLocationMaxBodyBytes   = 64 << 10
)

// ipLocationSeparators 是接口用来分隔地名的字符：纯真接口用 U+2013 短破折号
// 连接省市区（中国–广东–广州），IPv6 结果里还夹着制表符，展示前统一换成空格。
const ipLocationSeparators = "–—\t\r\n"

var (
	ipLocationCacheOnce sync.Once
	ipLocationCache     *cachex.HybridCache[string]
	ipLocationClient    = &http.Client{Timeout: ipLocationRequestTimeout}
	ipLocationSlots     = make(chan struct{}, ipLocationMaxConcurrency)
)

func getIPLocationCache() *cachex.HybridCache[string] {
	ipLocationCacheOnce.Do(func() {
		ipLocationCache = cachex.NewHybridCache[string](cachex.HybridCacheConfig[string]{
			Namespace: cachex.Namespace(ipLocationCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.StringCodec{},
			Memory: func() *hot.HotCache[string, string] {
				return hot.NewHotCache[string, string](hot.LRU, ipLocationCacheCapacity).
					WithTTL(ipLocationCacheTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return ipLocationCache
}

// GetIPLocation 返回 IP 归属地，形如「中国 广东 广州 联通」。
// 非法或非公网 IP（私有、回环、链路本地、组播）返回空串；外部接口失败同样
// 返回空串，第三方错误文案不会透出给前端。
func GetIPLocation(ipText string) string {
	ip := net.ParseIP(strings.TrimSpace(ipText))
	if ip == nil || !isPublicIP(ip) {
		return ""
	}
	key := ip.String()
	cache := getIPLocationCache()
	if location, found, err := cache.Get(key); err == nil && found {
		return location
	}

	// 外部查询受限并发：打满时直接返回空串，避免慢请求堆积拖垮实例。
	select {
	case ipLocationSlots <- struct{}{}:
		defer func() { <-ipLocationSlots }()
	default:
		return ""
	}

	location := fetchIPLocation(key)
	ttl := ipLocationCacheTTL
	if location == "" {
		// 失败结果短缓存，避免同一个 IP 反复请求外部接口。
		ttl = ipLocationFailureTTL
	}
	_ = cache.SetWithTTL(key, location, ttl)
	return location
}

func isPublicIP(ip net.IP) bool {
	return !(ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast())
}

type ipLocationAPIResponse struct {
	Code int `json:"code"`
	Data struct {
		IP struct {
			Query string `json:"query"`
		} `json:"ip"`
		Country string `json:"country"`
		Local   string `json:"local"`
	} `json:"data"`
}

func fetchIPLocation(ip string) string {
	apiTemplate := strings.TrimSpace(common.GetEnvOrDefaultString("IP_LOCATION_API", defaultIPLocationAPI))
	if apiTemplate == "" {
		return ""
	}
	endpoint := apiTemplate
	if strings.Contains(apiTemplate, "%s") {
		endpoint = fmt.Sprintf(apiTemplate, url.QueryEscape(ip))
	}

	resp, err := ipLocationClient.Get(endpoint)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, ipLocationMaxBodyBytes))
	if err != nil {
		return ""
	}

	var parsed ipLocationAPIResponse
	if err := common.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	// 接口对非法 IP 仍返回 code=0，只把错误写在 country 里，
	// 因此用回显的 query 是否为空判断这次查询是否真的成功。
	if parsed.Code != 0 || parsed.Data.IP.Query == "" {
		return ""
	}
	return formatIPLocation(parsed.Data.Country, parsed.Data.Local)
}

// formatIPLocation 归一化接口返回的地名：分隔符换空格、压缩空白、丢弃空段。
func formatIPLocation(country, local string) string {
	parts := make([]string, 0, 2)
	for _, raw := range []string{country, local} {
		cleaned := strings.Map(func(r rune) rune {
			if strings.ContainsRune(ipLocationSeparators, r) {
				return ' '
			}
			return r
		}, raw)
		cleaned = strings.Join(strings.Fields(cleaned), " ")
		if cleaned != "" {
			parts = append(parts, cleaned)
		}
	}
	return strings.Join(parts, " ")
}

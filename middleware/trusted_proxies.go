package middleware

import (
	"errors"
	"fmt"
	"log"
	"net/textproto"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/http/httpguts"
)

// CaptureClientIP snapshots the client address before relay handlers or
// upstream processing can alter the request context.
func CaptureClientIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(common.RequestClientIPKey, c.ClientIP())
		c.Next()
	}
}

var defaultTrustedProxyCIDRs = []string{
	"127.0.0.0/8",
	"::1",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
}

func ConfigureTrustedProxies(engine *gin.Engine) error {
	rawRemoteIPHeaders := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_HEADERS"))
	if rawRemoteIPHeaders != "" {
		parts := strings.Split(rawRemoteIPHeaders, ",")
		remoteIPHeaders := make([]string, 0, len(parts))
		for _, part := range parts {
			header := strings.TrimSpace(part)
			if header == "" {
				continue
			}
			if !httpguts.ValidHeaderFieldName(header) {
				return fmt.Errorf("invalid TRUSTED_PROXY_HEADERS entry %q", header)
			}
			remoteIPHeaders = append(remoteIPHeaders, textproto.CanonicalMIMEHeaderKey(header))
		}
		if len(remoteIPHeaders) == 0 {
			return errors.New("TRUSTED_PROXY_HEADERS does not contain a header name")
		}
		engine.RemoteIPHeaders = remoteIPHeaders
	}

	rawTrustedProxies := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if rawTrustedProxies == "" {
		log.Print("WARNING: TRUSTED_PROXIES is unset or blank; trusting loopback, RFC 1918, and IPv6 ULA proxy addresses for compatibility. Set TRUSTED_PROXIES=none to trust no proxies, or configure explicit proxy IPs/CIDRs to replace these defaults.")
		return engine.SetTrustedProxies(defaultTrustedProxyCIDRs)
	}
	if strings.EqualFold(rawTrustedProxies, "none") {
		return engine.SetTrustedProxies(nil)
	}

	parts := strings.Split(rawTrustedProxies, ",")
	trustedProxies := make([]string, 0, len(parts))
	for _, part := range parts {
		trustedProxy := strings.TrimSpace(part)
		if trustedProxy == "" {
			continue
		}
		if strings.EqualFold(trustedProxy, "none") {
			return errors.New("TRUSTED_PROXIES=none must be used alone")
		}
		trustedProxies = append(trustedProxies, trustedProxy)
	}
	if len(trustedProxies) == 0 {
		return errors.New("TRUSTED_PROXIES does not contain an IP address or CIDR")
	}
	if err := engine.SetTrustedProxies(trustedProxies); err != nil {
		return fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}
	return nil
}


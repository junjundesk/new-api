package controller

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

// paymentReturnPath builds an absolute return URL from the configured server
// address. Any query or fragment configured on the base address is dropped so
// it cannot leak into the payment return URL and be re-read as callback fields.
func paymentReturnPath(suffix string) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	if parsed, err := url.Parse(base); err == nil {
		parsed.RawQuery = ""
		parsed.ForceQuery = false
		parsed.Fragment = ""
		parsed.RawFragment = ""
		base = strings.TrimRight(parsed.String(), "/")
	}
	return base + suffix
}

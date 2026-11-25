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
package operation_setting

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
)

// Persist the mode and its URL as one option; keep the URL when switching back
// to gateways. It is a browser destination, never fetched by the server.
const CardStoreOptionKey = "payment_setting.card_store"

type CardStoreConfig struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

func ParseCardStoreConfig(raw string) (CardStoreConfig, error) {
	var input struct {
		Enabled *bool  `json:"enabled"`
		URL     string `json:"url"`
	}
	if len(raw) > 16384 || common.UnmarshalJsonStr(raw, &input) != nil || input.Enabled == nil {
		return CardStoreConfig{}, errors.New("invalid card store configuration")
	}
	cfg := CardStoreConfig{Enabled: *input.Enabled, URL: strings.TrimSpace(input.URL)}
	if cfg.URL == "" && !cfg.Enabled {
		return cfg, nil
	}
	u, err := url.Parse(cfg.URL)
	if err != nil || len(cfg.URL) > 2048 || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.Contains(cfg.URL, `\`) || strings.ContainsFunc(cfg.URL, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return CardStoreConfig{}, errors.New("card store URL must be an absolute HTTP or HTTPS URL without credentials")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.ParseUint(port, 10, 16)
		if err != nil || n == 0 {
			return CardStoreConfig{}, errors.New("invalid card store URL port")
		}
	}
	return cfg, nil
}

// Use a value snapshot under the same lock used by the option loader. Readers
// never see a mode paired with an old URL during a live settings change.
func GetCardStoreConfig() CardStoreConfig {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return paymentSetting.CardStore
}

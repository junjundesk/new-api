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
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCardStoreConfigRequiresAPublicFacingHTTPURLWhenEnabled(t *testing.T) {
	for _, raw := range []string{"http://shop.test/tier?id=3", "https://buy.test/cards/#/store"} {
		encoded, err := common.Marshal(CardStoreConfig{Enabled: true, URL: " " + raw + " "})
		require.NoError(t, err)
		cfg, err := ParseCardStoreConfig(string(encoded))
		require.NoError(t, err)
		assert.True(t, cfg.Enabled)
		assert.Equal(t, raw, cfg.URL)
	}
	cfg, err := ParseCardStoreConfig(`{"enabled":false,"url":""}`)
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
	for _, raw := range []string{"", "/cards", "//other.test/shop", "https:///x", "javascript:alert(1)", "data:text/html,test", "https://buyer:secret@shop.test/", "http://shop.test:99999/", "https://shop.test/\x01x", "https://shop.test\\@other.test/"} {
		encoded, err := common.Marshal(CardStoreConfig{Enabled: true, URL: raw})
		require.NoError(t, err)
		_, err = ParseCardStoreConfig(string(encoded))
		assert.Error(t, err, "accepted %q", raw)
	}
	for _, raw := range []string{`{}`, `{"enabled":"yes","url":"https://shop.test"}`, `null`, `{"enabled":true,"url":"http://shop.test"} trailing`} {
		_, err := ParseCardStoreConfig(raw)
		assert.Error(t, err)
	}
}

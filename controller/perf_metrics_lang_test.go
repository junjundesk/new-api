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
package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The performance page always sends lang; a malformed or unknown tag must be
// ignored rather than failing the aggregation endpoint.
func TestNormalizePerfMetricsLang(t *testing.T) {
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "empty", lang: "", want: ""},
		{name: "trimmed", lang: "  zh-TW  ", want: "zh-TW"},
		{name: "primary", lang: "en", want: "en"},
		{name: "underscore variant", lang: "zh_TW", want: "zh_TW"},
		{name: "too long", lang: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", want: ""},
		{name: "injection", lang: "en<script>", want: ""},
		{name: "whitespace inside", lang: "zh TW", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizePerfMetricsLang(tt.lang))
		})
	}
}

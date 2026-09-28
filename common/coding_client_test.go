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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCodingClientUserAgent(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      bool
	}{
		{name: "codex cli", userAgent: "codex_cli_rs/0.44.0 (Mac OS 15.0; arm64)", want: true},
		{name: "codex bare", userAgent: "Codex/1.2.3", want: true},
		{name: "claude code spaced", userAgent: "claude-code/2.0.1 (external, cli)", want: true},
		{name: "claude code dashed", userAgent: "Claude Code/1.0.30", want: true},
		{name: "pi harness", userAgent: "pi/0.9.2 (cli)", want: true},
		{name: "opencode", userAgent: "opencode/1.1.0", want: true},
		{name: "omp", userAgent: "OMP/2.1 (macos)", want: true},
		{name: "zcode", userAgent: "zcode/0.4", want: true},
		{name: "deepseek harness", userAgent: "DeepSeek Harness/0.1.7-rc.2", want: true},
		{name: "dsh short name", userAgent: "dsh/0.1.7", want: true},
		{name: "open design", userAgent: "Open Design/1.4.0", want: true},
		{name: "empty", userAgent: "", want: false},
		{name: "browser", userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/131.0.0.0 Safari/537.36", want: false},
		{name: "cursor is not on the whitelist", userAgent: "Cursor/0.42.0", want: false},
		{name: "vscode is not on the whitelist", userAgent: "vscode/1.95.0", want: false},
		{name: "substring must not match", userAgent: "mozilla/5.0 shipping/1.0", want: false},
		{name: "opencode substring in a longer word", userAgent: "myopencodeclient/1.0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsCodingClientUserAgent(tt.userAgent))
		})
	}
}

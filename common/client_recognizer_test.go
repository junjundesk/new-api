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
	"github.com/stretchr/testify/require"
)

func TestRecognizeClient(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		wantName string
		wantCat  string
		wantVar  string
		wantVer  string
		wantConf string
	}{
		{
			name:     "Codex Desktop with version",
			ua:       "Codex Desktop/0.158.0-alpha.2.1 (Windows 10.0.19045; x86_64) unknown (Codex Desktop; 26.924.22138)",
			wantName: "Codex Desktop",
			wantCat:  clientCategoryCoding,
			wantVar:  "desktop",
			wantVer:  "0.158.0-alpha.2.1",
			wantConf: clientConfIdentified,
		},
		{
			name:     "Claude Code CLI",
			ua:       "claude-code/1.0.24 (cli, x64, unknown-os)",
			wantName: "Claude Code",
			wantCat:  clientCategoryCoding,
			wantVar:  "cli",
			wantVer:  "1.0.24",
			wantConf: clientConfIdentified,
		},
		{
			name:     "Cursor editor with version",
			ua:       "Cursor/0.42 (Windows)",
			wantName: "Cursor",
			wantCat:  clientCategoryCoding,
			wantVar:  "editor",
			wantVer:  "0.42",
			wantConf: clientConfIdentified,
		},
		{
			name:     "Cherry Studio chat app",
			ua:       "CherryStudio/1.2.3",
			wantName: "Cherry Studio",
			wantCat:  clientCategoryChat,
			wantVar:  "app",
			wantVer:  "1.2.3",
			wantConf: clientConfIdentified,
		},
		{
			name:     "Chrome browser",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
			wantName: "Chrome",
			wantCat:  clientCategoryBrowser,
			wantVar:  "browser",
			wantVer:  "131.0.0.0",
			wantConf: clientConfIdentified,
		},
		{
			name:     "OpenAI Python SDK",
			ua:       "OpenAI/Python 1.55.0",
			wantName: "OpenAI Python SDK",
			wantCat:  clientCategoryLibrary,
			wantVar:  "sdk",
			wantVer:  "1.55.0",
			wantConf: clientConfIdentified,
		},
		{
			name:     "NewAPI gateway",
			ua:       "New-API/1.0",
			wantName: "NewAPI",
			wantCat:  clientCategoryGateway,
			wantVar:  "gateway",
			wantConf: clientConfIdentified,
		},
		{
			name:     "Unknown fallback takes first segment as guessed",
			ua:       "SomeRandomTool/2.0 (custom)",
			wantName: "SomeRandomTool",
			wantCat:  clientCategoryOther,
			wantVar:  "",
			wantVer:  "",
			wantConf: clientConfGuessed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := recognizeClient(tt.ua)
			require.NotNil(t, info)
			assert.Equal(t, tt.wantName, info.Name)
			assert.Equal(t, tt.wantCat, info.Category)
			if tt.wantVar != "" {
				assert.Equal(t, tt.wantVar, info.Variant)
			} else {
				assert.Empty(t, info.Variant, "variant should be empty")
			}
			if tt.wantVer != "" {
				assert.Equal(t, tt.wantVer, info.Version)
			} else {
				assert.Empty(t, info.Version, "version should be empty")
			}
			if tt.wantConf != "" {
				assert.Equal(t, tt.wantConf, info.Confidence)
			}
			assert.Equal(t, clientSourceUserAgent, info.Source)
		})
	}
}

func TestRecognizeClientEmpty(t *testing.T) {
	assert.Nil(t, recognizeClient(""))
	assert.Nil(t, recognizeClient("   "))
}

func TestAttachClientInfoToOther(t *testing.T) {
	other := map[string]interface{}{}
	AttachClientInfoToOther(other, "Codex Desktop/0.158.0 (Windows)")
	require.Contains(t, other, "client")
	require.Contains(t, other, "user_agent")
	assert.Equal(t, "Codex Desktop/0.158.0 (Windows)", other["user_agent"])
	client, ok := other["client"].(*ClientInfo)
	require.True(t, ok)
	assert.Equal(t, "Codex Desktop", client.Name)

	// Existing keys must not be overwritten
	other2 := map[string]interface{}{"user_agent": "keep", "client": "keep"}
	AttachClientInfoToOther(other2, "Chrome/100")
	assert.Equal(t, "keep", other2["user_agent"])
	assert.Equal(t, "keep", other2["client"])

	// Empty UA leaves the map untouched
	other3 := map[string]interface{}{}
	AttachClientInfoToOther(other3, "")
	assert.Empty(t, other3)
}
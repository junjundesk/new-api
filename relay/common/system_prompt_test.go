/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

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

	"github.com/stretchr/testify/require"
)

func TestMergeChannelSystemPrompt(t *testing.T) {
	tests := []struct {
		name        string
		channel     string
		user        string
		concatenate bool
		want        string
	}{
		{
			name:    "channel prompt overrides user prompt by default",
			channel: "channel rules",
			user:    "user rules",
			want:    "channel rules",
		},
		{
			name:        "concatenation keeps channel prompt first",
			channel:     "channel rules",
			user:        "user rules",
			concatenate: true,
			want:        "channel rules\nuser rules",
		},
		{
			name:        "empty user prompt does not add a separator",
			channel:     "channel rules",
			user:        "  \n\t",
			concatenate: true,
			want:        "channel rules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MergeChannelSystemPrompt(tt.channel, tt.user, tt.concatenate))
		})
	}
}

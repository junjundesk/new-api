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
package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
)

// TestTaskModel2DtoHidesUpstreamModelFromUsers verifies the upstream model name
// recorded by the channel model mapping is only exposed on admin task views:
// it is channel configuration and must not leak to the user who submitted the
// task.
func TestTaskModel2DtoHidesUpstreamModelFromUsers(t *testing.T) {
	task := &model.Task{
		TaskID: "task-1",
		Properties: model.Properties{
			Input:             "prompt",
			OriginModelName:   "gpt-4o-mini",
			UpstreamModelName: "qwen-turbo",
		},
	}

	adminDto := TaskModel2Dto(task, true)
	adminProperties, ok := adminDto.Properties.(model.Properties)
	assert.True(t, ok)
	assert.Equal(t, "qwen-turbo", adminProperties.UpstreamModelName)
	assert.Equal(t, "gpt-4o-mini", adminProperties.OriginModelName)

	userDto := TaskModel2Dto(task, false)
	userProperties, ok := userDto.Properties.(model.Properties)
	assert.True(t, ok)
	assert.Empty(t, userProperties.UpstreamModelName, "upstream model name must not reach user task views")
	assert.Equal(t, "gpt-4o-mini", userProperties.OriginModelName)
}

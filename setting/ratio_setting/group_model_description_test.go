package ratio_setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckGroupModelDescriptions(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "empty object", value: "{}"},
		{name: "valid map", value: `{"vip":{"gpt-4o":"VIP 专享"}}`},
		{name: "empty nested map", value: `{"vip":{}}`},
		{name: "not an object", value: "[]", wantErr: "JSON 对象"},
		{name: "nested value is not a string", value: `{"vip":{"gpt-4o":1}}`, wantErr: "JSON 对象"},
		{name: "blank group name", value: `{" ":{"gpt-4o":"x"}}`, wantErr: "分组名称不能为空"},
		{name: "blank model name", value: `{"vip":{" ":"x"}}`, wantErr: "模型名称不能为空"},
		{
			name:    "description too long",
			value:   `{"vip":{"gpt-4o":"` + strings.Repeat("a", maxGroupModelDescriptionLength+1) + `"}}`,
			wantErr: "模型描述过长",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := CheckGroupModelDescriptions(test.value)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestGetGroupModelDescriptionsSkipsBlanksAndReturnsCopy(t *testing.T) {
	descriptions := GetGroupRatioSetting().GroupModelDescriptions
	require.NotNil(t, descriptions)
	previous := descriptions.ReadAll()
	t.Cleanup(func() {
		descriptions.Clear()
		descriptions.AddAll(previous)
	})

	descriptions.Set("vip", map[string]string{
		"gpt-4o":       "VIP 专享",
		" ":            "empty model name",
		"blank-desc":   "   ",
		"second-model": "也配置了",
	})

	assert.Equal(t, map[string]string{
		"gpt-4o":       "VIP 专享",
		"second-model": "也配置了",
	}, GetGroupModelDescriptions("vip"))

	// Callers may mutate the returned map without touching the stored setting.
	copied := GetGroupModelDescriptions("vip")
	require.NotNil(t, copied)
	copied["gpt-4o"] = "changed"
	assert.Equal(t, "VIP 专享", GetGroupModelDescriptions("vip")["gpt-4o"])

	assert.Nil(t, GetGroupModelDescriptions("missing-group"))
}

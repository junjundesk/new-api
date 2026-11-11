package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     string
	}{
		{name: "long name keeps two characters each side", username: "lihua88", want: "li***88"},
		{name: "five characters", username: "abcde", want: "ab***de"},
		{name: "short name keeps only the first character", username: "abcd", want: "a***"},
		{name: "single character", username: "a", want: "a***"},
		{name: "multibyte name is cut on characters", username: "张三李四王五", want: "张三***王五"},
		{name: "empty", username: "", want: "***"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MaskUsername(tt.username))
		})
	}
}

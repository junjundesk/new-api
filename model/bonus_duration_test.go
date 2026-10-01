package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBonusDuration(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "bare number is hours", input: "1", want: time.Hour},
		{name: "bare decimal hours", input: "1.5", want: 90 * time.Minute},
		{name: "day", input: "1天", want: 24 * time.Hour},
		{name: "day and hours", input: "1天2小时", want: 26 * time.Hour},
		{name: "day hour minute", input: "1天2小时30分", want: 26*time.Hour + 30*time.Minute},
		{name: "week", input: "2周", want: 14 * 24 * time.Hour},
		{name: "month", input: "1月", want: 30 * 24 * time.Hour},
		{name: "year", input: "1年", want: 365 * 24 * time.Hour},
		{name: "english unit", input: "2h", want: 2 * time.Hour},
		{name: "spaces between segments", input: "1 天 2 小时", want: 26 * time.Hour},
		{name: "empty", input: "", wantErr: true},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "garbage unit", input: "1abc", wantErr: true},
		{name: "trailing junk", input: "1天abc", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBonusDuration(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestUserBonusGrantRemaining(t *testing.T) {
	g := &UserBonusGrant{AmountTotal: 100, AmountUsed: 30}
	assert.Equal(t, int64(70), g.Remaining())

	g = &UserBonusGrant{AmountTotal: 100, AmountUsed: 150}
	assert.Equal(t, int64(0), g.Remaining(), "over-used grant must not report negative remaining")
}

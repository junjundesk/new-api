package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The referral program ships with BonusDuration "0", and that value must mean
// "never expires" rather than "invalid": a shipped default that errors out
// would leave every reward unsettled.
func TestReferralBonusExpireTime(t *testing.T) {
	cases := []struct {
		name        string
		duration    string
		wantNever   bool
		wantErr     bool
		atLeastFrom time.Duration
		atMostFrom  time.Duration
	}{
		{name: "zero means never expires", duration: "0", wantNever: true},
		{name: "blank means never expires", duration: "", wantNever: true},
		{name: "padded zero means never expires", duration: " 0 ", wantNever: true},
		{name: "hours", duration: "1", atLeastFrom: 59 * time.Minute, atMostFrom: 2 * time.Hour},
		{name: "explicit days", duration: "1天", atLeastFrom: 23 * time.Hour, atMostFrom: 25 * time.Hour},
		{name: "garbage is rejected", duration: "abc", wantErr: true},
		{name: "negative is rejected", duration: "-1", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := referralBonusExpireTime(tc.duration)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.wantNever {
				assert.Equal(t, int64(0), got, "0 must mean a grant that never expires")
				return
			}
			delta := time.Duration(got-common.GetTimestamp()) * time.Second
			assert.GreaterOrEqual(t, delta, tc.atLeastFrom)
			assert.LessOrEqual(t, delta, tc.atMostFrom)
		})
	}
}

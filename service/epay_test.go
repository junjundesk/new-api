package service

import (
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEpayCallback(t *testing.T) {
	previousPartnerID := operation_setting.EpayId
	operation_setting.EpayId = "10001"
	t.Cleanup(func() { operation_setting.EpayId = previousPartnerID })

	testCases := []struct {
		name    string
		values  url.Values
		wantErr error
	}{
		{
			name: "accepts a well formed callback",
			values: url.Values{
				"pid":          {"10001"},
				"trade_no":     {"2026010100001"},
				"out_trade_no": {"USR1NOABC123"},
				"type":         {"alipay"},
				"name":         {"TUC10"},
				"money":        {"10.00"},
				"trade_status": {"TRADE_SUCCESS"},
				"sign":         {"d41d8cd98f00b204e9800998ecf8427e"},
				"sign_type":    {"MD5"},
			},
		},
		{
			name: "rejects a repeated parameter",
			values: url.Values{
				"pid":          {"10001"},
				"out_trade_no": {"USR1NOABC123", "USR2NOABC123"},
				"money":        {"10.00"},
			},
			wantErr: ErrEpayCallbackDuplicateParam,
		},
		{
			name: "rejects an ampersand smuggled into a value",
			values: url.Values{
				"pid":          {"10001"},
				"out_trade_no": {"USR1NOABC123"},
				"name":         {"x&trade_status=TRADE_SUCCESS"},
				"money":        {"10.00"},
			},
			wantErr: ErrEpayCallbackDelimiterInValue,
		},
		{
			name: "rejects an equals sign smuggled into a value",
			values: url.Values{
				"pid":          {"10001"},
				"out_trade_no": {"USR1NOABC123"},
				"name":         {"x=money=10.00"},
				"money":        {"10.00"},
			},
			wantErr: ErrEpayCallbackDelimiterInValue,
		},
		{
			name: "rejects a fragment smuggled into a value",
			values: url.Values{
				"pid":          {"10001"},
				"out_trade_no": {"USR1NOABC123#extra"},
				"money":        {"10.00"},
			},
			wantErr: ErrEpayCallbackDelimiterInValue,
		},
		{
			name: "rejects a foreign partner id",
			values: url.Values{
				"pid":          {"20002"},
				"out_trade_no": {"USR1NOABC123"},
				"money":        {"10.00"},
			},
			wantErr: ErrEpayCallbackPartnerMismatch,
		},
		{
			name: "rejects a missing partner id",
			values: url.Values{
				"out_trade_no": {"USR1NOABC123"},
				"money":        {"10.00"},
			},
			wantErr: ErrEpayCallbackPartnerMismatch,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params, err := ValidateEpayCallback(tc.values)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, params)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, len(tc.values), len(params))
			for key, values := range tc.values {
				assert.Equal(t, values[0], params[key])
			}
		})
	}
}

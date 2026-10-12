package dto

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelModelRatioValidation(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		nonFinite float64
		present   bool
		wantRatio float64
		wantError bool
	}{
		{name: "missing inherits", payload: `{"proxy":""}`},
		{name: "explicit zero", payload: `{"proxy":"","model_group_ratios":{"billing-model":0}}`, present: true, wantRatio: 0},
		{name: "explicit one", payload: `{"proxy":"","model_group_ratios":{"billing-model":1}}`, present: true, wantRatio: 1},
		{name: "fractional discount", payload: `{"proxy":"","model_group_ratios":{"billing-model":0.8}}`, present: true, wantRatio: 0.8},
		{name: "inclusive upper bound", payload: `{"proxy":"","model_group_ratios":{"billing-model":1000}}`, present: true, wantRatio: 1000},
		{name: "null is not inheritance", payload: `{"proxy":"","model_group_ratios":{"billing-model":null}}`, present: true, wantError: true},
		{name: "negative", payload: `{"proxy":"","model_group_ratios":{"billing-model":-0.1}}`, present: true, wantError: true},
		{name: "NaN", nonFinite: math.NaN(), present: true, wantError: true},
		{name: "positive infinity", nonFinite: math.Inf(1), present: true, wantError: true},
		{name: "above upper bound", payload: `{"proxy":"","model_group_ratios":{"billing-model":1000.01}}`, present: true, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var settings ChannelSettings
			if tt.payload != "" {
				// The standalone relaykit tests cannot import the host common package.
				require.NoError(t, json.Unmarshal([]byte(tt.payload), &settings))
			} else {
				// JSON cannot encode nonfinite numbers; exercise the Go API directly.
				ratio := tt.nonFinite
				settings.ModelGroupRatios = map[string]*float64{"billing-model": &ratio}
			}
			ratio, present := settings.ModelGroupRatios["billing-model"]
			assert.Equal(t, tt.present, present)

			err := settings.ValidateModelGroupRatios()
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "model_group_ratios")
				return
			}
			require.NoError(t, err)
			if tt.present {
				require.NotNil(t, ratio)
				assert.Equal(t, tt.wantRatio, *ratio)
			}

			encoded, err := json.Marshal(settings)
			require.NoError(t, err)
			assert.JSONEq(t, tt.payload, string(encoded), "saving must preserve explicit zero and omission")
		})
	}
}

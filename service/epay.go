package service

import (
	"errors"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

func GetCallbackAddress() string {
	if operation_setting.CustomCallbackAddress == "" {
		return system_setting.ServerAddress
	}
	return operation_setting.CustomCallbackAddress
}

// epaySigningDelimiters are the characters used to join the epay signing
// string ("k=v&k=v"). A callback parameter whose value contains any of them can
// smuggle an extra key/value pair into that string: an attacker who knows one
// signature can then re-split the same string into different parameters and
// still verify. Such callbacks are rejected instead of trusted.
const epaySigningDelimiters = "&=#"

var (
	ErrEpayCallbackDuplicateParam   = errors.New("epay callback contains a duplicate parameter")
	ErrEpayCallbackDelimiterInValue = errors.New("epay callback parameter value contains a signing delimiter")
	ErrEpayCallbackPartnerMismatch  = errors.New("epay callback partner id mismatch")
)

// ValidateEpayCallback flattens the callback form into a single-valued map
// after rejecting the inputs that would let a caller forge the signed string:
//
//   - a repeated key, which url.Values.Get would silently collapse to its first
//     value while the gateway may have signed the other one;
//   - a value containing "&", "=" or "#", the delimiters of the signing string.
//
// It also pins pid to the configured merchant id so a callback signed by a
// different partner cannot complete an order.
func ValidateEpayCallback(values url.Values) (map[string]string, error) {
	params := make(map[string]string, len(values))
	for key, rawValues := range values {
		if len(rawValues) != 1 {
			return nil, ErrEpayCallbackDuplicateParam
		}
		value := rawValues[0]
		if strings.ContainsAny(value, epaySigningDelimiters) {
			return nil, ErrEpayCallbackDelimiterInValue
		}
		params[key] = value
	}
	if params["pid"] != operation_setting.EpayId {
		return nil, ErrEpayCallbackPartnerMismatch
	}
	return params, nil
}

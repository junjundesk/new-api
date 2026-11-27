package common

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/jinzhu/copier"
)

var deepCopyOptions = copier.Option{
	DeepCopy:    true,
	IgnoreEmpty: true,
	// A Responses input can be tens of MiB. Copy its raw JSON in bulk,
	// not by reflecting over each byte. Retry attempts still own their copy.
	Converters: []copier.TypeConverter{{
		SrcType: json.RawMessage(nil),
		DstType: json.RawMessage(nil),
		Fn: func(src any) (any, error) {
			return slices.Clone(src.(json.RawMessage)), nil
		},
	}},
}

func DeepCopy[T any](src *T) (*T, error) {
	if src == nil {
		return nil, fmt.Errorf("copy source cannot be nil")
	}
	var dst T
	err := copier.CopyWithOption(&dst, src, deepCopyOptions)
	if err != nil {
		return nil, err
	}
	return &dst, nil
}

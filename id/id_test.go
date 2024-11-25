package id

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var tests = []ID{
	{value: [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	{value: [16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
	{value: [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}},
	{value: [16]byte{0xef, 0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80, 0x90, 0xa0, 0xb0, 0xc0, 0xd0, 0xe0, 0xf0}},
}

func TestIDRoundtrip(t *testing.T) {
	for _, tt := range tests {
		s := tt.String()
		v, err := Parse(s)

		assert.NoError(t, err, "error parsing id string")
		assert.Equalf(t, tt, v, "value failed trip: %v -> %s -> %v", tt.value, s, v.value)
	}
}

func TestIDJsonRoundtrip(t *testing.T) {
	for _, tt := range tests {
		s, err := json.Marshal(tt)
		assert.NoError(t, err, "error json marshaling")

		var v ID
		err = json.Unmarshal(s, &v)

		assert.NoError(t, err, "error json unmarshaling")
		assert.Equalf(t, tt, v, "value failed trip: %v -> %s -> %v", tt.value, s, v.value)
	}
}
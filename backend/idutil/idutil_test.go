package idutil

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromFloat(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want uint
		ok   bool
	}{
		{"one", 1, 1, true},
		{"max uint32", math.MaxUint32, math.MaxUint32, true},
		{"zero", 0, 0, false},
		{"negative", -1, 0, false},
		{"fraction", 1.5, 0, false},
		{"too large", math.MaxUint32 + 1, 0, false},
		{"NaN", math.NaN(), 0, false},
		{"Inf", math.Inf(1), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := FromFloat(tt.in)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParse(t *testing.T) {
	got, err := Parse("42")
	assert.NoError(t, err)
	assert.Equal(t, uint(42), got)

	for _, in := range []string{"", "0", "-1", "abc", "4294967296", "1.5"} {
		_, err := Parse(in)
		assert.Error(t, err, in)
	}
}

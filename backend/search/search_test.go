package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchQuery(t *testing.T) {
	cases := map[string]string{
		"zoe":              `"zoe"*`,
		"  Zoë   Müller ":  `"Zoë"* "Müller"*`,
		"john-smith":       `"john"* "smith"*`,
		"zoe@work.example": `"zoe"* "work"* "example"*`,
		`o"brien`:          `"o"* "brien"*`,
		"+41 79":           `"41"* "79"*`,
		"AND OR NOT NEAR":  `"AND"* "OR"* "NOT"* "NEAR"*`,
		"col:val*":         `"col"* "val"*`,
	}
	for in, want := range cases {
		got, ok := MatchQuery(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
}

func TestMatchQueryWithoutWords(t *testing.T) {
	for _, in := range []string{"", "   ", "+", "-*", `""`, "()"} {
		_, ok := MatchQuery(in)
		assert.False(t, ok, in)
	}
}

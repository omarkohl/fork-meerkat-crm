package search

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMatchQuery(t *testing.T) {
	cases := map[string]string{
		"zoe":              `"zoe"*`,
		"  Zoë   Müller ":  `"Zoë"* "Müller"*`,
		"john-smith":       `"john"* "smith"*`,
		"zoe@work.example": `"zoe"* "work"* "example"*`,
		`o"brien`:          `"o"* "brien"*`,
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

func TestMatchQueryLeavesPhoneLikeAndCJKInputToTheSubstringSearch(t *testing.T) {
	for _, in := range []string{"795550", "+41 79", "(030) 1234", "0101", "555-0101", "1.2.3",
		"山田", "田中太郎", "やまだ", "ヤマダ", "김민수", "Yamada 山田"} {
		_, ok := MatchQuery(in)
		assert.False(t, ok, in)
	}
	// words that merely contain digits still use the index
	for _, in := range []string{"room 12", "abc123", "2024 report", "+41 anna"} {
		_, ok := MatchQuery(in)
		assert.True(t, ok, in)
	}
}

func TestMatchQueryCapsLongInput(t *testing.T) {
	long := strings.Repeat("ab ", 100)
	got, ok := MatchQuery(long)
	assert.True(t, ok)
	assert.Equal(t, MaxTermWords, strings.Count(got, `"*`))

	// 300 characters in one word: cut to 200
	got, ok = MatchQuery(strings.Repeat("a", 300))
	assert.True(t, ok)
	assert.Equal(t, `"`+strings.Repeat("a", MaxTermRunes)+`"*`, got)

	// multi-byte characters are cut by character, not byte
	got, _ = MatchQuery(strings.Repeat("é", 300))
	assert.Equal(t, MaxTermRunes, len([]rune(got))-3)
}

func TestAvailableIsRememberedPerDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	assert.False(t, Available(db, "contacts_fts"))
	require.NoError(t, db.Exec("CREATE VIRTUAL TABLE contacts_fts USING fts5(a)").Error)
	assert.False(t, Available(db, "contacts_fts"), "the first answer is kept")

	other, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, other.Exec("CREATE VIRTUAL TABLE contacts_fts USING fts5(a)").Error)
	assert.True(t, Available(other, "contacts_fts"), "another database is looked up separately")
	// a transaction on the same database shares the answer
	require.NoError(t, other.Transaction(func(tx *gorm.DB) error {
		assert.True(t, Available(tx, "contacts_fts"))
		return nil
	}))
}

// Package search builds SQLite FTS5 queries for contacts, notes and activities. The indexes
// are created and kept in sync by migration 29; when they are missing (a database that was
// not migrated, or an SQLite build without FTS5) callers fall back to LIKE.
package search

import (
	"strings"
	"unicode"

	"gorm.io/gorm"
)

// MatchQuery turns what the user typed into an FTS5 MATCH expression: every word must
// match as a prefix ("mül" finds "Müller"), in any order. Each word is quoted, so FTS5
// operators and punctuation in the input are plain text. ok is false when the input has no
// letters or digits, in which case the caller should use a LIKE search.
func MatchQuery(term string) (string, bool) {
	words := strings.FieldsFunc(term, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if len(words) == 0 {
		return "", false
	}
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = `"` + w + `"*`
	}
	return strings.Join(quoted, " "), true
}

// Available reports whether the FTS index with this name exists.
func Available(db *gorm.DB, table string) bool {
	var n int64
	err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&n).Error
	return err == nil && n > 0
}

// Hits is a subquery of the matching ids with their rank (smaller is better), for joining.
// The columns are named hit_id and hit_rank so they never clash with the joined table's:
//
//	Joins("JOIN "+search.Hits("contacts_fts")+" AS hits ON hits.hit_id = contacts.id", match)
func Hits(table string) string {
	return "(SELECT rowid AS hit_id, rank AS hit_rank FROM " + table + " WHERE " + table + " MATCH ?)"
}

// ContactNames limits a contacts_fts expression to the name columns.
func ContactNames(match string) string {
	return "{firstname lastname nickname} : (" + match + ")"
}

package services

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzCSVImport runs untrusted CSV through the whole non-database import path: parse,
// suggest the column mapping, build a contact per row, validate it. No input may panic
// and the parser must keep its documented limits.
//
// Run: go test ./services -run '^$' -fuzz FuzzCSVImport -fuzztime 30s
func FuzzCSVImport(f *testing.F) {
	seeds := []string{
		"First Name,Last Name,Email,Phone,Birthday\nAda,Lovelace,ada@example.com,+15551234567,1815-12-10\n",
		"firstname,lastname,e-mail 1 - label,e-mail 1 - value,phone 2 - value,circles\nA,B,Work,a@b.c,123,\"x; y\"\n",
		"Name,Address 1 - Street,Address 1 - City,Address 2 - Street,Gender,Notes\n\"Q, \"\"R\"\"\",1 Main,Town,2 Side,f,\"multi\nline\"\n",
		"a,a,a\n1,2\n1,2,3,4\n",
		"\"unterminated,quote\nx,y\n",
		"\xef\xbb\xbfFirst Name\nBOM\n",
		"Birthday\n--02-29\n02/29/2000\n",
		"",
		",\n,\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		headers, rows, err := ParseCSV(strings.NewReader(data))
		if err != nil {
			return
		}
		if len(headers) == 0 || len(rows) == 0 || len(rows) > MaxCSVRows {
			t.Fatalf("ParseCSV broke its contract: %d headers, %d rows", len(headers), len(rows))
		}
		mappings := SuggestColumnMappings(headers)
		if len(mappings) != len(headers) {
			t.Fatalf("got %d mappings for %d headers", len(mappings), len(headers))
		}
		for _, row := range rows {
			c := BuildContactFromRow(1, headers, row, mappings)
			ValidateImportedContact(&c)
			ContactToPreviewMap(&c)
		}
	})
}

// FuzzImportValueParsers covers the single-value helpers behind the CSV and VCF import.
func FuzzImportValueParsers(f *testing.F) {
	for _, s := range []string{"1990-01-02", "--02-29", "02/29/2000", "29.02.2000", "+1 (555) 123-4567", "f", "x;y, z", "", "0000-00-00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// Unknown formats come back trimmed and unchanged (they fail validation later);
		// anything the function rewrote must be in the app's ISO format.
		if n := NormalizeBirthday(s); n != strings.TrimSpace(s) && !IsValidBirthdayFormat(n) {
			t.Fatalf("NormalizeBirthday(%q) rewrote it to %q, which is not a valid birthday", s, n)
		}
		_ = IsValidPhone(s)
		_ = NormalizeGender(s)
		_ = normalizePhoneForComparison(s)
		for _, circle := range ParseCircles(s) {
			if circle == "" || !utf8.ValidString(circle) && utf8.ValidString(s) {
				t.Fatalf("ParseCircles(%q) produced %q", s, circle)
			}
		}
		_, _, _ = suggestGroupedMapping(s)
	})
}

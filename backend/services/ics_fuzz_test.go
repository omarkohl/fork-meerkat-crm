package services

import (
	"strings"
	"testing"
	"time"

	"meerkat/models"
)

// checkICS asserts the structural rules a calendar client relies on: CRLF line ends, no
// physical line over 75 octets, no bare control characters, and balanced BEGIN/END.
func checkICS(t *testing.T, doc string) {
	t.Helper()
	if !strings.HasSuffix(doc, "\r\n") {
		t.Fatalf("document does not end with CRLF: %q", doc)
	}
	depth, events := 0, 0
	for _, line := range strings.Split(strings.TrimSuffix(doc, "\r\n"), "\r\n") {
		if len(line) > icsLineLimit {
			t.Fatalf("line of %d octets: %q", len(line), line)
		}
		if strings.ContainsAny(line, "\r\n\x00") {
			t.Fatalf("bare control character in line %q", line)
		}
		switch line {
		case "BEGIN:VCALENDAR", "BEGIN:VEVENT":
			depth++
			if line == "BEGIN:VEVENT" {
				events++
			}
		case "END:VCALENDAR", "END:VEVENT":
			depth--
		}
		if depth < 0 || depth > 2 {
			t.Fatalf("unbalanced BEGIN/END at %q", line)
		}
	}
	if depth != 0 {
		t.Fatalf("unbalanced document: %q", doc)
	}
}

// FuzzICSText fuzzes the text escaping and line folding on their own.
//
// Run: go test ./services -run '^$' -fuzz FuzzICSText -fuzztime 30s
func FuzzICSText(f *testing.F) {
	for _, s := range []string{"plain", "a;b,c\\d\r\ne", strings.Repeat("é", 100), strings.Repeat("a", 200), "\x80\x80", strings.Repeat("\x80", 200), "日本語のテキスト" + strings.Repeat("😀", 30)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// Folding must terminate and keep the bytes even for input that is not UTF-8.
		raw := foldICSLine(s)
		if !strings.ContainsAny(s, "\r\n") {
			if got := strings.ReplaceAll(strings.TrimSuffix(raw, "\r\n"), "\r\n ", ""); got != s {
				t.Fatalf("unfold(fold(raw)) != raw: %q vs %q", got, s)
			}
		}
		esc := escapeICSText(s)
		if strings.ContainsAny(esc, "\r\n") {
			t.Fatalf("escaped text keeps a line break: %q", esc)
		}
		folded := foldICSLine("SUMMARY:" + esc)
		checkICS(t, "BEGIN:VCALENDAR\r\n"+folded+"END:VCALENDAR\r\n")
		// Unfolding gives back the logical line.
		if got := strings.ReplaceAll(strings.TrimSuffix(folded, "\r\n"), "\r\n ", ""); got != "SUMMARY:"+esc {
			t.Fatalf("unfold(fold(x)) != x: %q vs %q", got, "SUMMARY:"+esc)
		}
	})
}

// FuzzBuildICSFeed stores arbitrary names, birthdays, messages and recurrences and
// checks the generated feed stays well formed (user text must never inject lines).
//
// Run: go test ./services -run '^$' -fuzz FuzzBuildICSFeed -fuzztime 30s
func FuzzBuildICSFeed(f *testing.F) {
	f.Add("Ada", "Lovelace", "1815-12-10", "Call Ada", "yearly")
	f.Add("A\r\nBEGIN:VEVENT", "B;C", "--02-29", "x\r\nEND:VCALENDAR\r\n", "monthly")
	f.Add(strings.Repeat("é", 90), "", "0001-02-29", strings.Repeat("😀", 40), "six-months")
	f.Add("", "", "9999-99-99", "", "bogus")

	db := setupICSTestDB(f)
	user := icsUser(f, db, "fuzz")
	f.Fuzz(func(t *testing.T, first, last, birthday, message, recurrence string) {
		tx := db.Begin()
		defer tx.Rollback()
		c := models.Contact{UserID: user.ID, Firstname: first, Lastname: last, Birthday: birthday}
		if err := tx.Create(&c).Error; err != nil {
			return // e.g. NUL bytes or invalid UTF-8 refused by the driver
		}
		r := models.Reminder{UserID: user.ID, ContactID: &c.ID, Message: message, Recurrence: recurrence,
			RemindAt: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)}
		if err := tx.Create(&r).Error; err != nil {
			return
		}
		doc, err := BuildICSFeed(tx, user.ID, "en")
		if err != nil {
			t.Fatalf("BuildICSFeed: %v", err)
		}
		checkICS(t, doc)
	})
}

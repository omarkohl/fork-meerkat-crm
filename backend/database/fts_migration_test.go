package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func ftsCount(t *testing.T, db *sql.DB, table, match string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM "+table+" WHERE "+table+" MATCH ?", match).Scan(&n); err != nil {
		t.Fatalf("%s %q: %v", table, match, err)
	}
	return n
}

func TestFTSMigrationBuildsTheIndexAndTriggersKeepItInSync(t *testing.T) {
	db, err := sql.Open("sqlite", sqliteDSN(filepath.Join(t.TempDir(), "fts.db"), true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrateTo(t, db, 28)
	for _, q := range []string{
		"INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')",
		`INSERT INTO contacts (id, user_id, firstname, lastname, emails, phones) VALUES
		   (1, 1, 'Zoë', 'Müller', '[{"type":"home","value":"zoe@work.example"}]', '[{"type":"mobile","value":"+41 79 555 0101"}]'),
		   (2, 1, 'Bob', 'Smith', 'not json', NULL)`,
		"INSERT INTO notes (id, user_id, content, date) VALUES (1, 1, 'Talked about the Zürich trip', '2030-01-01')",
		"INSERT INTO activities (id, user_id, title, description, location, date) VALUES (1, 1, 'Café catch-up', 'Planning', 'Münster', '2030-01-01')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	// Up: existing rows are indexed
	migrateTo(t, db, 29)
	for table, queries := range map[string][]string{
		"contacts_fts":   {"zoe", "muller", "mül*", "work*", "555", "smith"},
		"notes_fts":      {"zurich", "zür*"},
		"activities_fts": {"cafe", "munster", "plan*"},
	} {
		for _, m := range queries {
			if ftsCount(t, db, table, m) != 1 {
				t.Errorf("%s MATCH %q: want 1 hit after migration", table, m)
			}
		}
	}

	// Triggers: insert, update (only indexed columns matter), delete
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec("INSERT INTO contacts (id, user_id, firstname, emails) VALUES (3, 1, 'Carol', '[{\"value\":\"carol@example.org\"}]')")
	if ftsCount(t, db, "contacts_fts", "carol") != 1 || ftsCount(t, db, "contacts_fts", "example") != 2 {
		t.Error("insert not indexed")
	}
	mustExec("UPDATE contacts SET firstname = 'Karolina' WHERE id = 3")
	if ftsCount(t, db, "contacts_fts", "carol") != 1 || ftsCount(t, db, "contacts_fts", "karolina") != 1 {
		t.Error("update not reindexed (carol still matches through the e-mail address only)")
	}
	mustExec("UPDATE contacts SET emails = '[]' WHERE id = 3")
	if ftsCount(t, db, "contacts_fts", "carol") != 0 {
		t.Error("e-mail change not reindexed")
	}
	mustExec("UPDATE contacts SET photo = 'x' WHERE id = 3")
	if ftsCount(t, db, "contacts_fts", "karolina") != 1 {
		t.Error("unrelated update broke the index")
	}
	mustExec("DELETE FROM contacts WHERE id = 3")
	if ftsCount(t, db, "contacts_fts", "karolina") != 0 {
		t.Error("delete not removed from the index")
	}
	mustExec("UPDATE notes SET content = 'Lunch plans' WHERE id = 1")
	if ftsCount(t, db, "notes_fts", "zurich") != 0 || ftsCount(t, db, "notes_fts", "lunch") != 1 {
		t.Error("note update not reindexed")
	}
	mustExec("UPDATE activities SET title = 'Dinner' WHERE id = 1")
	if ftsCount(t, db, "activities_fts", "cafe") != 0 || ftsCount(t, db, "activities_fts", "dinner") != 1 {
		t.Error("activity update not reindexed")
	}
	mustExec("DELETE FROM activities WHERE id = 1")
	mustExec("DELETE FROM notes WHERE id = 1")
	if ftsCount(t, db, "activities_fts", "dinner") != 0 || ftsCount(t, db, "notes_fts", "lunch") != 0 {
		t.Error("deletes not removed from the index")
	}

	// Down: tables and triggers go, the data stays
	migrateTo(t, db, 28)
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name LIKE '%_fts%'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("fts objects after down = %d (err %v)", n, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM contacts").Scan(&n); err != nil || n != 2 {
		t.Fatalf("contacts after down = %d (err %v)", n, err)
	}
}

package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// NOTE values that earlier versions kept in vcard_extra move into the new column
// and leave vcard_extra, so the next export does not emit them twice.
func TestContactNoteMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.db")
	db, err := sql.Open("sqlite", sqliteDSN(path, true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrateTo(t, db, 32)

	for _, q := range []string{
		"INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')",
		`INSERT INTO contacts (id, user_id, firstname, vcard_extra) VALUES (1, 1, 'Two',
			'{"properties":{"NOTE":[{"Value":"first"},{"Value":"second"}],"X-FOO":[{"Value":"bar"}]}}')`,
		"INSERT INTO contacts (id, user_id, firstname, vcard_extra) VALUES (2, 1, 'None', '')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	migrateTo(t, db, 33)
	var note, extra string
	if err := db.QueryRow("SELECT note, vcard_extra FROM contacts WHERE id = 1").Scan(&note, &extra); err != nil {
		t.Fatal(err)
	}
	if note != "first\n\nsecond" {
		t.Errorf("note = %q", note)
	}
	if extra != `{"properties":{"X-FOO":[{"Value":"bar"}]}}` {
		t.Errorf("vcard_extra = %q", extra)
	}
	if err := db.QueryRow("SELECT note FROM contacts WHERE id = 2").Scan(&note); err != nil || note != "" {
		t.Errorf("note without NOTE = %q, %v", note, err)
	}

	migrateTo(t, db, 32)
	if columnExists(t, db, "contacts", "note") {
		t.Fatal("note still present after down")
	}
}

package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// Links whose contact row is already gone (left by earlier purges) become
// tombstones when the column is added; links of existing contacts do not.
func TestCardDAVLinkPurgedAtMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "purged.db")
	db, err := sql.Open("sqlite", sqliteDSN(path, true))
	if err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	if !columnExists(t, db, "carddav_contact_links", "purged_at") {
		t.Fatal("purged_at missing after up")
	}
	defer db.Close()

	// Down: the column goes away.
	migrateTo(t, db, 24)
	if columnExists(t, db, "carddav_contact_links", "purged_at") {
		t.Fatal("purged_at still present after down")
	}

	for _, q := range []string{
		"INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')",
		"INSERT INTO contacts (id, user_id, firstname) VALUES (7, 1, 'Live')",
		"INSERT INTO carddav_connections (id, user_id, base_url, address_book_path) VALUES (1, 1, 'https://x', '/ab')",
		"INSERT INTO carddav_contact_links (connection_id, user_id, contact_id, remote_uid, remote_path) VALUES (1, 1, 7, 'live', '/ab/live.vcf')",
		"INSERT INTO carddav_contact_links (connection_id, user_id, contact_id, remote_uid, remote_path) VALUES (1, 1, 99, 'gone', '/ab/gone.vcf')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	// Up again on seeded data.
	migrateTo(t, db, 25)
	var live, gone int
	if err := db.QueryRow("SELECT COUNT(*) FROM carddav_contact_links WHERE remote_uid = 'live' AND purged_at IS NULL").Scan(&live); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM carddav_contact_links WHERE remote_uid = 'gone' AND purged_at IS NOT NULL").Scan(&gone); err != nil {
		t.Fatal(err)
	}
	if live != 1 || gone != 1 {
		t.Fatalf("live link untouched = %d, orphan link tombstoned = %d, want 1 and 1", live, gone)
	}
}

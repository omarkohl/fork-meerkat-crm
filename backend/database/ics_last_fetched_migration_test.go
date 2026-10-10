package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestIcsFeedLastFetchedAtMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ics_fetched.db")
	db, err := sql.Open("sqlite", sqliteDSN(path, true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	if !columnExists(t, db, "ics_feeds", "last_fetched_at") {
		t.Fatal("last_fetched_at missing after up")
	}

	// Down: the column goes away, the feed rows stay.
	for _, q := range []string{
		"INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')",
		"INSERT INTO ics_feeds (user_id, token_hash) VALUES (1, 'h1')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	migrateTo(t, db, 25)
	if columnExists(t, db, "ics_feeds", "last_fetched_at") {
		t.Fatal("last_fetched_at still present after down")
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM ics_feeds").Scan(&n); err != nil || n != 1 {
		t.Fatalf("feed rows after down = %d (err %v), want 1", n, err)
	}

	// Up again on seeded data: existing feeds have never been fetched.
	migrateTo(t, db, 26)
	if err := db.QueryRow("SELECT COUNT(*) FROM ics_feeds WHERE last_fetched_at IS NULL").Scan(&n); err != nil || n != 1 {
		t.Fatalf("feeds with NULL last_fetched_at = %d (err %v), want 1", n, err)
	}
}

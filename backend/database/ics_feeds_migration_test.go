package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestIcsFeedsMigrationUpAndDown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ics.db")
	sqlDB, err := sql.Open("sqlite", sqliteDSN(path, true))
	if err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(sqlDB); err != nil {
		t.Fatal(err)
	}
	if !tableExists(t, sqlDB, "ics_feeds") {
		t.Fatal("ics_feeds missing after up")
	}

	if _, err := sqlDB.Exec("INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')"); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec("INSERT INTO ics_feeds (user_id, token_hash) VALUES (1, 'h1')"); err != nil {
		t.Fatal(err)
	}
	// One feed per user, unique hashes.
	if _, err := sqlDB.Exec("INSERT INTO ics_feeds (user_id, token_hash) VALUES (1, 'h2')"); err == nil {
		t.Error("second feed for the same user should fail")
	}
	// Deleting the user removes the feed.
	if _, err := sqlDB.Exec("DELETE FROM users WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM ics_feeds").Scan(&n); err != nil || n != 0 {
		t.Errorf("feed rows after user delete = %d (err %v), want 0", n, err)
	}
	sqlDB.Close()

	if err := MigrateDown(path); err != nil {
		t.Fatal(err)
	}
	after, err := sql.Open("sqlite", sqliteDSN(path, false))
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	if tableExists(t, after, "ics_feeds") {
		t.Fatal("ics_feeds still present after down")
	}
}

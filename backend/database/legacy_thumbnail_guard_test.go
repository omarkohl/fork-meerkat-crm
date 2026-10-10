package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func guardDB(t *testing.T, legacy, file string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", sqliteDSN(filepath.Join(t.TempDir(), "g.db"), true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	migrateTo(t, db, 30)
	if _, err := db.Exec("INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO contacts (id, user_id, firstname, photo_thumbnail, photo_thumbnail_file) VALUES (1, 1, 'Zoe', ?, ?)", legacy, file); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLegacyThumbnailDropIsRefusedWhileRowsAreNotExported(t *testing.T) {
	dropping := fstest.MapFS{
		"migrations/000031_drop.up.sql": {Data: []byte("ALTER TABLE contacts DROP COLUMN photo_thumbnail;")},
	}
	harmless := fstest.MapFS{
		"migrations/000031_other.up.sql": {Data: []byte("ALTER TABLE contacts DROP COLUMN photo_thumbnail_file;")},
	}

	pending := guardDB(t, "data:image/jpeg;base64,AAAA", "")
	err := checkLegacyThumbnailsExported(pending, dropping, 30)
	if err == nil || !strings.Contains(err.Error(), "ExportLegacyThumbnails") {
		t.Fatalf("want a refusal mentioning the export, got %v", err)
	}
	if err := checkLegacyThumbnailsExported(pending, harmless, 30); err != nil {
		t.Fatalf("an unrelated migration must pass: %v", err)
	}
	if err := checkLegacyThumbnailsExported(pending, dropping, 31); err != nil {
		t.Fatalf("an already applied migration must pass: %v", err)
	}

	exported := guardDB(t, "data:image/jpeg;base64,AAAA", "x_thumb.jpg")
	if err := checkLegacyThumbnailsExported(exported, dropping, 30); err != nil {
		t.Fatalf("exported rows must pass: %v", err)
	}
	empty := guardDB(t, "", "")
	if err := checkLegacyThumbnailsExported(empty, dropping, 30); err != nil {
		t.Fatalf("rows without a thumbnail must pass: %v", err)
	}
}

package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestPhotoThumbnailFileMigration(t *testing.T) {
	db, err := sql.Open("sqlite", sqliteDSN(filepath.Join(t.TempDir(), "thumb.db"), true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrateTo(t, db, 29)
	if _, err := db.Exec("INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO contacts (id, user_id, firstname, photo_thumbnail) VALUES (1, 1, 'Zoe', 'data:image/jpeg;base64,AAAA')"); err != nil {
		t.Fatal(err)
	}

	// Up: the new column is empty, the old data is untouched
	migrateTo(t, db, 30)
	var file, legacy string
	if err := db.QueryRow("SELECT photo_thumbnail_file, photo_thumbnail FROM contacts WHERE id = 1").Scan(&file, &legacy); err != nil {
		t.Fatal(err)
	}
	if file != "" || legacy != "data:image/jpeg;base64,AAAA" {
		t.Fatalf("file = %q, legacy = %q", file, legacy)
	}

	// Down: the column goes, the legacy thumbnail and the contact stay
	migrateTo(t, db, 29)
	if columnExists(t, db, "contacts", "photo_thumbnail_file") {
		t.Fatal("photo_thumbnail_file still present after down")
	}
	if err := db.QueryRow("SELECT photo_thumbnail FROM contacts WHERE id = 1").Scan(&legacy); err != nil || legacy == "" {
		t.Fatalf("legacy thumbnail after down = %q (err %v)", legacy, err)
	}
}

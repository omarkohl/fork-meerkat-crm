package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSessionsAndAuditLogMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.db")
	db, err := sql.Open("sqlite", sqliteDSN(path, true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrateTo(t, db, 26)
	if _, err := db.Exec("INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')"); err != nil {
		t.Fatal(err)
	}

	// Up on an existing user: version 0, tables usable
	migrateTo(t, db, 27)
	var version int
	if err := db.QueryRow("SELECT token_version FROM users WHERE id = 1").Scan(&version); err != nil || version != 0 {
		t.Fatalf("token_version = %d (err %v), want 0", version, err)
	}
	for _, q := range []string{
		"INSERT INTO sessions (user_id, public_id, expires_at) VALUES (1, 'abc', '2030-01-01')",
		"INSERT INTO audit_logs (event) VALUES ('login')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// public_id is unique
	if _, err := db.Exec("INSERT INTO sessions (user_id, public_id, expires_at) VALUES (1, 'abc', '2030-01-01')"); err == nil {
		t.Fatal("duplicate public_id accepted")
	}

	// Down: tables and column go, the user stays
	migrateTo(t, db, 26)
	if columnExists(t, db, "users", "token_version") {
		t.Fatal("token_version still present after down")
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil || n != 1 {
		t.Fatalf("users after down = %d (err %v)", n, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&n); err == nil {
		t.Fatal("sessions table still exists after down")
	}
}

package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestTOTPMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "totp.db")
	db, err := sql.Open("sqlite", sqliteDSN(path, true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrateTo(t, db, 27)
	if _, err := db.Exec("INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')"); err != nil {
		t.Fatal(err)
	}

	// Up: existing users have no 2FA
	migrateTo(t, db, 28)
	var secret string
	var enabled bool
	var step int64
	if err := db.QueryRow("SELECT totp_secret, totp_enabled, totp_last_step FROM users WHERE id = 1").Scan(&secret, &enabled, &step); err != nil {
		t.Fatal(err)
	}
	if secret != "" || enabled || step != 0 {
		t.Fatalf("defaults = %q %v %d", secret, enabled, step)
	}
	if _, err := db.Exec("INSERT INTO recovery_codes (user_id, code_hash) VALUES (1, 'h')"); err != nil {
		t.Fatal(err)
	}

	// Down: columns and table go, the user stays
	migrateTo(t, db, 27)
	for _, col := range []string{"totp_secret", "totp_enabled", "totp_last_step"} {
		if columnExists(t, db, "users", col) {
			t.Fatalf("%s still present after down", col)
		}
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil || n != 1 {
		t.Fatalf("users after down = %d (err %v)", n, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM recovery_codes").Scan(&n); err == nil {
		t.Fatal("recovery_codes still exists after down")
	}
}

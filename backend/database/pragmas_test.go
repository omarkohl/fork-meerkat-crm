package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestInitDBSetsPragmasOnEveryConnection(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	// Hold several connections at once so the pool opens more than one.
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		conn, err := sqlDB.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		want := map[string]string{
			"journal_mode": "wal",
			"busy_timeout": "5000",
			"foreign_keys": "1",
			"synchronous":  "1", // NORMAL
		}
		for pragma, expected := range want {
			var got string
			if err := conn.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&got); err != nil {
				t.Fatalf("PRAGMA %s: %v", pragma, err)
			}
			if got != expected {
				t.Errorf("conn %d: PRAGMA %s = %q, want %q", i, pragma, got, expected)
			}
		}
	}
}

func TestSQLiteDSN(t *testing.T) {
	if got, want := sqliteDSN("a.db", true), "a.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := sqliteDSN("file:a.db?mode=rw", false), "file:a.db?mode=rw&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestForeignKeyViolationsReportsOrphans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fk.db")
	sqlDB, err := sql.Open("sqlite", sqliteDSN(path, false))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := RunMigrations(sqlDB); err != nil {
		t.Fatal(err)
	}
	if got, err := foreignKeyViolations(sqlDB); err != nil || len(got) != 0 {
		t.Fatalf("clean db: got %v, err %v", got, err)
	}
	if _, err := sqlDB.Exec("INSERT INTO webhooks (user_id, name, url, secret) VALUES (9999, 'n', 'http://x', 's')"); err != nil {
		t.Fatal(err)
	}
	got, err := foreignKeyViolations(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if got["webhooks -> users"] != 1 {
		t.Errorf("got %v, want one webhooks -> users violation", got)
	}
}

package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOutboxMigration(t *testing.T) {
	db, err := sql.Open("sqlite", sqliteDSN(filepath.Join(t.TempDir(), "outbox.db"), true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrateTo(t, db, 31)
	for _, q := range []string{
		"INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')",
		"INSERT INTO webhooks (id, user_id, name, url, secret) VALUES (1, 1, 'w', 'http://x', 's')",
		// succeeded / waiting for a retry / failed for good
		"INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, status_code, attempts) VALUES (1, 1, 'e', '{}', 204, 1)",
		"INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, error, attempts, next_retry_at) VALUES (2, 1, 'e', '{}', 'boom', 1, '2030-01-01')",
		"INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, error, attempts) VALUES (3, 1, 'e', '{}', 'boom', 3)",
		// a failed try that a later try delivered, and a chain that failed to the end
		"INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, error, attempts) VALUES (4, 1, 'e', 'p2', 'boom', 1)",
		"INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, status_code, attempts) VALUES (5, 1, 'e', 'p2', 204, 2)",
		"INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, error, attempts) VALUES (6, 1, 'e', 'p3', 'boom', 1)",
		"INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, error, attempts) VALUES (7, 1, 'e', 'p3', 'boom', 2)",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	migrateTo(t, db, 32)
	want := map[int]string{1: "delivered", 2: "pending", 3: "dead", 4: "delivered", 5: "delivered", 6: "delivered", 7: "dead"}
	for id, status := range want {
		var got string
		if err := db.QueryRow("SELECT status FROM webhook_deliveries WHERE id = ?", id).Scan(&got); err != nil || got != status {
			t.Errorf("delivery %d: status %q (err %v), want %q", id, got, err, status)
		}
	}
	if _, err := db.Exec("INSERT INTO email_outbox (to_addr, subject, html, next_attempt_at) VALUES ('a@b.c', 's', 'h', '2030-01-01')"); err != nil {
		t.Fatalf("email_outbox: %v", err)
	}

	migrateTo(t, db, 31)
	if columnExists(t, db, "webhook_deliveries", "status") || columnExists(t, db, "webhook_deliveries", "locked_at") {
		t.Fatal("outbox columns still present after down")
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM webhook_deliveries").Scan(&n); err != nil || n != 7 {
		t.Fatalf("deliveries after down: %d (err %v)", n, err)
	}
	if _, err := db.Exec("SELECT 1 FROM email_outbox"); err == nil {
		t.Fatal("email_outbox still present after down")
	}
}

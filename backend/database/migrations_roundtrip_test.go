package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// queryEach runs a query and calls scan for every row; the rows are closed on return.
func queryEach(t *testing.T, db *sql.DB, query string, scan func(*sql.Rows) error) {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			t.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

// schemaSnapshot describes every table (columns with type, nullability, default and
// primary key position, foreign keys) and every index, independent of the order the
// columns were added in and of the exact SQL text SQLite keeps after ALTER TABLE.
func schemaSnapshot(t *testing.T, db *sql.DB) []string {
	t.Helper()
	type obj struct{ typ, name, table string }
	var objs []obj
	queryEach(t, db, `SELECT type, name, tbl_name FROM sqlite_master
		WHERE name NOT LIKE 'sqlite_%' AND tbl_name <> 'schema_migrations' ORDER BY type, name`, func(r *sql.Rows) error {
		var o obj
		err := r.Scan(&o.typ, &o.name, &o.table)
		objs = append(objs, o)
		return err
	})

	var out []string
	for _, o := range objs {
		out = append(out, fmt.Sprintf("%s %s", o.typ, o.name))
		switch o.typ {
		case "table":
			var lines []string
			queryEach(t, db, fmt.Sprintf("SELECT name, type, \"notnull\", dflt_value, pk FROM pragma_table_info('%s')", o.name), func(r *sql.Rows) error {
				var name, typ string
				var notnull, pk int
				var dflt sql.NullString
				err := r.Scan(&name, &typ, &notnull, &dflt, &pk)
				lines = append(lines, fmt.Sprintf("  col %s %s notnull=%d default=%s pk=%d", name, typ, notnull, dflt.String, pk))
				return err
			})
			queryEach(t, db, fmt.Sprintf("SELECT \"from\", \"table\", \"to\", on_delete FROM pragma_foreign_key_list('%s')", o.name), func(r *sql.Rows) error {
				var from, table, to, onDelete string
				err := r.Scan(&from, &table, &to, &onDelete)
				lines = append(lines, fmt.Sprintf("  fk %s -> %s.%s on_delete=%s", from, table, to, onDelete))
				return err
			})
			sort.Strings(lines)
			out = append(out, lines...)
		case "index":
			var names []string
			queryEach(t, db, fmt.Sprintf("SELECT name FROM pragma_index_info('%s') ORDER BY seqno", o.name), func(r *sql.Rows) error {
				var n sql.NullString
				err := r.Scan(&n)
				names = append(names, n.String)
				return err
			})
			var unique int
			if err := db.QueryRow("SELECT \"unique\" FROM pragma_index_list(?) WHERE name = ?", o.table, o.name).Scan(&unique); err != nil {
				t.Fatal(err)
			}
			out = append(out, fmt.Sprintf("  on %s(%s) unique=%d", o.table, strings.Join(names, ","), unique))
		}
	}
	return out
}

func newMigrator(t *testing.T, db *sql.DB) *migrate.Migrate {
	t.Helper()
	driver, err := withInstance(db, &sqliteConfig{})
	if err != nil {
		t.Fatal(err)
	}
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// seedAllTables puts a row into the main tables of the current schema, so the down
// migrations have data to cope with (dropped columns, recreated tables).
func seedAllTables(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, q := range []string{
		"INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')",
		"INSERT INTO contacts (id, user_id, firstname, lastname, email, birthday, circles) VALUES (1, 1, 'Ada', 'Lovelace', 'ada@example.com', '1815-12-10', 'Friends')",
		"INSERT INTO contacts (id, user_id, firstname, deleted_at) VALUES (2, 1, 'Trashed', CURRENT_TIMESTAMP)",
		"INSERT INTO notes (id, user_id, contact_id, content, date) VALUES (1, 1, 1, 'note', CURRENT_TIMESTAMP)",
		"INSERT INTO activities (id, user_id, title, date) VALUES (1, 1, 'Lunch', CURRENT_TIMESTAMP)",
		"INSERT INTO activity_contacts (activity_id, contact_id) VALUES (1, 1)",
		"INSERT INTO relationships (id, user_id, contact_id, related_contact_id, name, type) VALUES (1, 1, 1, 2, 'friend', 'friend')",
		"INSERT INTO reminders (id, user_id, contact_id, message, remind_at, recurrence) VALUES (1, 1, 1, 'call', CURRENT_TIMESTAMP, 'once')",
		"INSERT INTO reminder_completions (id, user_id, contact_id, message, completed_at) VALUES (1, 1, 1, 'call', CURRENT_TIMESTAMP)",
		"INSERT INTO api_tokens (id, user_id, name, token_hash) VALUES (1, 1, 't', 'hash')",
		"INSERT INTO webhooks (id, user_id, name, url, secret, events) VALUES (1, 1, 'w', 'https://example.com', 's', '[\"contact.created\"]')",
		"INSERT INTO calendar_subscriptions (id, user_id, name, url) VALUES (1, 1, 'cal', 'https://example.com/c.ics')",
		"INSERT INTO carddav_connections (id, user_id, base_url, address_book_path) VALUES (1, 1, 'https://x', '/ab')",
		"INSERT INTO carddav_contact_links (connection_id, user_id, contact_id, remote_uid, remote_path) VALUES (1, 1, 1, 'uid', '/ab/uid.vcf')",
		"INSERT INTO ics_feeds (user_id, token_hash, last_fetched_at) VALUES (1, 'feedhash', CURRENT_TIMESTAMP)",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
}

// Every migration must apply, roll back and apply again on a database that holds data,
// and the schema after up/down/up must equal the one after the first up. Foreign keys
// are off, as in InitDB, because migrations may recreate tables.
func TestMigrationsRoundTripOnSeededDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roundtrip.db")
	db, err := sql.Open("sqlite", sqliteDSN(path, false))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("first up: %v", err)
	}
	first := schemaSnapshot(t, db)
	if len(first) == 0 {
		t.Fatal("empty schema after up")
	}
	seedAllTables(t, db)

	m := newMigrator(t, db)
	if err := m.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}
	if rest := schemaSnapshot(t, db); len(rest) != 0 {
		t.Errorf("objects left after rolling everything back: %v", rest)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("second up: %v", err)
	}
	second := schemaSnapshot(t, db)
	if strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Errorf("schema differs after up/down/up\nfirst:\n%s\n\nsecond:\n%s", strings.Join(first, "\n"), strings.Join(second, "\n"))
	}
}

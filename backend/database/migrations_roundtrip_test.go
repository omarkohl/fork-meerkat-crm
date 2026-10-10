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

// seedStatements fill the main tables with one or two rows each. last_fetched_at is left
// out of the ics_feeds row so the same statements work on older schemas.
var seedStatements = []struct{ table, query string }{
	{"users", "INSERT INTO users (id, username, email, password) VALUES (1, 'u', 'u@example.com', 'x')"},
	{"contacts", "INSERT INTO contacts (id, user_id, firstname, lastname, email, birthday, circles) VALUES (1, 1, 'Ada', 'Lovelace', 'ada@example.com', '1815-12-10', 'Friends')"},
	{"contacts", "INSERT INTO contacts (id, user_id, firstname, deleted_at) VALUES (2, 1, 'Trashed', CURRENT_TIMESTAMP)"},
	{"notes", "INSERT INTO notes (id, user_id, contact_id, content, date) VALUES (1, 1, 1, 'note', CURRENT_TIMESTAMP)"},
	{"activities", "INSERT INTO activities (id, user_id, title, date) VALUES (1, 1, 'Lunch', CURRENT_TIMESTAMP)"},
	{"activity_contacts", "INSERT INTO activity_contacts (activity_id, contact_id) VALUES (1, 1)"},
	{"relationships", "INSERT INTO relationships (id, user_id, contact_id, related_contact_id, name, type) VALUES (1, 1, 1, 2, 'friend', 'friend')"},
	{"reminders", "INSERT INTO reminders (id, user_id, contact_id, message, remind_at, recurrence) VALUES (1, 1, 1, 'call', CURRENT_TIMESTAMP, 'once')"},
	{"reminder_completions", "INSERT INTO reminder_completions (id, user_id, contact_id, message, completed_at) VALUES (1, 1, 1, 'call', CURRENT_TIMESTAMP)"},
	{"api_tokens", "INSERT INTO api_tokens (id, user_id, name, token_hash) VALUES (1, 1, 't', 'hash')"},
	{"webhooks", "INSERT INTO webhooks (id, user_id, name, url, secret, events) VALUES (1, 1, 'w', 'https://example.com', 's', '[\"contact.created\"]')"},
	{"calendar_subscriptions", "INSERT INTO calendar_subscriptions (id, user_id, name, url) VALUES (1, 1, 'cal', 'https://example.com/c.ics')"},
	{"carddav_connections", "INSERT INTO carddav_connections (id, user_id, base_url, address_book_path) VALUES (1, 1, 'https://x', '/ab')"},
	{"carddav_contact_links", "INSERT INTO carddav_contact_links (connection_id, user_id, contact_id, remote_uid, remote_path) VALUES (1, 1, 1, 'uid', '/ab/uid.vcf')"},
	{"ics_feeds", "INSERT INTO ics_feeds (user_id, token_hash) VALUES (1, 'feedhash')"},
}

func tableNames(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	queryEach(t, db, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations'", func(r *sql.Rows) error {
		var n string
		err := r.Scan(&n)
		names[n] = true
		return err
	})
	return names
}

// seed inserts the rows of every table that exists in the current schema.
func seed(t *testing.T, db *sql.DB) {
	t.Helper()
	existing := tableNames(t, db)
	for _, st := range seedStatements {
		if !existing[st.table] {
			continue
		}
		if _, err := db.Exec(st.query); err != nil {
			t.Fatalf("seed %q: %v", st.query, err)
		}
	}
}

// rowCounts returns the number of rows of every seeded table that exists.
func rowCounts(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	existing := tableNames(t, db)
	counts := map[string]int{}
	for _, st := range seedStatements {
		if !existing[st.table] {
			continue
		}
		if _, done := counts[st.table]; done {
			continue
		}
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + st.table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		counts[st.table] = n
	}
	return counts
}

func latestVersion(t *testing.T, db *sql.DB) uint {
	t.Helper()
	v, dirty, err := newMigrator(t, db).Version()
	if err != nil || dirty {
		t.Fatalf("version = %d, dirty = %v, err = %v", v, dirty, err)
	}
	return v
}

func openMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	// Foreign keys are off, as in InitDB, because migrations may recreate tables.
	db, err := sql.Open("sqlite", sqliteDSN(filepath.Join(t.TempDir(), "migrations.db"), false))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// Every migration is rolled back and re-applied one step at a time on a database that
// holds data: the schema after each up step must equal the schema recorded on the way
// down, and a rollback must not lose rows of tables that survive it.
func TestMigrationsRoundTripStepByStepOnSeededDatabase(t *testing.T) {
	db := openMigrationDB(t)
	if err := RunMigrations(db); err != nil {
		t.Fatalf("first up: %v", err)
	}
	latest := latestVersion(t, db)
	seed(t, db)
	m := newMigrator(t, db)

	snapshots := map[uint]string{}
	for v := latest; v >= 1; v-- {
		snapshots[v] = withoutKnownLeftovers(v, strings.Join(schemaSnapshot(t, db), "\n"))
		before := rowCounts(t, db)
		if err := m.Steps(-1); err != nil {
			t.Fatalf("down from version %d: %v", v, err)
		}
		for table, n := range rowCounts(t, db) {
			if before[table] != n {
				t.Errorf("down from version %d: table %s has %d rows, had %d", v, table, n, before[table])
			}
		}
	}
	if rest := schemaSnapshot(t, db); len(rest) != 0 {
		t.Errorf("objects left after rolling everything back: %v", rest)
	}

	for v := uint(1); v <= latest; v++ {
		if err := m.Steps(1); err != nil {
			t.Fatalf("up to version %d: %v", v, err)
		}
		if got := withoutKnownLeftovers(v, strings.Join(schemaSnapshot(t, db), "\n")); got != snapshots[v] {
			t.Errorf("schema at version %d differs after down/up:\n%s", v, lineDiff(snapshots[v], got))
		}
	}
}

// Rows written under an older schema must survive the migrations that follow it.
func TestMigrationsUpKeepsRowsSeededAtOlderVersions(t *testing.T) {
	for _, start := range []uint{12, 20, 23, 24} {
		t.Run(fmt.Sprintf("from_v%d", start), func(t *testing.T) {
			db := openMigrationDB(t)
			migrateTo(t, db, start)
			seed(t, db)
			before := rowCounts(t, db)
			if len(before) == 0 {
				t.Fatal("nothing was seeded")
			}

			if err := RunMigrations(db); err != nil {
				t.Fatalf("up from version %d: %v", start, err)
			}
			after := rowCounts(t, db)
			for table, n := range before {
				if after[table] != n {
					t.Errorf("table %s has %d rows after up, had %d", table, after[table], n)
				}
			}
			// The newest schema accepts the data: spot-check a migrated column.
			var name string
			if err := db.QueryRow("SELECT firstname FROM contacts WHERE id = 1").Scan(&name); err != nil || name != "Ada" {
				t.Errorf("contact after up = %q (err %v), want Ada", name, err)
			}
		})
	}
}

// lineDiff lists the lines only in one of two newline-separated snapshots.
func lineDiff(before, after string) string {
	seen := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, l := range strings.Split(s, "\n") {
			m[l] = true
		}
		return m
	}
	b, a := seen(before), seen(after)
	var out []string
	for _, l := range strings.Split(before, "\n") {
		if !a[l] {
			out = append(out, "- "+l)
		}
	}
	for _, l := range strings.Split(after, "\n") {
		if !b[l] {
			out = append(out, "+ "+l)
		}
	}
	return strings.Join(out, "\n")
}

// withoutKnownLeftovers drops, for schema versions below 8, the three contacts columns
// that the down migration of 000008 deliberately leaves behind ("SQLite limitation" in
// its comment; migrations are never edited, and a down migration cannot be fixed by a
// new one). Rolling back to version 7 or lower therefore keeps them, while a fresh
// database at that version does not have them.
func withoutKnownLeftovers(version uint, snapshot string) string {
	if version >= 8 {
		return snapshot
	}
	var kept []string
	for _, l := range strings.Split(snapshot, "\n") {
		switch strings.TrimSpace(l) {
		case "col etag TEXT notnull=0 default= pk=0", "col vcard_extra TEXT notnull=0 default= pk=0", "col vcard_uid TEXT notnull=0 default= pk=0":
			continue
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n")
}

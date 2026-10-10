package database

import (
	"database/sql"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// migrateTo moves the database to an explicit schema version, up or down, so
// migration tests do not depend on which migration happens to be the newest.
func migrateTo(t *testing.T, db *sql.DB, version uint) {
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
	if err := m.Migrate(version); err != nil && err != migrate.ErrNoChange {
		t.Fatal(err)
	}
}

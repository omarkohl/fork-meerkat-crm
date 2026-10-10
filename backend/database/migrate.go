package database

import (
	"database/sql"
	"embed"
	"fmt"
	"meerkat/logger"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// sqliteDSN appends the connection pragmas. They are set per connection (via
// the driver's _pragma parameter) because busy_timeout, foreign_keys and
// synchronous do not persist; journal_mode=WAL persists in the file.
func sqliteDSN(path string, foreignKeys bool) string {
	pragmas := []string{"journal_mode(WAL)", "busy_timeout(5000)", "synchronous(NORMAL)"}
	if foreignKeys {
		pragmas = append(pragmas, "foreign_keys(1)")
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	for _, p := range pragmas {
		path += sep + "_pragma=" + p
		sep = "&"
	}
	return path
}

// foreignKeyViolations runs PRAGMA foreign_key_check and returns the number of
// violating rows per "table -> parent". It contains no row data.
func foreignKeyViolations(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var table, parent string
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return nil, err
		}
		counts[table+" -> "+parent]++
	}
	return counts, rows.Err()
}

func warnForeignKeyViolations(db *sql.DB) {
	counts, err := foreignKeyViolations(db)
	if err != nil {
		logger.Warn().Err(err).Msg("Could not run foreign key check")
		return
	}
	for rel, n := range counts {
		logger.Warn().Str("relation", rel).Int("rows", n).Msg("Foreign key violations found (orphan rows); writes touching them may fail now that foreign keys are enforced")
	}
}

// InitDB initializes the database connection and runs migrations
func InitDB(dbPath string) (*gorm.DB, error) {
	// Open database connection for migrations
	// Foreign keys stay off here: migrations may recreate tables, which with
	// enforcement on would cascade deletes or fail on dependants.
	sqlDB, err := sql.Open("sqlite", sqliteDSN(dbPath, false))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Run migrations
	err = RunMigrations(sqlDB)
	if err == nil {
		// Databases created while foreign keys were off may hold orphan rows.
		warnForeignKeyViolations(sqlDB)
	}
	_ = sqlDB.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Open GORM connection
	db, err := gorm.Open(sqlite.Open(sqliteDSN(dbPath, true)), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect with GORM: %w", err)
	}

	return db, nil
}

// RunMigrations runs all pending database migrations
func RunMigrations(db *sql.DB) error {
	driver, err := withInstance(db, &sqliteConfig{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if dirty {
		logger.Warn().Uint("version", version).Msg("Database is in dirty state, forcing version")
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force version: %w", err)
		}
	}

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Get final version
	version, _, err = m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get final version: %w", err)
	}

	if err == migrate.ErrNilVersion {
		logger.Info().Msg("No migrations applied (database is empty)")
	} else {
		logger.Info().Uint("version", version).Msg("Migrations applied successfully")
	}

	return nil
}

// MigrateDown rolls back the last migration (use with caution)
func MigrateDown(dbPath string) error {
	sqlDB, err := sql.Open("sqlite", sqliteDSN(dbPath, false))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer sqlDB.Close()

	driver, err := withInstance(sqlDB, &sqliteConfig{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	logger.Info().Msg("Migration rolled back successfully")
	return nil
}

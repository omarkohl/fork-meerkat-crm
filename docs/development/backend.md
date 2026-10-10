---
title: Backend Development
parent: Development
nav_order: 2
---

# Backend Development

## Setup

Requires current [Go version](https://go.dev/doc/install).

```sh
cd backend
cp .env.example .env   # then edit with your values
go mod tidy
```

## Running Locally

```sh
source .env
go run main.go
```

The server starts on `PORT` (default `8080`). Migrations run automatically.

## Adding a New Endpoint

1. Define an input DTO in `models/` with validation tags.
2. Add a controller function in `controllers/`.
3. Register the route in `routes/routes.go`.
4. Add the validation schema to the middleware registration if needed.

## Database Migrations

### Creating a Migration

```sh
make migrate-create NAME=add_foo_column
```

This creates two files in `database/migrations/`: `NNNNNN_add_foo_column.up.sql` and `.down.sql`.

### Dropping the legacy thumbnail column

`contacts.photo_thumbnail` (base64 data URL) is kept only as the copy a downgrade needs. `services.ExportLegacyThumbnails` moves it into files at startup, but it runs after the migrations. A migration that drops the column (`DROP COLUMN photo_thumbnail`) must therefore never be the first thing a database sees after the export-less version: ship it at least one release after the export, and never edit that rule away. `RunMigrations` enforces it (`checkLegacyThumbnailsExported`): it refuses to start when a pending migration drops the column while some contact still has a `data:` thumbnail and no `photo_thumbnail_file`, and the error says to start the previous version once. Keep the `DROP COLUMN photo_thumbnail` spelling so the check recognises the migration. The same release removes `LegacyThumbnail`, `serveLegacyThumbnail` and the fallback in `ContactToVCard`.

### SQLite settings

The database is opened in WAL mode with `busy_timeout`, `synchronous=NORMAL` and foreign keys on (see `sqliteDSN` in `database/migrate.go`). Migrations run on a separate connection without foreign key enforcement because they may recreate tables. Never copy `meerkat.db` while the app runs; use the built-in [backups](../deployment.html#backups) (`VACUUM INTO`).

### Search index (FTS5)

Search uses SQLite FTS5 tables `contacts_fts`, `notes_fts` and `activities_fts` (migration 29). Each has the id of the indexed row as its rowid and is kept in sync by `AFTER INSERT/UPDATE/DELETE` triggers on the real table; the tokenizer is `unicode61 remove_diacritics 2`. The index is not scoped by user: queries always join back to the real table and filter `user_id` and `deleted_at` there. `search.MatchQuery` quotes every typed word as a prefix term, so user input can never be FTS5 syntax, caps the input (200 characters, 32 words) and hands phone-like digits and CJK text back to the LIKE code (`ok == false`). `search.Available` remembers per database whether an index exists, so create the tables before the first search on a database.

- **Changing an indexed column** (the list in the trigger's `UPDATE OF`) or **recreating `contacts`, `notes` or `activities`** (SQLite drops triggers with the table) means the migration must recreate the triggers and rebuild the index: `DELETE FROM contacts_fts;` followed by the `INSERT ... SELECT` from migration 29.
- When the tables do not exist (for example the in-memory test databases built with `AutoMigrate`) the controllers fall back to the old `LIKE` search, so both paths stay covered by tests; `controllers/search_fts_test.go` builds the index from the migration file.

### Running Migrations

Migrations run automatically on startup. To apply or roll back manually:

```sh
make migrate-up
make migrate-down
make migrate-status
```

## Models and DTOs

GORM models (`Contact`, `Activity`, etc.) live alongside input DTOs (`ContactInput`, etc.) in `models/`. DTOs are what controllers receive after validation while models are what GORM persists.

All models include `UserID uint` for tenant isolation.

## Services

Complex business logic lives in `services/`. Controllers call services; services own multi-step operations (e.g. sending emails). Services receive a `*gorm.DB` and any needed config, they do not access `*gin.Context`.

## Testing

```sh
go test ./...
```

Tests use an in-memory SQLite database (auto-migrated). See [Testing](testing.md).

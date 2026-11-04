package todo

import (
	"database/sql"
	"fmt"
	"time"
)

// Migrate applies schema migrations to db. Safe to call multiple times.
// Returns error if migration fails (caller should abort startup).
func Migrate(db *sql.DB) error {
	// schema_migrations must be created first
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations: %w", err)
	}

	// todos table definition
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS todos (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		title      TEXT    NOT NULL,
		notes      TEXT    NOT NULL DEFAULT '',
		completed  INTEGER NOT NULL DEFAULT 0,
		priority   TEXT    NOT NULL DEFAULT 'medium',
		due_date   TEXT    NOT NULL DEFAULT '',
		created_at TEXT    NOT NULL,
		updated_at TEXT    NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("failed to create todos: %w", err)
	}

	// Apply version 1 migration (schema already created above, just record it)
	if err := applyMigration(db, 1, func(tx *sql.Tx) error { return nil }); err != nil {
		return err
	}

	return nil
}

func applyMigration(db *sql.DB, version int, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var count int
	err = tx.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return tx.Commit()
	}

	if err := fn(tx); err != nil {
		return err
	}

	_, err = tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		version, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return err
	}

	return tx.Commit()
}

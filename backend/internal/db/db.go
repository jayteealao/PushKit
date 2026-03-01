package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	// Enable foreign keys.
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return db, nil
}

func CreateTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id               TEXT PRIMARY KEY,
		user_id          TEXT NOT NULL,
		s3_key           TEXT NOT NULL UNIQUE,
		original_filename TEXT NOT NULL,
		content_type     TEXT NOT NULL DEFAULT 'application/octet-stream',
		size_bytes       INTEGER,
		sha256           TEXT,
		tags_json        TEXT,
		created_at       TEXT NOT NULL,
		status           TEXT NOT NULL DEFAULT 'INITIATED'
	);

	CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
	CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at);
	CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);

	CREATE TABLE IF NOT EXISTS api_keys (
		key     TEXT PRIMARY KEY,
		user_id TEXT NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// DB wraps the raw SQL connection and is the type passed around to all
// database operations throughout the app.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the scout database at ~/.scout/scout.db.
// It creates the directory if it doesn't exist, applies performance
// pragmas, and runs all schema migrations. Safe to call on every startup.
func Open() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	scoutDir := filepath.Join(home, ".scout")
	if err := os.MkdirAll(scoutDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(scoutDir, "scout.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	// WAL mode: better concurrent read performance, prevents lock contention.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, err
	}
	// Foreign keys are off by default in SQLite — enable per connection.
	if _, err := conn.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return nil, err
	}

	if err := migrate(conn); err != nil {
		return nil, err
	}

	return &DB{conn: conn}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// migrate creates all tables and FTS5 triggers if they don't already exist.
// Using IF NOT EXISTS makes this safe to run on every startup (idempotent).
func migrate(conn *sql.DB) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op if Commit() is called below

	statements := []string{
		// Workspaces: directories the user has registered with scout watch add
		`CREATE TABLE IF NOT EXISTS workspaces (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			path     TEXT NOT NULL UNIQUE,
			name     TEXT NOT NULL,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Files: every individual file tracked by scout
		`CREATE TABLE IF NOT EXISTS files (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			path     TEXT NOT NULL UNIQUE,
			name     TEXT NOT NULL,
			preview  TEXT,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Tags: labels that can be applied to files
		`CREATE TABLE IF NOT EXISTS tags (
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		)`,

		// file_tags: the many-to-many link between files and tags
		`CREATE TABLE IF NOT EXISTS file_tags (
			file_id INTEGER REFERENCES files(id) ON DELETE CASCADE,
			tag_id  INTEGER REFERENCES tags(id)  ON DELETE CASCADE,
			PRIMARY KEY (file_id, tag_id)
		)`,

		// files_fts: the full-text search index over path, name, and preview.
		// content='files' means it mirrors the files table.
		// content_rowid='id' links FTS rows back to files rows by id.
		// IMPORTANT: this virtual table does NOT auto-update when files changes —
		// the three triggers below are required to keep it in sync.
		`CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(
			path, name, preview,
			content='files',
			content_rowid='id'
		)`,

		// Trigger: when a file is inserted, add it to the FTS index.
		`CREATE TRIGGER IF NOT EXISTS files_ai AFTER INSERT ON files BEGIN
			INSERT INTO files_fts(rowid, path, name, preview)
			VALUES (new.id, new.path, new.name, new.preview);
		END`,

		// Trigger: when a file is deleted, remove it from the FTS index.
		// The special ('delete', ...) syntax is FTS5-specific — not standard SQL.
		`CREATE TRIGGER IF NOT EXISTS files_ad AFTER DELETE ON files BEGIN
			INSERT INTO files_fts(files_fts, rowid, path, name, preview)
			VALUES ('delete', old.id, old.path, old.name, old.preview);
		END`,

		// Trigger: when a file is updated, replace its FTS entry.
		`CREATE TRIGGER IF NOT EXISTS files_au AFTER UPDATE ON files BEGIN
			INSERT INTO files_fts(files_fts, rowid, path, name, preview)
			VALUES ('delete', old.id, old.path, old.name, old.preview);
			INSERT INTO files_fts(rowid, path, name, preview)
			VALUES (new.id, new.path, new.name, new.preview);
		END`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

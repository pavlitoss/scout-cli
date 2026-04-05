package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// File represents a tracked file in the database.
type File struct {
	ID      int64
	Path    string
	Name    string
	Preview *string
	AddedAt time.Time
}

// UpsertFile inserts a file or updates name/preview if the path already exists.
// Uses ON CONFLICT DO UPDATE to preserve the existing ID and avoid breaking FK references.
func (d *DB) UpsertFile(path, name string, preview *string) (int64, error) {
	res, err := d.conn.Exec(`
		INSERT INTO files (path, name, preview)
		VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET name=excluded.name, preview=excluded.preview
	`, path, name, preview)
	if err != nil {
		return 0, fmt.Errorf("upsert file: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		// ON CONFLICT DO UPDATE doesn't always return the last insert id;
		// fall back to a SELECT.
		var existing int64
		if serr := d.conn.QueryRow(`SELECT id FROM files WHERE path = ?`, path).Scan(&existing); serr != nil {
			return 0, fmt.Errorf("upsert file lookup: %w", serr)
		}
		return existing, nil
	}
	return id, nil
}

// DeleteFile removes a file by its exact path.
func (d *DB) DeleteFile(path string) error {
	_, err := d.conn.Exec(`DELETE FROM files WHERE path = ?`, path)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

// DeleteFileByID removes a file by its database ID.
func (d *DB) DeleteFileByID(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM files WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete file by id: %w", err)
	}
	return nil
}

// DeleteFilesUnderPath removes all files whose path is exactly prefix or starts with prefix + "/".
// Returns the number of rows deleted.
func (d *DB) DeleteFilesUnderPath(prefix string) (int64, error) {
	res, err := d.conn.Exec(
		`DELETE FROM files WHERE path = ? OR path LIKE ?`,
		prefix,
		prefix+"/%",
	)
	if err != nil {
		return 0, fmt.Errorf("delete files under path: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// GetAllFiles returns every tracked file.
func (d *DB) GetAllFiles() ([]File, error) {
	rows, err := d.conn.Query(`SELECT id, path, name, preview, added_at FROM files ORDER BY path`)
	if err != nil {
		return nil, fmt.Errorf("get all files: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

// GetFilesUnderPath returns all files whose path is exactly prefix or starts with prefix + "/".
func (d *DB) GetFilesUnderPath(prefix string) ([]File, error) {
	rows, err := d.conn.Query(
		`SELECT id, path, name, preview, added_at FROM files WHERE path = ? OR path LIKE ? ORDER BY path`,
		prefix,
		prefix+"/%",
	)
	if err != nil {
		return nil, fmt.Errorf("get files under path: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

// GetFileByPath returns a single file by its exact path.
// Returns an error wrapping sql.ErrNoRows if not found.
func (d *DB) GetFileByPath(path string) (File, error) {
	row := d.conn.QueryRow(
		`SELECT id, path, name, preview, added_at FROM files WHERE path = ?`,
		path,
	)
	f, err := scanFile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return File{}, fmt.Errorf("file not found: %s: %w", path, err)
		}
		return File{}, fmt.Errorf("get file by path: %w", err)
	}
	return f, nil
}

// scanFiles reads all rows from a query into a []File slice.
func scanFiles(rows *sql.Rows) ([]File, error) {
	var files []File
	for rows.Next() {
		var f File
		var addedAt string
		if err := rows.Scan(&f.ID, &f.Path, &f.Name, &f.Preview, &addedAt); err != nil {
			return nil, err
		}
		f.AddedAt, _ = time.Parse("2006-01-02 15:04:05", addedAt)
		files = append(files, f)
	}
	return files, rows.Err()
}

// scanFile reads a single row into a File.
func scanFile(row *sql.Row) (File, error) {
	var f File
	var addedAt string
	err := row.Scan(&f.ID, &f.Path, &f.Name, &f.Preview, &addedAt)
	if err != nil {
		return File{}, err
	}
	f.AddedAt, _ = time.Parse("2006-01-02 15:04:05", addedAt)
	return f, nil
}

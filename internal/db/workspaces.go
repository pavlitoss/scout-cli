package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Workspace represents a watched directory registered with scout.
type Workspace struct {
	ID      int64
	Path    string
	Name    string
	AddedAt time.Time
}

// WorkspaceWithCount extends Workspace with a count of tracked files.
type WorkspaceWithCount struct {
	Workspace
	FileCount int
}

// AddWorkspace registers a directory as a workspace.
// Returns an error if the path is already registered.
func (d *DB) AddWorkspace(path, name string) error {
	_, err := d.conn.Exec(
		`INSERT INTO workspaces (path, name) VALUES (?, ?)`,
		path, name,
	)
	if err != nil {
		return fmt.Errorf("add workspace: %w", err)
	}
	return nil
}

// RemoveWorkspace unregisters a workspace by path.
func (d *DB) RemoveWorkspace(path string) error {
	res, err := d.conn.Exec(`DELETE FROM workspaces WHERE path = ?`, path)
	if err != nil {
		return fmt.Errorf("remove workspace: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("workspace not found: %s", path)
	}
	return nil
}

// GetAllWorkspaces returns all registered workspaces.
func (d *DB) GetAllWorkspaces() ([]Workspace, error) {
	rows, err := d.conn.Query(`SELECT id, path, name, added_at FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("get all workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []Workspace
	for rows.Next() {
		var w Workspace
		var addedAt string
		if err := rows.Scan(&w.ID, &w.Path, &w.Name, &addedAt); err != nil {
			return nil, err
		}
		w.AddedAt, _ = time.Parse("2006-01-02 15:04:05", addedAt)
		workspaces = append(workspaces, w)
	}
	return workspaces, rows.Err()
}

// GetWorkspaceByPath returns a single workspace by its path.
func (d *DB) GetWorkspaceByPath(path string) (Workspace, error) {
	var w Workspace
	var addedAt string
	err := d.conn.QueryRow(
		`SELECT id, path, name, added_at FROM workspaces WHERE path = ?`, path,
	).Scan(&w.ID, &w.Path, &w.Name, &addedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workspace{}, fmt.Errorf("workspace not found: %s: %w", path, err)
		}
		return Workspace{}, fmt.Errorf("get workspace: %w", err)
	}
	w.AddedAt, _ = time.Parse("2006-01-02 15:04:05", addedAt)
	return w, nil
}

// ListWorkspaces returns all workspaces with their file counts.
func (d *DB) ListWorkspaces() ([]WorkspaceWithCount, error) {
	rows, err := d.conn.Query(`
		SELECT w.id, w.path, w.name, w.added_at, COUNT(f.id) as file_count
		FROM workspaces w
		LEFT JOIN files f ON f.path = w.path OR f.path LIKE w.path || '/%'
		GROUP BY w.id
		ORDER BY w.name
	`)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()

	var results []WorkspaceWithCount
	for rows.Next() {
		var wc WorkspaceWithCount
		var addedAt string
		if err := rows.Scan(&wc.ID, &wc.Path, &wc.Name, &addedAt, &wc.FileCount); err != nil {
			return nil, err
		}
		wc.AddedAt, _ = time.Parse("2006-01-02 15:04:05", addedAt)
		results = append(results, wc)
	}
	return results, rows.Err()
}

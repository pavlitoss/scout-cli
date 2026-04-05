package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// Tag represents a user-defined label stored in the database.
// Names are stored with the @ prefix (e.g. "@myproject").
type Tag struct {
	ID   int64
	Name string
}

// TagWithMeta extends Tag with a file count and workspace flag.
type TagWithMeta struct {
	Tag
	FileCount   int
	IsWorkspace bool
}

// EnsureTag gets or creates a tag by name. Returns the tag ID.
func (d *DB) EnsureTag(name string) (int64, error) {
	_, err := d.conn.Exec(`INSERT OR IGNORE INTO tags (name) VALUES (?)`, name)
	if err != nil {
		return 0, fmt.Errorf("ensure tag insert: %w", err)
	}
	var id int64
	if err := d.conn.QueryRow(`SELECT id FROM tags WHERE name = ?`, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("ensure tag select: %w", err)
	}
	return id, nil
}

// TagFile links a file to a tag. Idempotent.
func (d *DB) TagFile(fileID, tagID int64) error {
	_, err := d.conn.Exec(
		`INSERT OR IGNORE INTO file_tags (file_id, tag_id) VALUES (?, ?)`,
		fileID, tagID,
	)
	if err != nil {
		return fmt.Errorf("tag file: %w", err)
	}
	return nil
}

// TagFilesUnderPath adds a tag to all files whose path is exactly prefix or starts with prefix + "/".
func (d *DB) TagFilesUnderPath(prefix string, tagID int64) error {
	_, err := d.conn.Exec(`
		INSERT OR IGNORE INTO file_tags (file_id, tag_id)
		SELECT id, ? FROM files WHERE path = ? OR path LIKE ?
	`, tagID, prefix, prefix+"/%")
	if err != nil {
		return fmt.Errorf("tag files under path: %w", err)
	}
	return nil
}

// UntagFile removes a specific tag from a specific file.
func (d *DB) UntagFile(fileID, tagID int64) error {
	_, err := d.conn.Exec(
		`DELETE FROM file_tags WHERE file_id = ? AND tag_id = ?`,
		fileID, tagID,
	)
	if err != nil {
		return fmt.Errorf("untag file: %w", err)
	}
	return nil
}

// UntagFilesUnderPath removes a tag from all files under a path prefix.
func (d *DB) UntagFilesUnderPath(prefix string, tagID int64) error {
	_, err := d.conn.Exec(`
		DELETE FROM file_tags
		WHERE tag_id = ?
		  AND file_id IN (
		      SELECT id FROM files WHERE path = ? OR path LIKE ?
		  )
	`, tagID, prefix, prefix+"/%")
	if err != nil {
		return fmt.Errorf("untag files under path: %w", err)
	}
	return nil
}

// GetTagByName returns a tag by name. Returns an error wrapping sql.ErrNoRows if not found.
func (d *DB) GetTagByName(name string) (Tag, error) {
	var t Tag
	err := d.conn.QueryRow(`SELECT id, name FROM tags WHERE name = ?`, name).Scan(&t.ID, &t.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Tag{}, fmt.Errorf("tag not found: %s: %w", name, err)
		}
		return Tag{}, fmt.Errorf("get tag by name: %w", err)
	}
	return t, nil
}

// ListTags returns all tags with file counts and a flag indicating workspace tags.
func (d *DB) ListTags() ([]TagWithMeta, error) {
	rows, err := d.conn.Query(`
		SELECT t.id, t.name,
		       COUNT(ft.file_id) as file_count,
		       CASE WHEN w.id IS NOT NULL THEN 1 ELSE 0 END as is_workspace
		FROM tags t
		LEFT JOIN file_tags ft ON ft.tag_id = t.id
		LEFT JOIN workspaces w ON '@' || w.name = t.name
		GROUP BY t.id
		ORDER BY t.name
	`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var tags []TagWithMeta
	for rows.Next() {
		var tm TagWithMeta
		var isWS int
		if err := rows.Scan(&tm.ID, &tm.Name, &tm.FileCount, &isWS); err != nil {
			return nil, err
		}
		tm.IsWorkspace = isWS == 1
		tags = append(tags, tm)
	}
	return tags, rows.Err()
}

// GetFilesByTag returns all files associated with a tag name.
func (d *DB) GetFilesByTag(tagName string) ([]File, error) {
	rows, err := d.conn.Query(`
		SELECT f.id, f.path, f.name, f.preview, f.added_at
		FROM files f
		JOIN file_tags ft ON ft.file_id = f.id
		JOIN tags t ON t.id = ft.tag_id
		WHERE t.name = ?
		ORDER BY f.path
	`, tagName)
	if err != nil {
		return nil, fmt.Errorf("get files by tag: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

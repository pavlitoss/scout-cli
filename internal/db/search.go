package db

import (
	"fmt"
	"strings"
)

// SearchResult pairs a File with its BM25 relevance rank.
// Note: bm25() in SQLite FTS5 returns negative values — more negative = more relevant.
// Results are returned ordered by rank ASC (most relevant first).
type SearchResult struct {
	File
	Rank float64
}

// Search runs an FTS5 query over all tracked files, ranked by BM25.
func (d *DB) Search(query string) ([]SearchResult, error) {
	ftsQuery := buildFTSQuery(query)
	rows, err := d.conn.Query(`
		SELECT f.id, f.path, f.name, f.preview, f.added_at, bm25(files_fts) as rank
		FROM files_fts
		JOIN files f ON f.id = files_fts.rowid
		WHERE files_fts MATCH ?
		ORDER BY rank
		LIMIT 200
	`, ftsQuery)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	return scanSearchResults(rows)
}

// SearchByTag runs an FTS5 query scoped to files with a given tag name.
func (d *DB) SearchByTag(tagName, query string) ([]SearchResult, error) {
	ftsQuery := buildFTSQuery(query)
	rows, err := d.conn.Query(`
		SELECT f.id, f.path, f.name, f.preview, f.added_at, bm25(files_fts) as rank
		FROM files_fts
		JOIN files f ON f.id = files_fts.rowid
		JOIN file_tags ft ON ft.file_id = f.id
		JOIN tags t ON t.id = ft.tag_id
		WHERE files_fts MATCH ?
		  AND t.name = ?
		ORDER BY rank
		LIMIT 200
	`, ftsQuery, tagName)
	if err != nil {
		return nil, fmt.Errorf("search by tag: %w", err)
	}
	defer rows.Close()
	return scanSearchResults(rows)
}

// buildFTSQuery transforms raw user input into a valid FTS5 query string.
// Each term gets a trailing * for prefix matching. Terms with special chars are quoted.
func buildFTSQuery(raw string) string {
	terms := strings.Fields(raw)
	if len(terms) == 0 {
		return ""
	}

	ftsSpecialChars := `"()-+^`
	ftsKeywords := map[string]bool{"OR": true, "AND": true, "NOT": true}

	var parts []string
	for _, term := range terms {
		// Strip any trailing * the user typed
		term = strings.TrimRight(term, "*")
		if term == "" {
			continue
		}

		upper := strings.ToUpper(term)
		needsQuote := ftsKeywords[upper] || strings.ContainsAny(term, ftsSpecialChars)

		if needsQuote {
			// Escape any internal double-quotes by doubling them
			escaped := strings.ReplaceAll(term, `"`, `""`)
			parts = append(parts, `"`+escaped+`"*`)
		} else {
			parts = append(parts, term+"*")
		}
	}

	return strings.Join(parts, " ")
}

func scanSearchResults(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]SearchResult, error) {
	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		var preview *string
		var addedAt string
		if err := rows.Scan(&sr.ID, &sr.Path, &sr.Name, &preview, &addedAt, &sr.Rank); err != nil {
			return nil, err
		}
		sr.Preview = preview
		results = append(results, sr)
	}
	return results, rows.Err()
}

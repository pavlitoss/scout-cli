# Scout CLI — Implementation Plan

## Table of Contents

1. [Cross-Cutting Concerns](#cross-cutting-concerns)
2. [Phase 0: Project Scaffold](#phase-0-project-scaffold)
3. [Phase 1: Watch System](#phase-1-watch-system)
4. [Phase 2: Keyword Search (FTS5)](#phase-2-keyword-search-fts5)
5. [Phase 3: Tag System](#phase-3-tag-system)
6. [Phase 4: Prune](#phase-4-prune)
7. [Phase 5: UI Polish](#phase-5-ui-polish)
8. [Phase 6: Distribution](#phase-6-distribution)
9. [Dependency Reference](#dependency-reference)
10. [File Layout Reference](#appendix-a-file-layout-reference)
11. [Tricky Parts Summary](#appendix-b-tricky-parts-summary)

---

## Cross-Cutting Concerns

These rules apply to every phase and every file. Read them before writing any code.

### Path Normalization

All file and directory paths stored in the database must be absolute and clean. Every path received from the user — via CLI arg, config, or filesystem scan — must pass through a single normalization function before use. Apply this rule at the earliest possible point (argument parsing or the top of each command handler), never inside DB functions.

```go
// internal/pathutil/pathutil.go
func Normalize(p string) (string, error)
```

Steps inside `Normalize`:
1. Expand `~` to the real home directory using `os.UserHomeDir()`.
2. Call `filepath.Abs(p)` to make it absolute.
3. Call `filepath.Clean(p)` to remove trailing slashes, `..`, and `.` segments.
4. Return the cleaned path.

Do not call `os.Stat` inside `Normalize` — it is a pure string transform. Existence checks are the caller's responsibility.

### Error Handling Convention

- DB functions return `(T, error)` — they never print anything.
- Command handlers (in `cmd/`) catch errors and print them using the styled error renderer from `internal/ui/styles.go`.
- All fatal errors exit with code `1` via `os.Exit(1)` or `cmd.SilenceUsage = true` + returning an error from `RunE`.
- Use `fmt.Errorf("watch add: %w", err)` wrapping everywhere so error chains are informative.
- Never use `log.Fatal` or `panic` in production paths.

### Exit Codes

| Situation | Code |
|---|---|
| Success | 0 |
| User error (bad args, file not found) | 1 |
| Internal error (DB failure, I/O) | 1 |

### Cobra Pattern

Every subcommand uses `RunE` (not `Run`) so that errors propagate cleanly. Set `SilenceUsage: true` on the root command so cobra doesn't print the usage block on runtime errors — only on argument validation failures.

### SQLite WAL Mode

Enable WAL mode immediately after opening the database connection. This prevents write-lock contention if the user runs two scout invocations simultaneously and improves read concurrency.

```sql
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
```

Both pragmas must be re-applied each connection open, because they are connection-level settings, not persistent schema changes (WAL mode is persistent once set, but foreign keys are always per-connection).

### Binary Detection

To determine whether a file is binary and should receive a `NULL` preview: read the first 512 bytes and call `http.DetectContentType()` from the Go standard library. If the returned MIME type does not start with `text/`, treat the file as binary. This avoids importing a third-party magic-byte library.

### File Preview Size

Read at most 512 bytes from the file to perform binary detection, then store at most 200 bytes (characters) of valid UTF-8 text as the preview. Use `[]rune` conversion to count characters, not bytes, to avoid cutting a multi-byte character mid-sequence.

---

## Phase 0: Project Scaffold

**Goal:** A compilable `scout` binary that initializes `~/.scout/`, opens SQLite, runs schema migrations, and dispatches root-level arguments correctly. `scout --help` and all subcommand stubs exist.

### 0.1 Initialize Go module

```
go mod init github.com/<your-username>/scout-cli
```

The module path in `go.mod` must match the import prefix used in every `internal/` package. Choose this once and never change it.

### 0.2 `go.mod` — Full Dependency List

```
module github.com/youruser/scout-cli

go 1.22

require (
    github.com/BurntSushi/toml v1.3.2
    github.com/charmbracelet/bubbletea v0.26.4
    github.com/charmbracelet/lipgloss v0.11.0
    github.com/spf13/cobra v1.8.1
    modernc.org/sqlite v1.30.0
)
```

Run `go mod tidy` after adding these to pull in indirect dependencies. The `modernc.org/sqlite` driver is pure Go — no CGO, no system `libsqlite3` required. This is the key distribution advantage; do not substitute it with `mattn/go-sqlite3`.

### 0.3 `main.go`

Minimal entry point. All logic lives in `cmd/` and `internal/`.

```go
// main.go
package main

import (
    "github.com/youruser/scout-cli/cmd"
    "os"
)

func main() {
    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

`cmd.Execute()` is the exported function from `cmd/root.go`.

### 0.4 `internal/db/db.go` — Auto-Init and Schema Migration

This file owns the database lifecycle.

**Types and functions to implement:**

```go
package db

import (
    "database/sql"
    _ "modernc.org/sqlite"
)

// DB wraps *sql.DB and is passed to all repository functions.
type DB struct {
    conn *sql.DB
}

// Open opens (or creates) the scout SQLite database.
// It creates ~/.scout/ if missing, applies PRAGMA settings,
// and runs all schema migrations in order.
func Open() (*DB, error)

// Close closes the underlying connection.
func (d *DB) Close() error

// migrate runs all CREATE TABLE and CREATE VIRTUAL TABLE statements
// inside a single transaction. Uses IF NOT EXISTS so it is safe
// to call on every startup — it is idempotent.
func migrate(conn *sql.DB) error
```

**Implementation notes for `Open()`:**

1. Call `os.UserHomeDir()` to get `~`.
2. Build the path `filepath.Join(home, ".scout")`.
3. Call `os.MkdirAll(scoutDir, 0755)` — idempotent, safe to call every run.
4. Build the DB path `filepath.Join(scoutDir, "scout.db")`.
5. Call `sql.Open("sqlite", dbPath)` using the modernc driver name `"sqlite"` (not `"sqlite3"`).
6. Call `conn.Ping()` to verify the file is accessible.
7. Execute `PRAGMA journal_mode=WAL;` and `PRAGMA foreign_keys=ON;` as separate `Exec` calls.
8. Call `migrate(conn)`.
9. Return `&DB{conn: conn}`.

**Migration SQL — implement as a single string constant:**

```sql
CREATE TABLE IF NOT EXISTS workspaces (
  id       INTEGER PRIMARY KEY AUTOINCREMENT,
  path     TEXT NOT NULL UNIQUE,
  name     TEXT NOT NULL,
  added_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS files (
  id       INTEGER PRIMARY KEY AUTOINCREMENT,
  path     TEXT NOT NULL UNIQUE,
  name     TEXT NOT NULL,
  preview  TEXT,
  added_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tags (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS file_tags (
  file_id INTEGER REFERENCES files(id) ON DELETE CASCADE,
  tag_id  INTEGER REFERENCES tags(id)  ON DELETE CASCADE,
  PRIMARY KEY (file_id, tag_id)
);

CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(
  path, name, preview,
  content='files',
  content_rowid='id'
);
```

**FTS5 Trigger setup — CRITICAL GOTCHA:**

FTS5 content tables do not automatically stay in sync with the base table. You must create three triggers to maintain the FTS index. Add these to the migration:

```sql
-- Keep FTS index in sync with files table inserts
CREATE TRIGGER IF NOT EXISTS files_ai AFTER INSERT ON files BEGIN
  INSERT INTO files_fts(rowid, path, name, preview)
  VALUES (new.id, new.path, new.name, new.preview);
END;

-- Keep FTS index in sync with files table deletes
CREATE TRIGGER IF NOT EXISTS files_ad AFTER DELETE ON files BEGIN
  INSERT INTO files_fts(files_fts, rowid, path, name, preview)
  VALUES ('delete', old.id, old.path, old.name, old.preview);
END;

-- Keep FTS index in sync with files table updates
CREATE TRIGGER IF NOT EXISTS files_au AFTER UPDATE ON files BEGIN
  INSERT INTO files_fts(files_fts, rowid, path, name, preview)
  VALUES ('delete', old.id, old.path, old.name, old.preview);
  INSERT INTO files_fts(rowid, path, name, preview)
  VALUES (new.id, new.path, new.name, new.preview);
END;
```

Without these triggers, `INSERT INTO files` does not update `files_fts`, and searches return nothing. The triggers use FTS5's special `INSERT ... ('delete', ...)` command syntax to remove old entries — this is not standard SQL and is specific to FTS5 content tables.

Run the entire migration block (schema + triggers) wrapped in a transaction: `BEGIN IMMEDIATE` ... `COMMIT`. Use `IF NOT EXISTS` on every `CREATE` so the migration is fully idempotent.

### 0.5 `internal/pathutil/pathutil.go`

```go
package pathutil

func Normalize(p string) (string, error)
func IsDir(p string) (bool, error)  // calls os.Stat, returns false if not exists
```

### 0.6 `cmd/root.go` — Root Command and Argument Dispatch

```go
package cmd

import (
    "github.com/spf13/cobra"
    "github.com/youruser/scout-cli/internal/db"
)

// Execute is called from main.go
func Execute() error

// rootCmd is the base cobra command.
var rootCmd *cobra.Command
var database *db.DB

func init() // registers all subcommands
```

**Argument dispatch logic in `rootCmd.RunE`:**

```
args length == 0                          → print help, exit 0
args[0] starts with "@"                  → call tagShowAction(args[0])
args length == 1                         → call searchAction(args[0])
args length == 2 && args[0] starts "@"  → call scopedSearchAction(args[0], args[1])
otherwise                                → print help, exit 0
```

Set `Args: cobra.ArbitraryArgs` on the root command to accept 0–2 positional args without cobra rejecting them before `RunE` can inspect them.

Use `PersistentPreRunE` to open the database once and assign it to a package-level `database` variable. Use `PersistentPostRunE` to close it. This way every subcommand inherits DB access without repeating the open/close logic.

**Important:** cobra calls `PersistentPreRunE` on the most specific matching command. If a subcommand defines its own `PreRunE`, it will shadow the root's `PersistentPreRunE`. Use `PersistentPreRunE` exclusively — never `PreRunE` — for DB initialization.

### 0.7 `cmd/watch.go`, `cmd/tag.go`, `cmd/prune.go` — Stub Subcommands

Create each file with the command tree registered but `RunE` returning `nil` (no-op stubs). This proves the CLI tree compiles and `--help` output is correct before any logic is wired up.

```go
// cmd/watch.go
var watchCmd    = &cobra.Command{Use: "watch", Short: "Manage watched workspaces"}
var watchAddCmd = &cobra.Command{Use: "add <path>", Short: "Register a workspace",   Args: cobra.ExactArgs(1), RunE: runWatchAdd}
var watchRmCmd  = &cobra.Command{Use: "remove <path>", Short: "Unregister a workspace", Args: cobra.ExactArgs(1), RunE: runWatchRemove}
var watchLsCmd  = &cobra.Command{Use: "list", Short: "List workspaces",               RunE: runWatchList}
var watchSyncCmd= &cobra.Command{Use: "sync", Short: "Rescan workspaces",             RunE: runWatchSync}
```

```go
// cmd/tag.go
var tagCmd       = &cobra.Command{Use: "tag", Short: "Manage tags"}
var tagAddCmd    = &cobra.Command{Use: "add <@tag> <path>",    Args: cobra.ExactArgs(2), RunE: runTagAdd}
var tagRemoveCmd = &cobra.Command{Use: "remove <@tag> <path>", Args: cobra.ExactArgs(2), RunE: runTagRemove}
var tagListCmd   = &cobra.Command{Use: "list",                                           RunE: runTagList}
var tagShowCmd   = &cobra.Command{Use: "show <@tag>",          Args: cobra.ExactArgs(1), RunE: runTagShow}
```

```go
// cmd/prune.go
var pruneCmd = &cobra.Command{Use: "prune", Short: "Remove stale DB entries", RunE: runPrune}
```

### Phase 0 Checklist

- [ ] `go.mod` with all 5 direct dependencies
- [ ] `main.go` delegates to `cmd.Execute()`
- [ ] `internal/db/db.go` opens DB, creates `~/.scout/`, runs migration with all 5 tables + 3 triggers
- [ ] `internal/pathutil/pathutil.go` with `Normalize()`
- [ ] `cmd/root.go` with argument dispatch skeleton and DB lifecycle in `PersistentPreRunE` / `PersistentPostRunE`
- [ ] All subcommand stubs registered — `scout --help` shows the full tree

---

## Phase 1: Watch System

**Goal:** `scout watch add`, `remove`, `list`, and `sync` are fully functional. Files are scanned and stored in SQLite with previews. Implicit workspace tags are created.

### 1.1 `internal/db/files.go` — File CRUD

```go
package db

type File struct {
    ID      int64
    Path    string
    Name    string
    Preview *string  // nil for binary files
    AddedAt time.Time
}

// UpsertFile inserts a file or updates name/preview if path already exists.
// Uses ON CONFLICT DO UPDATE to avoid breaking FK references in file_tags.
func (d *DB) UpsertFile(path, name string, preview *string) (int64, error)

// DeleteFile removes a file by its exact path.
func (d *DB) DeleteFile(path string) error

// DeleteFileByID removes a file by its database ID.
func (d *DB) DeleteFileByID(id int64) error

// DeleteFilesUnderPath removes all files whose path starts with the given prefix.
func (d *DB) DeleteFilesUnderPath(prefix string) (int64, error)

// GetAllFiles returns all tracked files.
func (d *DB) GetAllFiles() ([]File, error)

// GetFilesUnderPath returns all files under a given path prefix.
func (d *DB) GetFilesUnderPath(prefix string) ([]File, error)

// GetFileByPath returns a single file by its exact path.
func (d *DB) GetFileByPath(path string) (File, error)
```

**Upsert pattern — do not use `INSERT OR REPLACE`:**

`INSERT OR REPLACE` deletes the old row and inserts a new one (changing `id`), which silently breaks FK references in `file_tags`. Use this instead:

```sql
INSERT INTO files (path, name, preview)
VALUES (?, ?, ?)
ON CONFLICT(path) DO UPDATE SET name=excluded.name, preview=excluded.preview
```

**`DeleteFilesUnderPath` — path separator gotcha:**

Always normalize paths to forward slashes before storage using `filepath.ToSlash`. Query with:

```sql
WHERE path = ? OR path LIKE ?
```

Bind params: exact path and `prefix + "/%"`. The `/` must be a literal forward slash, not `filepath.Separator`, because LIKE is applied to the stored (forward-slash) form.

### 1.2 `internal/db/workspaces.go`

```go
package db

type Workspace struct {
    ID      int64
    Path    string
    Name    string
    AddedAt time.Time
}

type WorkspaceWithCount struct {
    Workspace
    FileCount int
}

func (d *DB) AddWorkspace(path, name string) error
func (d *DB) RemoveWorkspace(path string) error
func (d *DB) GetAllWorkspaces() ([]Workspace, error)
func (d *DB) ListWorkspaces() ([]WorkspaceWithCount, error)
```

For `ListWorkspaces`, count files whose paths fall under the workspace:

```sql
SELECT w.id, w.path, w.name, w.added_at,
       COUNT(f.id) as file_count
FROM workspaces w
LEFT JOIN files f ON f.path = w.path OR f.path LIKE w.path || '/%'
GROUP BY w.id
ORDER BY w.name
```

### 1.3 `internal/db/tags.go` — Tag CRUD

```go
package db

type Tag struct {
    ID   int64
    Name string
}

type TagWithMeta struct {
    Tag
    FileCount   int
    IsWorkspace bool
}

// EnsureTag gets or creates a tag by name. Returns the tag ID.
func (d *DB) EnsureTag(name string) (int64, error)

// TagFile links a file to a tag. Idempotent (ON CONFLICT DO NOTHING).
func (d *DB) TagFile(fileID, tagID int64) error

// TagFilesUnderPath adds a tag to all files under a path prefix.
func (d *DB) TagFilesUnderPath(prefix string, tagID int64) error

// UntagFile removes a specific tag from a specific file.
func (d *DB) UntagFile(fileID, tagID int64) error

// UntagFilesUnderPath removes a tag from all files under a prefix.
func (d *DB) UntagFilesUnderPath(prefix string, tagID int64) error

// GetTagByName returns a tag by name, or an error if not found.
func (d *DB) GetTagByName(name string) (Tag, error)

// ListTags returns all tags with file counts and workspace flag.
func (d *DB) ListTags() ([]TagWithMeta, error)

// GetFilesByTag returns all files associated with a tag name.
func (d *DB) GetFilesByTag(tagName string) ([]File, error)
```

**Tag name convention:** store tag names WITH the `@` prefix (e.g. `@myproject`). This matches how users type them and avoids stripping/adding logic at every call site.

**`ListTags` SQL:**

```sql
SELECT t.id, t.name,
       COUNT(ft.file_id) as file_count,
       CASE WHEN w.id IS NOT NULL THEN 1 ELSE 0 END as is_workspace
FROM tags t
LEFT JOIN file_tags ft ON ft.tag_id = t.id
LEFT JOIN workspaces w ON w.name = t.name
GROUP BY t.id
ORDER BY t.name
```

Note: workspace tags are matched by comparing `workspaces.name` to `tags.name`. Since workspace names are stored without `@` (derived from `filepath.Base`) and tag names are stored with `@`, the JOIN condition should account for this: `'@' || w.name = t.name`.

### 1.4 `internal/scanner/scanner.go` — File System Scanner

```go
package scanner

type Result struct {
    Path    string
    Name    string
    Preview *string  // nil for binary or unreadable files
}

// ScanDir walks a directory recursively and returns a Result for each file.
// Skips hidden files/directories (names starting with ".").
// Skips symlinks to avoid loops.
func ScanDir(root string) ([]Result, error)

// ReadPreview reads the first 200 characters of a text file.
// Returns nil if the file is binary or unreadable.
func ReadPreview(path string) *string
```

**`ScanDir` implementation — use `filepath.WalkDir`:**

`filepath.WalkDir` avoids an extra `os.Stat` per entry compared to `filepath.Walk`.

Skip criteria inside the walk function:
- `d.Name()` starts with `"."` → skip, and if directory return `filepath.SkipDir`
- `d.Type()&fs.ModeSymlink != 0` → skip to avoid cycles
- `d.IsDir()` → continue descending, don't emit a result

**`ReadPreview` implementation:**

1. Open file, defer close.
2. Read up to 512 bytes: `n, _ = f.Read(buf[:512])`.
3. Pass `buf[:n]` to `http.DetectContentType()`. If result doesn't start with `"text/"`, return `nil`.
4. Convert to runes: `runes := []rune(string(buf[:n]))`.
5. Truncate to 200 runes if longer.
6. Return pointer to `string(runes[:min(200, len(runes))])`.

### 1.5 Wire Up `cmd/watch.go`

**`runWatchAdd`:**

```
1. Normalize args[0] → absPath
2. os.Stat(absPath) — error if not a directory
3. name = filepath.Base(absPath)
4. db.AddWorkspace(absPath, name) — error if already registered
5. scanner.ScanDir(absPath) → []Result
6. For each result: db.UpsertFile(r.Path, r.Name, r.Preview)
7. tagName = "@" + name
8. db.EnsureTag(tagName) → tagID
9. db.TagFilesUnderPath(absPath, tagID)
10. Print: "Registered workspace @<name> (<count> files)"
```

**`runWatchRemove`:**

```
1. Normalize args[0] → absPath
2. db.RemoveWorkspace(absPath) — error if not found
3. name = filepath.Base(absPath); tagName = "@" + name
4. db.GetTagByName(tagName) → tag
5. db.UntagFilesUnderPath(absPath, tag.ID)
6. db.DeleteFilesUnderPath(absPath)
```

**`runWatchSync`:**

```
1. db.GetAllWorkspaces() → []Workspace
2. For each workspace:
   a. scanner.ScanDir(workspace.Path) → currentResults
   b. db.GetFilesUnderPath(workspace.Path) → existingFiles
   c. Build currentPaths = map[string]Result
   d. Build existingPaths = map[string]File
   e. NEW = currentPaths \ existingPaths → UpsertFile + TagFile
   f. REMOVED = existingPaths \ currentPaths → DeleteFile
   g. Print: "Synced @<name>: +<added> added, <removed> removed"
```

### Phase 1 Checklist

- [ ] `internal/db/workspaces.go` — workspace CRUD
- [ ] `internal/db/files.go` — file CRUD with `ON CONFLICT DO UPDATE` upsert
- [ ] `internal/db/tags.go` — `EnsureTag`, `TagFilesUnderPath`, `ListTags`
- [ ] `internal/scanner/scanner.go` — `ScanDir` and `ReadPreview`
- [ ] `cmd/watch.go` — all four handlers wired up
- [ ] `scout watch add ~/some/dir` works end-to-end

---

## Phase 2: Keyword Search (FTS5)

**Goal:** `scout "query"` and `scout @tag "query"` return ranked results from SQLite FTS5.

### 2.1 `internal/db/search.go` — FTS5 Query Logic

```go
package db

type SearchResult struct {
    File
    Rank float64
}

// Search runs an FTS5 query over all tracked files, ranked by BM25.
func (d *DB) Search(query string) ([]SearchResult, error)

// SearchByTag runs an FTS5 query scoped to files with a given tag.
func (d *DB) SearchByTag(tagName, query string) ([]SearchResult, error)

// buildFTSQuery transforms raw user input into a valid FTS5 query string.
func buildFTSQuery(raw string) string
```

**`buildFTSQuery` rules:**

1. Split on whitespace into terms.
2. Strip any trailing `*` from each term.
3. If a term contains FTS5 special chars (`"`, `(`, `)`, `-`, `+`, `^`) or is an FTS5 keyword (`OR`, `AND`, `NOT`), wrap in double-quotes: `"term"`.
4. Append `*` to each term for prefix matching.
5. Join terms with a space (implicit AND).

Example: `"postgres config"` → `postgres* config*`

**Search SQL:**

```sql
SELECT f.id, f.path, f.name, f.preview, f.added_at,
       bm25(files_fts) as rank
FROM files_fts
JOIN files f ON f.id = files_fts.rowid
WHERE files_fts MATCH ?
ORDER BY rank        -- bm25() returns negative values; ORDER BY rank = most relevant first
LIMIT 200
```

**Important:** `bm25()` in SQLite FTS5 returns negative values. More negative = more relevant. `ORDER BY rank` (ASC, the default) puts the most relevant results first. Add a code comment documenting this to prevent future confusion.

**Scoped search SQL:**

```sql
SELECT f.id, f.path, f.name, f.preview, f.added_at,
       bm25(files_fts) as rank
FROM files_fts
JOIN files f ON f.id = files_fts.rowid
JOIN file_tags ft ON ft.file_id = f.id
JOIN tags t ON t.id = ft.tag_id
WHERE files_fts MATCH ?
  AND t.name = ?
ORDER BY rank
LIMIT 200
```

### 2.2 Wire Up Root Command Search Actions

In `cmd/root.go`:

```go
func searchAction(query string) error {
    results, err := database.Search(query)
    // for each result: check os.Stat(r.Path)
    //   → if missing: db.DeleteFile(r.Path) silently, skip
    //   → if found: add to display list
    // print results or "No files found matching ..."
}

func scopedSearchAction(tag, query string) error {
    results, err := database.SearchByTag(tag, query)
    // same stale-path inline prune as above
}
```

The stale-path check runs inline during result rendering — no background goroutine, no separate pass.

### Phase 2 Checklist

- [ ] `internal/db/search.go` — `Search`, `SearchByTag`, `buildFTSQuery`
- [ ] `buildFTSQuery` handles multi-word queries, prefix matching, special char escaping
- [ ] `searchAction` and `scopedSearchAction` wired in `cmd/root.go`
- [ ] FTS5 triggers verified: insert a file, search for its name, result appears
- [ ] Stale file auto-prune works during search

---

## Phase 3: Tag System

**Goal:** `scout tag add/remove/list/show` and the `scout @tagname` shorthand are fully functional.

### 3.1 Wire Up `cmd/tag.go`

**`runTagAdd(cmd, args)`:**

```
1. Validate args[0] starts with "@"
2. tagName = args[0]
3. Normalize args[1] → absPath
4. Warn if absPath does not exist, but continue
5. db.EnsureTag(tagName) → tagID
6. If absPath is a directory:
     db.GetFilesUnderPath(absPath) → files
     for each file: db.TagFile(file.ID, tagID)
   Else:
     db.GetFileByPath(absPath) → file
     db.TagFile(file.ID, tagID)
7. Print count of tagged files
```

**`runTagRemove`:** mirror of `runTagAdd` using `UntagFile` / `UntagFilesUnderPath`.

**`runTagList`:**

```
db.ListTags() → []TagWithMeta
Print formatted table: name, count, "(workspace)" badge if IsWorkspace
```

**`runTagShow`:**

```
tagName = args[0] (validate starts with "@")
db.GetFilesByTag(tagName) → []File
Print each file path
```

**`tagShowAction` in `cmd/root.go`:**

Delegates directly to the same logic as `runTagShow`, accepting the tag string directly instead of cobra args.

### Phase 3 Checklist

- [ ] `runTagAdd`, `runTagRemove`, `runTagList`, `runTagShow` implemented
- [ ] `tagShowAction` wired in root dispatch
- [ ] `scout @myproject` correctly lists files
- [ ] `scout @myproject "query"` correctly scopes search

---

## Phase 4: Prune

**Goal:** `scout prune` removes stale DB entries for files that no longer exist on disk.

### 4.1 `cmd/prune.go` — Implement `runPrune`

```go
func runPrune(cmd *cobra.Command, args []string) error {
    files, err := database.GetAllFiles()
    // for each file:
    //   os.Stat(file.Path)
    //   if error (not found): database.DeleteFileByID(file.ID), append to pruned list
    // print summary:
    //   if len(pruned) == 0: "No stale entries found"
    //   else: "Pruned <n> stale entries:" + each path
}
```

**Note on transactions:** deletions in prune are not wrapped in a single transaction so that partial results are visible on interrupt. If atomicity is preferred, wrap in `BEGIN`/`COMMIT` but buffer all output until commit.

### Phase 4 Checklist

- [ ] `runPrune` walks all files and removes stale entries
- [ ] `scout prune` on a clean DB prints "No stale entries found"
- [ ] After deleting files on disk, `scout prune` removes them from DB

---

## Phase 5: UI Polish

**Goal:** All output uses lipgloss styling. Errors are red on stderr. Tags are green. Paths are cyan.

### 5.1 `internal/ui/styles.go` — Style Definitions

```go
package ui

import "github.com/charmbracelet/lipgloss"

var (
    StyleCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
    StyleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
    StyleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
    StyleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
    StyleGray   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
    StyleBold   = lipgloss.NewStyle().Bold(true)

    StyleTag    = StyleGreen.Copy().Bold(true)
    StylePath   = StyleCyan.Copy()
    StyleError  = StyleRed.Copy()
    StyleMuted  = StyleGray.Copy()
    StyleHeader = StyleBold.Copy()
)
```

Using ANSI 256-color numbers (`"14"`, `"10"`, etc.) instead of hex ensures compatibility with terminals that don't support truecolor.

**Render helpers:**

```go
func Error(msg string)              // prints to stderr in red
func PrintHeader(title string)      // bold title + divider line
func FormatTag(name string) string  // green bold @tagname
func FormatPath(path string) string // cyan, ~ for home dir
```

**`FormatPath`:** replace the home dir prefix with `~` for display only. The stored path remains absolute.

### 5.2 `internal/ui/results.go` — Bubbletea Model

```go
package ui

import tea "github.com/charmbracelet/bubbletea"

type ResultsModel struct {
    Title  string
    Items  []string
    Footer string
}

func (m ResultsModel) Init() tea.Cmd                             { return tea.Quit }
func (m ResultsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)  { return m, tea.Quit }
func (m ResultsModel) View() string                             // header + divider + items + footer
```

The model is static (no keyboard interaction). Bubbletea is used here for its `View()` rendering pipeline and future extensibility (arrow-key navigation could be added later without restructuring the output layer).

**Usage pattern:**

```go
m := ui.ResultsModel{Title: `Results for "` + query + `"`, Items: paths, Footer: fmt.Sprintf("%d files", len(paths))}
p := tea.NewProgram(m)
p.Run()
```

**Gotcha:** bubbletea takes over stdout. Never mix `fmt.Println` and `tea.NewProgram` in the same output path. All output for result lists must go through `View()`.

For simpler output (tag list, watch list) use lipgloss directly with `fmt.Println` — no bubbletea program needed.

### 5.3 Apply Styles to All Commands

| Command | Styling |
|---|---|
| `watch list` | Tags green, paths cyan, counts gray |
| `watch add` | Success message cyan, count gray |
| `watch sync` | Added count green, removed count yellow |
| `tag list` | Tags green, counts gray, `(workspace)` badge muted |
| `tag show` | ResultsModel with tag as title |
| `search` | ResultsModel with query as title |
| errors | `ui.Error(msg)` → red on stderr |

### Phase 5 Checklist

- [ ] `internal/ui/styles.go` with full palette and helpers
- [ ] `internal/ui/results.go` with `ResultsModel`
- [ ] All command handlers use styled output
- [ ] Errors go to stderr in red
- [ ] `FormatPath` replaces home dir with `~`

---

## Phase 6: Distribution

**Goal:** GitHub Actions produces release binaries for 5 platforms. Homebrew tap formula is ready.

### 6.1 `.github/workflows/release.yml`

Trigger: `push` to a tag matching `v*`.

Steps:
1. `actions/checkout`
2. `actions/setup-go` with `go-version: "1.22"`
3. Build matrix for 5 targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
4. `GOOS`/`GOARCH` env vars + `go build -ldflags "-s -w -X main.version=${{ github.ref_name }}" -o scout-$GOOS-$GOARCH ./main.go` (append `.exe` for windows)
5. Compute SHA256 checksums
6. `softprops/action-gh-release` to upload artifacts

**Build flags:**
- `-s -w` strips debug info and DWARF, reducing binary size significantly
- `-X main.version=...` injects the Git tag as the version string

### 6.2 Homebrew Tap Formula

Create a separate repo named `homebrew-tap`. Formula at `Formula/scout.rb`:

```ruby
class Scout < Formula
  desc "CLI tool for finding files using workspaces and full-text search"
  homepage "https://github.com/youruser/scout-cli"
  version "0.1.0"

  on_macos do
    on_arm   { url "...darwin-arm64"; sha256 "..." }
    on_intel { url "...darwin-amd64"; sha256 "..." }
  end

  on_linux do
    on_arm   { url "...linux-arm64"; sha256 "..." }
    on_intel { url "...linux-amd64"; sha256 "..." }
  end

  def install
    bin.install Dir["scout-*"].first => "scout"
  end

  test do
    system "#{bin}/scout", "--version"
  end
end
```

Install via: `brew install scout-cli/tap/scout`

### 6.3 `scout --version`

In `cmd/root.go`, declare a package-level `version` variable and set it via ldflags:

```go
var version = "dev"  // overridden by -X main.version=... at build time

// In init():
rootCmd.Version = version
```

Cobra automatically adds `--version` / `-v` when `Version` is non-empty.

### Phase 6 Checklist

- [ ] `.github/workflows/release.yml` builds 5 targets on tag push
- [ ] Binaries use `-ldflags "-s -w"` for size reduction
- [ ] SHA256 checksums computed and uploaded to GitHub Releases
- [ ] `homebrew-tap` repo with `Formula/scout.rb`
- [ ] `scout --version` prints the injected version string

---

## Dependency Reference

```
module github.com/youruser/scout-cli

go 1.22

require (
    github.com/BurntSushi/toml v1.3.2
    github.com/charmbracelet/bubbletea v0.26.4
    github.com/charmbracelet/lipgloss v0.11.0
    github.com/spf13/cobra v1.8.1
    modernc.org/sqlite v1.30.0
)
```

| Package | Purpose |
|---|---|
| `modernc.org/sqlite` | Pure-Go SQLite driver. No CGO, no system library. Required for cross-compilation to all 5 platforms. |
| `github.com/spf13/cobra` | Command tree, arg validation, `--help` generation, flag parsing. |
| `github.com/charmbracelet/bubbletea` | TUI framework for result list rendering. |
| `github.com/charmbracelet/lipgloss` | Terminal styling — colors, bold, padding. |
| `github.com/BurntSushi/toml` | TOML config parsing for `~/.scout/config.toml`. |

---

## Appendix A: File Layout Reference

```
scout-cli/
  main.go
  go.mod
  go.sum
  cmd/
    root.go         # root command, argument dispatch, DB lifecycle
    watch.go        # watch subcommands
    tag.go          # tag subcommands
    prune.go        # prune command
  internal/
    db/
      db.go         # Open(), migrate(), DB type, FTS5 triggers
      workspaces.go # workspace CRUD  ← not in original spec; justified by separation of concerns
      files.go      # file CRUD
      tags.go       # tag CRUD
      search.go     # FTS5 search
    pathutil/
      pathutil.go   # Normalize(), IsDir()  ← new package; isolates path logic for testing
    scanner/
      scanner.go    # ScanDir(), ReadPreview()  ← new package; isolates FS I/O for testing
    ui/
      styles.go     # lipgloss style vars + render helpers
      results.go    # bubbletea ResultsModel
  specifications/
    specifications.md
    plan.md
  .github/
    workflows/
      release.yml
```

---

## Appendix B: Tricky Parts Summary

| Gotcha | Location | Resolution |
|---|---|---|
| FTS5 content table does not auto-sync | `db.go` migration | Three `AFTER INSERT/DELETE/UPDATE` triggers on `files` |
| `bm25()` returns negative values | `search.go` | `ORDER BY rank` ASC = most relevant first; add a comment |
| `INSERT OR REPLACE` breaks foreign keys | `files.go` | Use `INSERT ... ON CONFLICT(path) DO UPDATE SET ...` |
| Bubbletea takes over stdout | `results.go` | Never mix `fmt.Println` with `tea.NewProgram` in same output path |
| modernc driver name is `"sqlite"` not `"sqlite3"` | `db.go` | `sql.Open("sqlite", ...)` |
| `foreign_keys` pragma is per-connection | `db.go` | Re-execute `PRAGMA foreign_keys=ON` in every `Open()` |
| Path prefix LIKE must use `/` not `filepath.Separator` | `files.go` | Normalize all paths with `filepath.ToSlash` before storage |
| Hidden dirs (`.cache`, `node_modules` via `.`) scanned | `scanner.go` | Skip entries whose `d.Name()` starts with `"."` |
| Cobra `PersistentPreRunE` shadowed by child `PreRunE` | `root.go` | Never define `PreRunE` on child commands — only `RunE` |
| Workspace name vs tag name mismatch (`myproject` vs `@myproject`) | `tags.go` | Workspace names stored without `@`; tag names stored with `@`; JOIN uses `'@' \|\| w.name = t.name` |

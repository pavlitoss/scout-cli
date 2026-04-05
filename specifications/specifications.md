# Scout — Project Requirements

## Overview

`scout` is a lightweight, fast CLI tool for finding files using workspaces and full-text search. It works as a simple workspace-based file manager with instant keyword search powered by SQLite FTS5. It is built in Go, ships as a single binary, and stores everything in a single SQLite file.

---

## Core Philosophy

- **Zero dependencies for the user** — single binary, nothing to install
- **Offline first** — all core features work without internet or API keys
- **Lightweight** — no daemon, no background process, no indexing step
- **Simple storage** — single `~/.scout/scout.db` SQLite file
- **Zero setup** — auto-initializes on first use, no manual init step required

---

## Tech Stack

| Component | Choice | Notes |
|---|---|---|
| Language | Go | Single binary, fast startup, easy distribution |
| CLI framework | `cobra` | Industry standard for Go CLIs |
| Terminal UI | `bubbletea` + `lipgloss` | Colorful, interactive terminal visuals |
| Database | SQLite via `modernc.org/sqlite` | Pure Go, no CGO required |
| Full-text search | SQLite FTS5 | Built-in, zero extra deps |
| Config | TOML via `github.com/BurntSushi/toml` | Simple key-value config file |

---

## Installation

```bash
# Homebrew (via personal tap)
brew install scout-cli/tap/scout

# Direct install script
curl -fsSL https://scout.sh/install | sh

# Manual — download binary from GitHub releases and add to PATH
```

---

## File Structure

```
~/.scout/
  scout.db        # SQLite database — files, tags, FTS index
  config.toml     # API key, preferences
```

```
# Project layout
scout/
  cmd/
    root.go       # root cobra command + argument dispatch logic
    watch.go      # watch subcommands
    tag.go        # tag subcommands
    config.go     # config subcommands
    prune.go      # prune command
  internal/
    db/
      db.go       # SQLite setup, migrations, auto-init
      files.go    # file CRUD
      tags.go     # tag CRUD
      search.go   # FTS5 search logic
    ui/
      styles.go   # lipgloss color/style definitions
      results.go  # bubbletea results list component
      spinner.go  # loading spinner for LLM calls
  main.go
  go.mod
  go.sum
```

---

## Auto-Initialization

`~/.scout/` and the SQLite database are created automatically on the first invocation of any `scout` command. There is no `scout init` command. The user experience is: install → use.

---

## Database Schema

```sql
-- Watched directories (workspaces)
CREATE TABLE workspaces (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  path        TEXT NOT NULL UNIQUE,
  name        TEXT NOT NULL,           -- derived from directory name, used as implicit tag
  added_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tracked files
CREATE TABLE files (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  path         TEXT NOT NULL UNIQUE,
  name         TEXT NOT NULL,           -- filename only
  preview      TEXT,                    -- first 200 chars of file content, nullable
  added_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tags (optional, user-defined labels on top of workspaces)
CREATE TABLE tags (
  id    INTEGER PRIMARY KEY AUTOINCREMENT,
  name  TEXT NOT NULL UNIQUE
);

-- Many-to-many: files <-> tags
CREATE TABLE file_tags (
  file_id  INTEGER REFERENCES files(id) ON DELETE CASCADE,
  tag_id   INTEGER REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (file_id, tag_id)
);

-- FTS5 virtual table for full-text search over filenames, paths, and previews
CREATE VIRTUAL TABLE files_fts USING fts5(
  path,
  name,
  preview,
  content='files',
  content_rowid='id'
);
```

---

## Exclusions

### Default exclusions

When scanning a directory, scout skips the following by default:

**Directory names (anywhere in the tree):**
- `node_modules`
- `.git`
- `.hg`
- `.svn`
- `dist`, `build`, `out`, `target`
- `.cache`
- `__pycache__`
- `.venv`, `venv`, `env`

**Paths (absolute prefix match):**
- `/proc`
- `/sys`
- `/dev`
- `/run`
- `/tmp`

**Files:**
- Binary files — detected by scanning the first 512 bytes for null bytes; if any are found the file is skipped and no preview is stored
- Files larger than 1 MB are indexed (path + name) but their preview is skipped

### `.scoutignore`

A `.scoutignore` file placed in a watched directory lets the user add custom exclusion patterns. Patterns follow `.gitignore` syntax (glob-based, one pattern per line, `#` for comments).

```
# .scoutignore example
*.log
*.lock
secrets/
tmp/
```

Scout checks for `.scoutignore` only at the root of each watched workspace, not recursively. Patterns apply to all paths within that workspace.

### User-level ignore config

Global exclusions can be defined in `~/.scout/config.toml`:

```toml
[ignore]
dirs = ["vendor", "coverage", ".terraform"]
extensions = [".pyc", ".class", ".o", ".a"]
```

These are merged with the built-in defaults and any `.scoutignore` rules.

---

## Commands

### Root Command — Argument Dispatch

The root `scout` command dispatches based on argument shape. The rule is explicit and enforced by cobra's `Args` validator:

- Argument starts with `@` → tag operation (list files under that tag)
- Argument is a quoted string or contains spaces → full-text / LLM search
- Two arguments where the first starts with `@` and the second is a string → scoped search
- Anything else → show help

---

### `scout watch add <path>`
Registers a directory as a workspace. On registration, scout lazily scans all files inside it and stores their paths and names. The directory name becomes an implicit tag (e.g. `~/code/myproject` → `@myproject`). New files added to the directory are picked up on the next `scout` invocation that touches the workspace — no daemon required.

```bash
scout watch add ~/code/myproject
# Registered workspace @myproject (143 files)

scout watch add ~/dotfiles
# Registered workspace @dotfiles (31 files)
```

---

### `scout watch remove <path>`
Removes a workspace and untracks all its files from the DB. Does not delete the actual files.

```bash
scout watch remove ~/code/myproject
```

---

### `scout watch list`
Lists all registered workspaces.

```bash
scout watch list

# @myproject    ~/code/myproject    143 files
# @dotfiles     ~/dotfiles           31 files
```

---

### `scout watch sync`
Rescans all registered workspaces, adding newly created files and removing entries for deleted or moved files. Run this manually after large directory changes.

```bash
scout watch sync
# Synced @myproject: +12 added, 3 removed
```

---

### `scout tag add <@tagname> <path>`
Adds a user-defined tag to a file or directory (recursively for directories). Tags are optional labels on top of workspace-based tracking. The file must already be tracked (i.e. inside a watched workspace).

```bash
scout tag add @backend ~/code/myproject/src/
scout tag add @config ~/code/myproject/config/app.toml
```

---

### `scout tag remove <@tagname> <path>`
Removes a tag from a file or directory (recursively for directories).

```bash
scout tag remove @backend ~/code/myproject/src/
```

---

### `scout tag list`
Lists all existing tags with a count of how many files each has. Includes implicit workspace tags.

```bash
scout tag list

# Example output:
# @myproject    143 files   (workspace)
# @dotfiles      31 files   (workspace)
# @backend       22 files
# @config         5 files
```

---

### `scout tag show <@tagname>`
Lists all files associated with a given tag.

```bash
scout tag show @backend
```

---

### `scout <@tagname>`
Shorthand to list all files under a tag. Same as `scout tag show`.

```bash
scout @myproject
```

---

### `scout "<query>"`
Searches across all tracked files using SQLite FTS5 over filenames, paths, and file previews (first 200 chars of content).

```bash
scout "postgres config"
scout "backup database"
```

---

### `scout <@tagname> "<query>"`
Scopes a search to only files under a specific tag.

```bash
scout @myproject "postgres config"
scout @dotfiles "shell aliases"
```

---

### `scout prune`
Scans all tracked file paths. Removes DB entries for files that no longer exist (deleted, moved, renamed). Prints a summary of what was removed. Also runs automatically in the background after any search that returns a path that no longer exists on disk.

```bash
scout prune

# Pruned 4 stale entries:
#   ~/code/myproject/old-schema.sql  (not found)
#   ~/code/myproject/tmp/debug.log   (not found)
#   ...
```

---


## Search Behavior

### What is FTS5?

FTS5 (Full-Text Search 5) is a built-in SQLite extension that enables fast, ranked keyword search over text columns. Unlike a plain `LIKE` query that scans every row, FTS5 maintains an inverted index — a map from every word to the rows that contain it — so queries are fast even over thousands of files.

Key properties relevant to scout:
- **Tokenization** — input is split into terms; punctuation and case are normalized automatically
- **Prefix matching** — a query like `back*` matches `backup`, `background`, etc.
- **Ranking** — results are ranked by BM25, a standard relevance algorithm that weights term frequency and document length
- **Multi-column search** — a single query searches across `path`, `name`, and `preview` simultaneously, with configurable column weights

FTS5 is not fuzzy search (typos won't match), but prefix matching plus BM25 ranking gives results that feel natural for filename and path search.

### Search behavior
- Searches filenames, paths, and file previews (first 200 chars of content) simultaneously
- Results ranked by BM25 relevance — more specific matches appear first
- Prefix matching supported: `conf` matches `config.toml`, `conf.yaml`, etc.
- Instant — all local, no network call

### File previews
When a file is tracked (via `scout watch add` or `scout watch sync`), scout reads and stores the first 200 characters of its content in the `files.preview` column. This gives FTS5 enough signal to understand what a file contains without indexing full content. Binary files and unreadable files store a `NULL` preview.

---

## UI & Visual Design

Use `lipgloss` for styling and `bubbletea` for interactive components. The tool should feel polished and modern in the terminal.

### Color scheme
Define a consistent palette in `internal/ui/styles.go`:

```
Primary accent:   Cyan   (#00FFFF or terminal cyan)
Success / tag:    Green
Warning:          Yellow
Error:            Red
Muted / dim:      Gray
```

### Output formatting

**Tag list (`scout tag list`):**
```
  Tags
  ────────────────────
  @myproject    143 files   (workspace)
  @dotfiles      31 files   (workspace)
  @backend       22 files
```

**File results (`scout @myproject` or `scout "query"`):**
```
  Results for @myproject
  ────────────────────────────────────────
  ~/code/myproject/src/db.yaml
  ~/code/myproject/config/app.toml
  ~/code/myproject/scripts/backup.sh
  ...
  143 files
```

**Search results (`scout "query"`):**
```
  Results for "postgres config"
  ────────────────────────────────────────
  ~/code/myproject/config/database.yaml
  ~/code/myproject/.env
  ~/code/myproject/docker-compose.yml
```

### Error states
- No results: `No files found matching "<query>"`

---

## Error Handling

- If `~/.scout/scout.db` does not exist, create it automatically — never prompt the user to run init
- If a file path does not exist when tagging, show a warning but do not crash
- If a search result path no longer exists on disk, remove it from the DB silently and exclude it from results
- All errors should be printed in red using lipgloss styling
- Exit code `0` for success, `1` for errors

---

## Build & Distribution

```bash
# Build
go build -o scout ./main.go

# Cross-compile for common platforms
GOOS=linux GOARCH=amd64 go build -o scout-linux-amd64 ./main.go
GOOS=darwin GOARCH=arm64 go build -o scout-darwin-arm64 ./main.go
GOOS=windows GOARCH=amd64 go build -o scout-windows-amd64.exe ./main.go
```

Releases should be published to GitHub Releases with prebuilt binaries for:
- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`

Homebrew distribution is via a personal tap (`scout-cli/tap/scout`), not Homebrew core.

### Install Script (self-hosted)

An install script is hosted on the personal home server and serves as the primary one-liner install method:

```bash
curl -fsSL https://<homeserver>/scout/install.sh | sh
```

The script detects OS and architecture, downloads the appropriate binary from GitHub Releases, and installs it to `/usr/local/bin/scout`:

```sh
#!/bin/sh
set -e

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

REPO="pavlitoss/scout-cli"
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)

BINARY="scout-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"

echo "Installing scout ${VERSION} (${OS}/${ARCH})..."
curl -fsSL "$URL" -o /usr/local/bin/scout
chmod +x /usr/local/bin/scout
echo "Done. Run 'scout --help' to get started."
```

The home server only serves this script — binaries are downloaded directly from GitHub Releases. If full independence from GitHub is desired in the future, binaries can be mirrored on the home server and the `URL` updated accordingly.

---

## Build Order (Recommended)

Build in this order so you have something useful at each step:

1. **Project scaffold** — `cobra` CLI, SQLite auto-init, argument dispatch logic
2. **Watch system** — `watch add`, `watch remove`, `watch list`, `watch sync`, lazy file scanning, implicit workspace tags
3. **Keyword search** — SQLite FTS5 over paths, names, and previews; `scout "query"`, scoped `scout @tag "query"`
4. **Tag system** — `tag add`, `tag remove`, `tag list`, `tag show`, `scout @tag`
5. **Prune** — `scout prune`, automatic stale-path removal on search miss
6. **UI polish** — `lipgloss` styles, formatted output, error states
7. **Distribution** — GitHub Actions release pipeline, Homebrew tap formula

---

## Out of Scope (for now)

- Real-time directory watching (inotify/FSEvents daemon)
- GUI or web interface
- Sync across machines
- Support for multiple config profiles

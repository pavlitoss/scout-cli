# scout

Tag and search your files, fast.

Scout is a lightweight CLI tool for finding files using workspaces and full-text search. Register directories, label files with custom tags, and search across everything instantly — no daemon, no cloud, no setup.

```
███████╗ ██████╗ ██████╗ ██╗   ██╗████████╗
██╔════╝██╔════╝██╔═══██╗██║   ██║╚══██╔══╝
███████╗██║     ██║   ██║██║   ██║   ██║
╚════██║██║     ██║   ██║██║   ██║   ██║
███████║╚██████╗╚██████╔╝╚██████╔╝   ██║
╚══════╝ ╚═════╝ ╚═════╝  ╚═════╝    ╚═╝
```

## Install

**macOS / Linux**

```sh
curl -fsSL https://scout.paulanagnostou04.workers.dev/install.sh | sh
```

**Windows (PowerShell)**

```powershell
$repo = "pavlitoss/scout-cli"
$version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
$url = "https://github.com/$repo/releases/download/$version/scout-windows-amd64.exe"
$dest = "$env:LOCALAPPDATA\Programs\scout\scout.exe"
New-Item -ItemType Directory -Force -Path (Split-Path $dest) | Out-Null
Invoke-WebRequest $url -OutFile $dest
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$(Split-Path $dest)", "User")
Write-Host "Done. Restart your terminal and run 'scout --help' to get started."
```

Or build from source:

```sh
git clone https://github.com/pavlitoss/scout-cli
cd scout-cli
make install
```

## Usage

```sh
# Register a directory
scout watch add ~/code/myproject

# Search across all tracked files
scout "postgres config"

# List files under a tag
scout @myproject

# Scoped search
scout @myproject "database"
```

## Commands

### `scout watch`

| Command | Description |
|---|---|
| `scout watch add <path>` | Register a directory as a workspace |
| `scout watch remove <path>` | Unregister a workspace |
| `scout watch list` | List all workspaces with file counts |
| `scout watch sync` | Rescan workspaces for new and deleted files |

When you add a workspace, scout indexes all files inside it. The directory name becomes an implicit tag — `~/code/myproject` becomes `@myproject`.

### `scout tag`

| Command | Description |
|---|---|
| `scout tag add @name <path>` | Tag a file or directory |
| `scout tag remove @name <path>` | Remove a tag from a file or directory |
| `scout tag list` | List all tags with file counts |
| `scout tag show @name` | List all files under a tag |

Tags let you slice your indexed files into custom groups. A file inside `~/code/myproject` can also be tagged `@backend`, `@config`, or anything else.

### `scout prune`

Remove DB entries for files that no longer exist on disk.

```sh
scout prune
# Pruned 3 stale entries
```

### Search shortcuts

```sh
scout "query"           # search all tracked files
scout @tag              # list all files under a tag
scout @tag "query"      # search within a tag
```

## Excluding files

Scout has sensible defaults — it skips `node_modules`, `.git`, `dist`, `__pycache__`, `.venv`, system paths (`/proc`, `/sys`, `/dev`), and binary files.

**Per-workspace:** place a `.scoutignore` file at the root of a watched directory. Uses gitignore-style glob patterns:

```
# .scoutignore
*.log
*.lock
tmp/
secrets/
```

**Global:** add rules to `~/.scout/config.toml`:

```toml
[ignore]
dirs = ["vendor", ".terraform"]
extensions = [".pyc", ".class"]
```

## Data

Everything is stored in `~/.scout/scout.db` — a single SQLite file. No daemon, no background process. Search is powered by SQLite FTS5.

## Build

```sh
make build      # build ./scout
make install    # build and install to /usr/local/bin + man page
make uninstall  # remove binary and man page
make clean      # remove local build artifact
```

## Man page

```sh
man scout
```

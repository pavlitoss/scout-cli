package scanner

import (
	"bufio"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Result holds the scanned metadata for a single file.
type Result struct {
	Path    string
	Name    string
	Preview *string // nil for binary files, unreadable files, or files over 1MB
}

// Options controls additional exclusions on top of the built-in defaults.
type Options struct {
	ExtraDirs       []string // extra directory names to skip
	ExtraExtensions []string // extra file extensions to skip (e.g. ".pyc")
	IgnorePatterns  []string // glob patterns from .scoutignore (matched against base name)
}

// defaultExcludedDirs are directory names that are always skipped.
var defaultExcludedDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".hg":          true,
	".svn":         true,
	"dist":         true,
	"build":        true,
	"out":          true,
	"target":       true,
	".cache":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"env":          true,
}

// defaultExcludedPaths are absolute path prefixes that are always skipped.
var defaultExcludedPaths = []string{
	"/proc", "/sys", "/dev", "/run", "/tmp",
}

// ScanDir walks root recursively and returns a Result for each non-excluded file.
func ScanDir(root string, opts Options) ([]Result, error) {
	extraDirs := make(map[string]bool, len(opts.ExtraDirs))
	for _, d := range opts.ExtraDirs {
		extraDirs[d] = true
	}

	extraExts := make(map[string]bool, len(opts.ExtraExtensions))
	for _, e := range opts.ExtraExtensions {
		extraExts[strings.ToLower(e)] = true
	}

	var results []Result

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip paths we can't access rather than aborting the whole scan.
			return nil
		}

		name := d.Name()

		// 1. Skip system path prefixes.
		for _, excluded := range defaultExcludedPaths {
			if strings.HasPrefix(path, excluded) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// 2. Skip built-in excluded directory names.
		if d.IsDir() && defaultExcludedDirs[name] {
			return filepath.SkipDir
		}

		// 3. Skip user-supplied extra directories.
		if d.IsDir() && extraDirs[name] {
			return filepath.SkipDir
		}

		// 4. Skip hidden entries (dot-files and dot-directories), except the root itself.
		if path != root && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 5. Skip symlinks to avoid cycles.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		// Don't emit results for directories — just keep descending.
		if d.IsDir() {
			return nil
		}

		// 6. Skip files with excluded extensions.
		ext := strings.ToLower(filepath.Ext(name))
		if ext != "" && extraExts[ext] {
			return nil
		}

		// 7. Skip files matching .scoutignore patterns (matched against the base name).
		for _, pattern := range opts.IgnorePatterns {
			if matched, _ := filepath.Match(pattern, name); matched {
				return nil
			}
		}

		results = append(results, Result{
			Path:    path,
			Name:    name,
			Preview: ReadPreview(path),
		})
		return nil
	})

	return results, err
}

// ReadPreview reads the first 200 runes of a text file.
// Returns nil for binary files, unreadable files, or files larger than 1MB.
func ReadPreview(path string) *string {
	const maxSize = 1 << 20 // 1MB

	info, err := os.Stat(path)
	if err != nil || info.Size() > maxSize {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return nil
	}

	// Detect content type to skip binary files.
	contentType := http.DetectContentType(buf[:n])
	if !strings.HasPrefix(contentType, "text/") {
		return nil
	}

	// Truncate to 200 runes to avoid cutting multi-byte characters.
	runes := []rune(string(buf[:n]))
	if len(runes) > 200 {
		runes = runes[:200]
	}
	s := string(runes)
	return &s
}

// LoadIgnoreFile reads .scoutignore at the workspace root.
// Returns an empty slice if the file does not exist.
func LoadIgnoreFile(workspaceRoot string) ([]string, error) {
	p := filepath.Join(workspaceRoot, ".scoutignore")
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, scanner.Err()
}

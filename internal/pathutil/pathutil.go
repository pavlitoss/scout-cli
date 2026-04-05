package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// Normalize converts a user-supplied path to an absolute, clean path.
// Expands ~ to the home directory, resolves relative paths, removes
// trailing slashes and redundant segments.
func Normalize(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[2:])
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// IsDir returns true if the path exists and is a directory.
// Returns false (not an error) if the path does not exist.
func IsDir(p string) (bool, error) {
	info, err := os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

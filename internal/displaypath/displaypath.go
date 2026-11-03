// Package displaypath formats filesystem paths for terminal output.
package displaypath

import (
	"os"
	"path/filepath"
	"strings"
)

// Fold replaces a leading home directory with ~ for display. Only whole path segments
// match, so a sibling such as /home/developer is left unchanged when the home directory
// is /home/dev. Keep absolute paths for filesystem calls.
func Fold(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return filepath.Join("~", rest)
	}
	return path
}

// FoldText folds every home-directory path inside free text, such as an error message
// that embeds a path. A match must continue with a path separator, so a sibling such as
// /home/developer is left unchanged when the home directory is /home/dev.
func FoldText(text string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return text
	}
	return strings.ReplaceAll(text, home+string(filepath.Separator), "~"+string(filepath.Separator))
}

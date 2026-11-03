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
// that embeds a path. A match must start the text or follow a character that cannot be
// part of the path (whitespace, a quote, ':', '=', or '('), and must continue with a path
// separator. A sibling such as /home/developer, or /srv/backup/home/dev/x when the home
// directory is /home/dev, is left unchanged.
func FoldText(text string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return text
	}
	prefix := home + string(filepath.Separator)
	var b strings.Builder
	for {
		i := strings.Index(text, prefix)
		if i < 0 {
			b.WriteString(text)
			return b.String()
		}
		b.WriteString(text[:i])
		if i == 0 || strings.ContainsRune(" \t\n\"'`:=(", rune(text[i-1])) {
			b.WriteString("~" + string(filepath.Separator))
		} else {
			b.WriteString(prefix)
		}
		text = text[i+len(prefix):]
	}
}

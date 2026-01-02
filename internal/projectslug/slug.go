package projectslug

import (
	"path/filepath"
	"strings"
	"unicode"
)

func FromPathBase(path string) string {
	return Slugify(filepath.Base(path))
}

// Slugify makes a Cloud Run-friendly name:
// lowercase, a-z0-9-, no leading/trailing '-', collapse repeats.
func Slugify(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)

	var b strings.Builder
	prevDash := false

	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if unicode.IsSpace(r) || r == '_' || r == '-' || r == '.' {
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
			continue
		}
		// drop other characters
	}

	out := b.String()
	out = strings.Trim(out, "-")
	// collapse accidental empties
	if out == "" {
		return ""
	}
	return out
}
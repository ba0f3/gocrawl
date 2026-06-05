package utils

import "strings"

// SanitizeForLog removes newline characters to prevent log injection
func SanitizeForLog(s string) string {
	idx := strings.IndexAny(s, "\n\r")
	if idx == -1 {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:idx])
	for i := idx; i < len(s); i++ {
		if c := s[i]; c != '\n' && c != '\r' {
			b.WriteByte(c)
		}
	}
	return b.String()
}

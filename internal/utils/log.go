package utils

import "strings"

// SanitizeForLog removes newline characters to prevent log injection
func SanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

package utils

// HasAnyLowercasePattern returns true if any of the provided patterns
// (which must be lowercase) are present in s.
// It performs a case-insensitive check without allocating memory for strings.ToLower.
func HasAnyLowercasePattern(s string, patterns []string) bool {
	sLen := len(s)
	if sLen == 0 {
		return false
	}
	for _, p := range patterns {
		pLen := len(p)
		if pLen > sLen {
			continue
		}
		for i := 0; i <= sLen-pLen; i++ {
			match := true
			for j := 0; j < pLen; j++ {
				c := s[i+j]
				if c >= 'A' && c <= 'Z' {
					c += 'a' - 'A'
				}
				if c != p[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

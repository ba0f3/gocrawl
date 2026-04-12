package utils

// ⚡ Bolt Optimization: Zero-allocation case-insensitive substring matcher.
// Note: This optimization requires all strings in the 'patterns' slice to be pre-lowercased.
func HasAnyLowercasePattern(s string, patterns []string) bool {
	sLen := len(s)
	if sLen == 0 {
		return false
	}

	for _, p := range patterns {
		pLen := len(p)
		if pLen == 0 {
			return true
		}
		if pLen > sLen {
			continue
		}

		p0 := p[0]

		for i := 0; i <= sLen-pLen; i++ {
			c := s[i]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != p0 {
				continue
			}

			match := true
			for j := 1; j < pLen; j++ {
				cj := s[i+j]
				if cj >= 'A' && cj <= 'Z' {
					cj += 'a' - 'A'
				}
				if cj != p[j] {
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

package utils

// HasAnyLowercasePattern returns true if any of the provided patterns
// (which must be lowercase ASCII) are present in s.
// It performs a case-insensitive check without allocating memory for strings.ToLower.
// Note that HasAnyLowercasePattern's case-insensitive matching is ASCII-only
// (it only folds A-Z bytes) and does not perform Unicode case folding.
func HasAnyLowercasePattern(s string, patterns []string) bool {
	sLen := len(s)
	if sLen == 0 {
		return false
	}
	for _, p := range patterns {
		pLen := len(p)
		if pLen == 0 || pLen > sLen {
			continue
		}
		p0 := p[0]
		if pLen == 1 {
			for i := 0; i <= sLen-1; i++ {
				c := s[i]
				if c >= 'A' && c <= 'Z' {
					c += 'a' - 'A'
				}
				if c == p0 {
					return true
				}
			}
			continue
		}
		p1 := p[1]
		for i := 0; i <= sLen-pLen; i++ {
			c0 := s[i]
			if c0 >= 'A' && c0 <= 'Z' {
				c0 += 'a' - 'A'
			}
			if c0 != p0 {
				continue
			}

			c1 := s[i+1]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c1 != p1 {
				continue
			}

			match := true
			for j := 2; j < pLen; j++ {
				c2 := s[i+j]
				if c2 >= 'A' && c2 <= 'Z' {
					c2 += 'a' - 'A'
				}
				if c2 != p[j] {
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

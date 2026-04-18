package utils

import "testing"

func TestHasAnyLowercasePattern(t *testing.T) {
	tests := []struct {
		s        string
		patterns []string
		expected bool
	}{
		{"", []string{"test"}, false},
		{"hello", []string{}, false},
		{"Hello World", []string{"world"}, true},
		{"Hello World", []string{"hello"}, true},
		{"Hello World", []string{"xyz", "lo w"}, true},
		{"Hello World", []string{"WORLD"}, false},
	}

	for i, tt := range tests {
		result := HasAnyLowercasePattern(tt.s, tt.patterns)
		if result != tt.expected {
			t.Errorf("Test %d failed: expected %v, got %v for string %q and patterns %v", i, tt.expected, result, tt.s, tt.patterns)
		}
	}
}

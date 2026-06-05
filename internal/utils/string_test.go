package utils

import (
	"testing"
)

func TestHasAnyLowercasePattern(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		patterns []string
		want     bool
	}{
		{
			name:     "empty string",
			s:        "",
			patterns: []string{"test"},
			want:     false,
		},
		{
			name:     "no patterns",
			s:        "test string",
			patterns: []string{},
			want:     false,
		},
		{
			name:     "pattern larger than string",
			s:        "test",
			patterns: []string{"test string"},
			want:     false,
		},
		{
			name:     "exact match",
			s:        "test",
			patterns: []string{"test"},
			want:     true,
		},
		{
			name:     "exact match case insensitive",
			s:        "TeSt",
			patterns: []string{"test"},
			want:     true,
		},
		{
			name:     "substring match",
			s:        "this is a test string",
			patterns: []string{"test"},
			want:     true,
		},
		{
			name:     "substring match case insensitive",
			s:        "this is a TeSt string",
			patterns: []string{"test"},
			want:     true,
		},
		{
			name:     "multiple patterns, first matches",
			s:        "test string",
			patterns: []string{"test", "other"},
			want:     true,
		},
		{
			name:     "multiple patterns, last matches",
			s:        "test string",
			patterns: []string{"other", "string"},
			want:     true,
		},
		{
			name:     "multiple patterns, no match",
			s:        "test string",
			patterns: []string{"other", "value"},
			want:     false,
		},
		{
			name:     "one character pattern",
			s:        "a test string",
			patterns: []string{"a"},
			want:     true,
		},
		{
			name:     "one character pattern, no match",
			s:        "test string",
			patterns: []string{"z"},
			want:     false,
		},
		{
			name:     "one character pattern uppercase in string",
			s:        "A test string",
			patterns: []string{"a"},
			want:     true,
		},
		{
			name:     "two character pattern",
			s:        "test string",
			patterns: []string{"te"},
			want:     true,
		},
		{
			name:     "empty pattern",
			s:        "test string",
			patterns: []string{""},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAnyLowercasePattern(tt.s, tt.patterns); got != tt.want {
				t.Errorf("HasAnyLowercasePattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

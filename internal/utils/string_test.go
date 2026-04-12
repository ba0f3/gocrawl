package utils

import "testing"

func TestHasAnyLowercasePattern(t *testing.T) {
	cases := []struct {
		s        string
		patterns []string
		want     bool
	}{
		{"", []string{"a"}, false},
		{"a", []string{""}, true},
		{"Hello World", []string{"hello"}, true},
		{"Hello World", []string{"world"}, true},
		{"HelloWorld", []string{"world"}, true},
		{"helloworld", []string{"hello"}, true},
		{"test", []string{"notfound"}, false},
		{"mixedCaseString", []string{"case"}, true},
		{"mixedCaseString", []string{"string"}, true},
		{"short", []string{"verylongpatternhere"}, false},
		{"multiple", []string{"one", "two", "multiple"}, true},
	}

	for _, tc := range cases {
		got := HasAnyLowercasePattern(tc.s, tc.patterns)
		if got != tc.want {
			t.Errorf("HasAnyLowercasePattern(%q, %v) = %v; want %v", tc.s, tc.patterns, got, tc.want)
		}
	}
}

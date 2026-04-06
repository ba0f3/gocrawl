package utils

import "testing"

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no newlines",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "with newline",
			input:    "hello\nworld",
			expected: "helloworld",
		},
		{
			name:     "with carriage return",
			input:    "hello\rworld",
			expected: "helloworld",
		},
		{
			name:     "with both",
			input:    "hello\r\nworld",
			expected: "helloworld",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple occurrences",
			input:    "\n\rhello\n\r world\r\n",
			expected: "hello world",
		},
		{
			name:     "only newlines",
			input:    "\n\n\r\r",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLog(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeForLog(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

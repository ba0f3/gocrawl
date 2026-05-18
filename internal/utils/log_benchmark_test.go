package utils

import (
	"strings"
	"testing"
)

var sampleStrings = []string{
	"hello world",
	"hello\nworld",
	"hello\rworld",
	"hello\r\nworld",
	"a long string that has no newlines and is just normal text",
	"a long string that has\nsome newlines\r\nand some carriage\rreturns",
	strings.Repeat("a", 100) + "\n" + strings.Repeat("b", 100),
}

func BenchmarkSanitizeForLog(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, s := range sampleStrings {
			_ = SanitizeForLog(s)
		}
	}
}

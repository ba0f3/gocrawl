package extractor

import (

	"testing"
)

func BenchmarkFilterReadable(b *testing.B) {
	s := "This is a readable string that is long enough to be interesting and should pass the filter check without any issues."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filterReadable(s)
	}
}

func BenchmarkFilterReadable_Rejects(b *testing.B) {
	s := "http://example.com/foo/bar"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filterReadable(s)
	}
}

package main

import (
	"fmt"
	"strings"
	"testing"
)

var paths = []string{"/docs", "/api", "/blog", "/very-long-path-that-is-not-matched", "/another-long-path", "/one-more", "/more-stuff", "/and-more"}
var excludePaths = []string{"/login", "/admin", "/private", "/exclude", "/skip", "/ignore", "/no-scrape"}

var testPaths = []string{
	"/docs/getting-started",
	"/api/v1/users",
	"/blog/hello-world",
	"/login",
	"/admin/dashboard",
	"/about",
	"/docs",
	"/another-unmatched-path",
	"/some-random-string",
	"/long-but-no-match",
}

func BenchmarkContains(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tp := range testPaths {
			if len(paths) > 0 {
				included := false
				for _, p := range paths {
					if strings.Contains(tp, p) {
						included = true
						break
					}
				}
				if !included {
					continue
				}
			}

			for _, p := range excludePaths {
				if strings.Contains(tp, p) {
					break
				}
			}
		}
	}
}

func main() {
	resContains := testing.Benchmark(BenchmarkContains)
	fmt.Printf("Contains: %v\n", resContains)
}

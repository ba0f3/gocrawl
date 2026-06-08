package utils

import (
	"net/url"
	"testing"
)

func BenchmarkResolveHref(b *testing.B) {
	baseURL, _ := url.Parse("https://example.com/some/path")
	hrefs := []string{
		"https://example.com/other",
		"/root-relative/path",
		"../relative/path",
		"?query=1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, href := range hrefs {
			_ = ResolveHref(baseURL, href)
		}
	}
}

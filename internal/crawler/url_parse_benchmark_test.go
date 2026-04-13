package crawler

import (
	"net/url"
	"testing"

	"gocrawl/internal/utils"
)

func BenchmarkAppendResolvedHref_Old(b *testing.B) {
	pageURL := "https://example.com/some/path/page.html"
	href := "/relative/link?q=1"

	seen := make(map[string]struct{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		baseURL, _ := url.Parse(pageURL)
		linkURL, _ := url.Parse(href)
		abs := baseURL.ResolveReference(linkURL).String()
		if _, ok := seen[abs]; ok {
			continue
		}
	}
}

func BenchmarkAppendResolvedHref_New(b *testing.B) {
	pageURL := "https://example.com/some/path/page.html"
	href := "/relative/link?q=1"

	seen := make(map[string]struct{})

	baseURL, _ := url.Parse(pageURL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		linkURL, _ := url.Parse(href)
		abs := baseURL.ResolveReference(linkURL).String()
		if _, ok := seen[abs]; ok {
			continue
		}
	}
}

func BenchmarkAppendResolvedHref_Bolt(b *testing.B) {
	pageURL := "https://example.com/some/path/page.html"
	href := "/relative/link?q=1"

	seen := make(map[string]struct{})

	baseURL, _ := url.Parse(pageURL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		abs := utils.ResolveHref(baseURL, href)
		if _, ok := seen[abs]; ok {
			continue
		}
	}
}

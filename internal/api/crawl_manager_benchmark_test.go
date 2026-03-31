package api

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func BenchmarkShouldScrapeURL(b *testing.B) {
	cm := &CrawlManager{}
	baseURL, _ := url.Parse("https://example.com")

	req := &CrawlRequestBody{
		AllowExternalLinks: false,
		AllowSubdomains:    false,
		IncludePaths: []string{
			"/docs", "/api", "/blog", "/very-long-path-that-is-not-matched",
			"/another-long-path", "/one-more", "/more-stuff", "/and-more",
		},
		ExcludePaths: []string{
			"/login", "/admin", "/private", "/exclude", "/skip", "/ignore", "/no-scrape",
		},
	}

	testCases := []string{
		"https://example.com/docs/getting-started",
		"https://example.com/api/v1/users",
		"https://example.com/blog/hello-world",
		"https://example.com/login",
		"https://example.com/admin/dashboard",
		"https://example.com/about",
		"https://example.com/docs",
		"https://example.com/another-unmatched-path",
		"https://example.com/some-random-string",
		"https://example.com/long-but-no-match",
		"https://external.com/docs",
	}

	var includeRe, excludeRe *regexp.Regexp
	if len(req.IncludePaths) > 0 {
		var parts []string
		for _, p := range req.IncludePaths {
			parts = append(parts, regexp.QuoteMeta(p))
		}
		includeRe = regexp.MustCompile(strings.Join(parts, "|"))
	}
	if len(req.ExcludePaths) > 0 {
		var parts []string
		for _, p := range req.ExcludePaths {
			parts = append(parts, regexp.QuoteMeta(p))
		}
		excludeRe = regexp.MustCompile(strings.Join(parts, "|"))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, u := range testCases {
			cm.shouldScrapeURL(u, baseURL, req, includeRe, excludeRe)
		}
	}
}

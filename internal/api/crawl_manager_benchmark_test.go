package api

import (
	"bytes"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
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

	expectedOrigin := baseURL.Scheme + "://" + baseURL.Host

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, u := range testCases {
			cm.shouldScrapeURL(u, baseURL, expectedOrigin, req, includeRe, excludeRe)
		}
	}
}

func BenchmarkLinkInArticleOrMain(b *testing.B) {
	html := `<html><body><div class="container"><article><p><a href="/link">link</a></p></article></div></body></html>`
	doc, _ := goquery.NewDocumentFromReader(bytes.NewReader([]byte(html)))
	sel := doc.Find("a").First()
	e := &colly.HTMLElement{DOM: sel}

	b.ResetTimer()
	for b.Loop() {
		linkInArticleOrMain(e)
	}
}

func BenchmarkLinkInArticleOrMainNotFound(b *testing.B) {
	html := `<html><body><div class="container"><footer><p><a href="/link">link</a></p></footer></div></body></html>`
	doc, _ := goquery.NewDocumentFromReader(bytes.NewReader([]byte(html)))
	sel := doc.Find("a").First()
	e := &colly.HTMLElement{DOM: sel}

	b.ResetTimer()
	for b.Loop() {
		linkInArticleOrMain(e)
	}
}

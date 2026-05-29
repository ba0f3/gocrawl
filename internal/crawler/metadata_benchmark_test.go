package crawler

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var benchmarkMetadataHtmlStr = `<html><head><title>My Title</title><meta name="description" content="My Desc"></head><body>` + strings.Repeat(`<div><p>Some text</p></div>`, 10000) + `</body></html>`

func BenchmarkExtractMetadataGoquery(b *testing.B) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(benchmarkMetadataHtmlStr))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doc.Find("title").Text()
		_ = doc.Find("meta[name='description']").AttrOr("content", "")
		_ = doc.Find("html").AttrOr("lang", "")
	}
}

func BenchmarkExtractMetadataFast(b *testing.B) {
	doc, err := html.Parse(strings.NewReader(benchmarkMetadataHtmlStr))
	if err != nil {
		b.Fatal(err)
	}
	nodes := []*html.Node{doc}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = extractMetadataFast(nodes)
	}
}

package crawler

import (
	"bytes"
	"testing"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractMetadataFastGoQueryParity(t *testing.T) {
	tests := []struct {
		name string
		html string
	}{
		{
			name: "Standard metadata",
			html: `<!DOCTYPE html><html lang="en"><head><title>My Test Title</title><meta name="description" content="This is a test description."></head><body><p>Hello world</p></body></html>`,
		},
		{
			name: "Nested text in title",
			html: `<!DOCTYPE html><html><head><title>My <b>Test</b> Title</title></head><body></body></html>`,
		},
		{
			name: "Case sensitive meta name match",
			html: `<!DOCTYPE html><html><head><meta name="description" content="Match this."></head><body></body></html>`,
		},
		{
			name: "Case sensitive meta name mismatch",
			html: `<!DOCTYPE html><html><head><meta name="Description" content="Should not match."></head><body></body></html>`,
		},
		{
			name: "Missing title and description",
			html: `<!DOCTYPE html><html lang="fr"><head></head><body></body></html>`,
		},
		{
			name: "Multiple titles - first wins in traversal order",
			html: `<!DOCTYPE html><html><head><title>Title 1</title><title>Title 2</title></head><body></body></html>`,
		},
		{
			name: "XML lang",
			html: `<!DOCTYPE html><html xml:lang="en" lang="es"><head><title>T</title></head><body></body></html>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _ := goquery.NewDocumentFromReader(bytes.NewReader([]byte(tt.html)))
			gqTitle := doc.Find("title").Text()
			gqDesc := doc.Find("meta[name='description']").AttrOr("content", "")
			gqLang := doc.Find("html").AttrOr("lang", "")

			var title, desc, lang string
			for _, n := range doc.Nodes {
				extractMetadataFast(n, &title, &desc, &lang)
			}

			if gqTitle != title {
				t.Errorf("Title parity failed: GoQuery %q vs Fast %q", gqTitle, title)
			}

			if gqDesc != desc {
				t.Errorf("Desc parity failed: GoQuery %q vs Fast %q", gqDesc, desc)
			}

			if gqLang != lang {
				t.Errorf("Lang parity failed: GoQuery %q vs Fast %q", gqLang, lang)
			}
		})
	}
}

func BenchmarkMetadataGoqueryVsFast(b *testing.B) {
	htmlDoc := `
	<html lang="en">
	<head>
	<title>Test Title</title>
	<meta name="description" content="Test description">
	</head>
	<body>
	<p>Hello world</p>
	<div>` + strings.Repeat("<p>More content</p>", 1000) + `</div>
	</body>
	</html>
	`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(htmlDoc))

	b.Run("Goquery", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = doc.Find("title").Text()
			_ = doc.Find("meta[name='description']").AttrOr("content", "")
			_ = doc.Find("html").AttrOr("lang", "")
		}
	})

	b.Run("Fast", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var title, desc, lang string
			for _, n := range doc.Nodes {
				extractMetadataFast(n, &title, &desc, &lang)
			}
			_, _, _ = title, desc, lang
		}
	})
}

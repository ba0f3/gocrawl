package extractor

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func BenchmarkScoreNode(b *testing.B) {
	htmlContent := `
		<div class="content article main" role="main">
			<p>This is a paragraph.</p>
			<p>Here is another paragraph with a <a href="#">link</a>.</p>
			<div>
				<p>Deep paragraph.</p>
			</div>
		</div>
	`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	sel := doc.Find("div.content").First()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scoreNode(sel)
	}
}

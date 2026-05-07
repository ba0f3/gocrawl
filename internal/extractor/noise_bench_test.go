package extractor

import (
	"strings"
	"testing"
	"github.com/PuerkitoBio/goquery"
)

func BenchmarkIsNoise(b *testing.B) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
		<html><body>
			<div class="Header-Nav Cookie-Banner ADVERT some-other-class Sidebar" id="Cookie-Banner-123">
				Some text
			</div>
			<div class="Main-Content">
				Content
			</div>
		</body></html>
	`))
	if err != nil {
		b.Fatal(err)
	}
	sel := doc.Find("div").First()
	sel2 := doc.Find(".Main-Content").First()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsNoise(sel)
		IsNoise(sel2)
	}
}

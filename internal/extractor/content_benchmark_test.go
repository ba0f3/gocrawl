package extractor

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func BenchmarkRecoverMarkdownH1(b *testing.B) {
	htmlContent := `
		<html>
			<head><title>Test Title</title></head>
			<body>
				<div>Some content before</div>
				<h1>This is the main title</h1>
				<div>Some content after</div>
			</body>
		</html>
	`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	md := "Here is some markdown text without the title."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RecoverMarkdownH1(doc, md)
	}
}

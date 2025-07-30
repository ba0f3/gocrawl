package extractor

import (
	"github.com/jaytaylor/html2text"
)

// ToMarkdown converts HTML content to Markdown
func ToMarkdown(htmlContent string) (string, error) {
	markdown, err := html2text.FromString(htmlContent)
	if err != nil {
		return "", err
	}
	return markdown, nil
}

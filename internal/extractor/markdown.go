package extractor

import (
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// ToMarkdown converts HTML content to Markdown
func ToMarkdown(htmlContent string) (string, error) {
	markdown, err := htmltomarkdown.ConvertString(htmlContent)
	if err != nil {
		return "", err
	}
	return markdown, nil
}

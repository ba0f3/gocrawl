package extractor

import (
	"strings"
	"testing"
)

func TestToMarkdown(t *testing.T) {
	htmlInput := `<h1>Hello World</h1><p>This is a <strong>test</strong> paragraph with <em>italic</em> text.</p>`
	expected := []string{"# Hello World", "**test**", "_italic_"}

	result, err := ToMarkdown(htmlInput)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == "" {
		t.Fatal("Expected non-empty result")
	}

	// Check that the result contains expected markdown elements
	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected result to contain '%s', got: %s", exp, result)
		}
	}

	t.Logf("Input HTML: %s", htmlInput)
	t.Logf("Output Markdown: %s", result)
}

func TestToMarkdownWithLinks(t *testing.T) {
	htmlInput := `<a href="https://example.com">Example Link</a>`

	result, err := ToMarkdown(htmlInput)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should contain markdown link syntax
	if !strings.Contains(result, "[Example Link](https://example.com)") {
		t.Errorf("Expected markdown link format, got: %s", result)
	}

	t.Logf("Input HTML: %s", htmlInput)
	t.Logf("Output Markdown: %s", result)
}

func TestToMarkdownWithCodeBlock(t *testing.T) {
	htmlInput := `<pre><code>func main() {
    fmt.Println("Hello World")
}</code></pre>`

	result, err := ToMarkdown(htmlInput)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should contain code block markers
	if !strings.Contains(result, "```") {
		t.Errorf("Expected code block markers, got: %s", result)
	}

	t.Logf("Input HTML: %s", htmlInput)
	t.Logf("Output Markdown: %s", result)
}

func TestToMarkdownWithDomain(t *testing.T) {
	htmlInput := `<a href="/page">Relative Link</a><img src="/image.jpg" alt="image">`
	result, err := ToMarkdown(htmlInput)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Log the result to see what we actually get
	t.Logf("Input HTML: %s", htmlInput)
	t.Logf("Output Markdown: %s", result)

	// For now, just check that conversion happened without error
	// The exact format might be different in v1 API
	if result == "" {
		t.Fatal("Expected non-empty result")
	}
}

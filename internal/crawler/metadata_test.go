package crawler

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestExtractMetadataFast(t *testing.T) {
	t.Run("Extract title, desc and lang", func(t *testing.T) {
		htmlStr := `<html><head><title>My Title</title><meta name="description" content="My Desc"></head><html lang="en"><body></body></html>`
		doc, err := html.Parse(strings.NewReader(htmlStr))
		if err != nil {
			t.Fatal(err)
		}

		title, desc, lang := extractMetadataFast([]*html.Node{doc})

		if title != "My Title" {
			t.Errorf("expected title 'My Title', got %q", title)
		}
		if desc != "My Desc" {
			t.Errorf("expected desc 'My Desc', got %q", desc)
		}
		if lang != "en" {
			t.Errorf("expected lang 'en', got %q", lang)
		}
	})

	t.Run("Case-insensitive match for name=description", func(t *testing.T) {
		htmlStr := `<html><head><meta name="Description" content="Case Insensitive Desc"></head><body></body></html>`
		doc, err := html.Parse(strings.NewReader(htmlStr))
		if err != nil {
			t.Fatal(err)
		}

		_, desc, _ := extractMetadataFast([]*html.Node{doc})

		if desc != "Case Insensitive Desc" {
			t.Errorf("expected desc 'Case Insensitive Desc', got %q", desc)
		}
	})

	t.Run("Early return on body tag", func(t *testing.T) {
		htmlStr := `<html><head></head><body><title>Should Not Extract</title></body></html>`
		doc, err := html.Parse(strings.NewReader(htmlStr))
		if err != nil {
			t.Fatal(err)
		}

		title, _, _ := extractMetadataFast([]*html.Node{doc})

		if title != "" {
			t.Errorf("expected empty title, got %q", title)
		}
	})
}

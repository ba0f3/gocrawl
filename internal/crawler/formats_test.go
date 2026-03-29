package crawler

import "testing"

func TestWantsFormatEmptyMeansAll(t *testing.T) {
	if !wantsFormat(&ScrapeRequest{}, "html") || !wantsFormat(&ScrapeRequest{Formats: nil}, "markdown") {
		t.Fatal("empty formats should request all output kinds")
	}
}

func TestWantsFormatExplicit(t *testing.T) {
	req := &ScrapeRequest{Formats: []string{"markdown"}}
	if !wantsFormat(req, "markdown") || wantsFormat(req, "html") || wantsFormat(req, "rawHtml") {
		t.Fatal("only markdown")
	}
}

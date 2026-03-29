package crawler

import (
	"testing"
)

func TestEffectiveOnlyMainContent(t *testing.T) {
	if !EffectiveOnlyMainContent(&ScrapeRequest{}) {
		t.Fatal("nil pointer should mean main content on")
	}
	f := false
	if EffectiveOnlyMainContent(&ScrapeRequest{OnlyMainContent: &f}) {
		t.Fatal("explicit false should be full body")
	}
	tr := true
	if !EffectiveOnlyMainContent(&ScrapeRequest{OnlyMainContent: &tr}) {
		t.Fatal("explicit true")
	}
}

func TestUserContentSelectors(t *testing.T) {
	req := &ScrapeRequest{
		ContentSelector:  " #main ",
		ContentSelectors: []string{".article", ""},
	}
	got := UserContentSelectors(req)
	if len(got) != 2 || got[0] != "#main" || got[1] != ".article" {
		t.Fatalf("got %q", got)
	}
	if UserContentSelectors(&ScrapeRequest{}) != nil {
		t.Fatal("expected nil")
	}
}

func TestEffectiveLinkSelector(t *testing.T) {
	if s := EffectiveLinkSelector(&ScrapeRequest{LinkSelector: " main a "}); s != "main a" {
		t.Fatalf("got %q", s)
	}
	if s := EffectiveLinkSelector(nil); s != "a[href]" {
		t.Fatalf("got %q", s)
	}
}

func TestSelectorsJSNonEmpty(t *testing.T) {
	js := SelectorsJS([]string{`div[data-x="y"]`})
	if js == "" || len(js) < 20 {
		t.Fatal("expected script body")
	}
}

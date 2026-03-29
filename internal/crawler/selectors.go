package crawler

import (
	"encoding/json"
	"fmt"
	"strings"
)

// UserContentSelectors returns non-empty selectors from the request (trimmed), or nil if unset.
func UserContentSelectors(req *ScrapeRequest) []string {
	if req == nil {
		return nil
	}
	var out []string
	if s := strings.TrimSpace(req.ContentSelector); s != "" {
		out = append(out, s)
	}
	for _, s := range req.ContentSelectors {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// EffectiveContentSelectors returns user-provided selectors when set, otherwise built-in main-content list.
func EffectiveContentSelectors(req *ScrapeRequest) []string {
	if u := UserContentSelectors(req); u != nil {
		return u
	}
	return MainContentSelectors()
}

// EffectiveLinkSelector returns the selector used to discover links on a scrape (default a[href]).
func EffectiveLinkSelector(req *ScrapeRequest) string {
	if req == nil {
		return "a[href]"
	}
	if s := strings.TrimSpace(req.LinkSelector); s != "" {
		return s
	}
	return "a[href]"
}

// MainContentSelectors lists CSS selectors tried in order for article/main body extraction.
func MainContentSelectors() []string {
	return []string{
		"main",
		"article",
		"[role='main']",
		"[itemprop='articleBody']",
		".article-body",
		".markdown-body",
		".post-content",
		".entry-content",
		"#main-content",
		"#content",
		".content",
		".post",
		".entry",
		"article .content",
	}
}

// MainContentSelectorsJS returns an IIFE script that returns innerHTML from the first matching selector or document.body.
func MainContentSelectorsJS() string {
	return SelectorsJS(MainContentSelectors())
}

// SelectorsJS builds the chromedp Evaluate script for the given CSS selectors (tried in order).
func SelectorsJS(selectors []string) string {
	if len(selectors) == 0 {
		selectors = MainContentSelectors()
	}
	var parts []string
	for _, s := range selectors {
		j, err := json.Marshal(s)
		if err != nil {
			continue
		}
		parts = append(parts, string(j))
	}
	if len(parts) == 0 {
		parts = append(parts, `"body"`)
	}
	return fmt.Sprintf(
		`(function(){var sels=[%s];for(var i=0;i<sels.length;i++){var el=document.querySelector(sels[i]);if(el&&el.innerHTML&&el.innerHTML.length>80)return el.innerHTML;}return document.body?document.body.innerHTML:'';})()`,
		strings.Join(parts, ","),
	)
}

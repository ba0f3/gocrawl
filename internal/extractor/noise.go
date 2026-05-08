package extractor

import (
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"gocrawl/internal/utils"
	"golang.org/x/net/html"
)

var noiseTags = map[string]struct{}{
	"script": {}, "style": {}, "noscript": {}, "iframe": {}, "svg": {}, "nav": {}, "aside": {},
	"footer": {}, "header": {}, "form": {}, "video": {}, "audio": {}, "canvas": {},
}

var exactNoiseClasses = []string{
	"header", "top", "navbar", "footer", "bottom", "sidebar",
	"modal", "popup", "overlay", "ad", "ads", "advert",
	"lang-selector", "language", "social", "social-media", "social-links",
	"menu", "navigation", "breadcrumbs", "breadcrumb", "share",
	"widget", "cookie", "newsletter", "subscribe", "skip-link",
	"sr-only", "visually-hidden", "notification", "alert", "toast",
	"pagination", "pager", "signup", "login-form", "search-form",
	"related-posts", "recommended",
}

var exactNoiseIDs = []string{
	"header", "footer", "nav", "sidebar", "menu", "modal",
	"popup", "cookie", "breadcrumbs", "widget", "ad", "social",
	"share", "newsletter", "subscribe", "comments", "related", "recommended",
}

var cookieConsentIDPrefixes = []string{
	"onetrust", "optanon", "ot-sdk", "cookiebot", "cybotcookiebot", "cc-", "cookie-law",
	"gdpr", "consent-", "cmp-", "sp_message", "qc-cmp", "trustarc", "evidon",
}

var cookieConsentClassPrefixes = []string{
	"onetrust", "optanon", "ot-sdk", "cookiebot", "cybotcookiebot", "cookie-law",
	"gdpr", "consent-", "sp_message", "trustarc", "evidon",
}

var structuralIDSuffixes = []string{"portal", "root", "container", "wrapper", "mount", "app"}

// IsNoise reports whether this node is structural/UI noise (nav, ads, cookie banners, etc.).
func IsNoise(sel *goquery.Selection) bool {
	if sel == nil || sel.Length() == 0 {
		return false
	}
	n := sel.Get(0)
	if n.Type != html.ElementNode {
		return false
	}
	tag := n.Data
	if tag == "body" || tag == "html" {
		return false
	}
	if _, ok := noiseTags[tag]; ok {
		return true
	}
	if role, _ := sel.Attr("role"); role != "" {
		// ⚡ Bolt Optimization: Use zero-allocation EqualFold instead of strings.ToLower
		rTrim := strings.TrimSpace(role)
		if strings.EqualFold(rTrim, "navigation") ||
			strings.EqualFold(rTrim, "banner") ||
			strings.EqualFold(rTrim, "complementary") ||
			strings.EqualFold(rTrim, "contentinfo") {
			return true
		}
	}
	if class, ok := sel.Attr("class"); ok {
		// ⚡ Bolt Optimization: Zero-allocation string scanning instead of strings.ToLower + strings.Contains
		if utils.HasAnyLowercasePattern(class, longNoisePatterns) {
			return true
		}

		// ⚡ Bolt Optimization: Use unified zero-allocation token scanning
		// instead of multiple passes with strings.Fields, isAdClass, and hasNoiseClassPattern.
		start := -1
		for i, r := range class {
			isSpace := unicode.IsSpace(r)
			if !isSpace && start == -1 {
				start = i
			} else if isSpace && start != -1 {
				tok := class[start:i]
				start = -1

				if isNoiseToken(tok) {
					return true
				}
			}
		}
		if start != -1 {
			if isNoiseToken(class[start:]) {
				return true
			}
		}
	}
	if id, ok := sel.Attr("id"); ok {
		// ⚡ Bolt Optimization: Zero-allocation case-insensitive matching
		isNoiseID := false
		for _, p := range exactNoiseIDs {
			if len(id) == len(p) && strings.EqualFold(id, p) {
				isNoiseID = true
				break
			}
		}

		if isNoiseID && !isStructuralID(id) {
			return true
		}

		for _, p := range cookieConsentIDPrefixes {
			if hasEqualFoldPrefix(id, p) {
				return true
			}
		}
	}
	return false
}

// IsNoiseDescendant is true if any ancestor is noise.
func IsNoiseDescendant(sel *goquery.Selection) bool {
	if sel == nil || sel.Length() == 0 {
		return false
	}
	// ⚡ Bolt Optimization: Manually traverse x/net/html nodes
	// instead of allocating new goquery.Selection objects in a loop.
	selWrapper := &goquery.Selection{Nodes: make([]*html.Node, 1)}
	for p := sel.Get(0).Parent; p != nil; p = p.Parent {
		// Update reusable wrapper element instead of creating new structs
		selWrapper.Nodes[0] = p
		if IsNoise(selWrapper) {
			return true
		}
	}
	return false
}

// ⚡ Bolt Optimization: Zero-allocation prefix check
func hasEqualFoldPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}

func isStructuralID(id string) bool {
	return utils.HasAnyLowercasePattern(id, structuralIDSuffixes)
}

func isNoiseToken(tok string) bool {
	for _, p := range cookieConsentClassPrefixes {
		if hasEqualFoldPrefix(tok, p) {
			return true
		}
	}
	for _, p := range exactNoiseClasses {
		if len(tok) == len(p) && strings.EqualFold(tok, p) {
			return true
		}
	}
	if hasEqualFoldPrefix(tok, "footer") || hasEqualFoldPrefix(tok, "header-") || hasEqualFoldPrefix(tok, "nav-") {
		return true
	}
	if isAdClassToken(tok) {
		return true
	}

	t := tok
	if idx := strings.LastIndex(t, ":"); idx >= 0 {
		t = t[idx+1:]
		for _, p := range exactNoiseClasses {
			if len(t) == len(p) && strings.EqualFold(t, p) {
				return true
			}
		}
		if hasEqualFoldPrefix(t, "footer") || hasEqualFoldPrefix(t, "header-") || hasEqualFoldPrefix(t, "nav-") {
			return true
		}
		if isAdClassToken(t) {
			return true
		}
	}
	for _, p := range shortNoisePatterns {
		if wordBoundaryMatch(t, p) {
			return true
		}
	}
	return false
}

func isAdClassToken(tok string) bool {
	if len(tok) == 2 && strings.EqualFold(tok, "ad") {
		return true
	}
	l := len(tok)
	if l >= 3 {
		c0, c1, c2 := tok[0], tok[1], tok[2]
		if c0 >= 'A' && c0 <= 'Z' {
			c0 += 'a' - 'A'
		}
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c0 == 'a' && c1 == 'd' && (c2 == '-' || c2 == '_') {
			return true
		}

		cEnd2, cEnd1, cEnd0 := tok[l-3], tok[l-2], tok[l-1]
		if cEnd1 >= 'A' && cEnd1 <= 'Z' {
			cEnd1 += 'a' - 'A'
		}
		if cEnd0 >= 'A' && cEnd0 <= 'Z' {
			cEnd0 += 'a' - 'A'
		}
		if (cEnd2 == '-' || cEnd2 == '_') && cEnd1 == 'a' && cEnd0 == 'd' {
			return true
		}
	}
	return false
}

var longNoisePatterns = []string{
	"sidebar", "navigation", "advertisement", "advert", "social-media", "social-links",
	"breadcrumb", "newsletter", "recommended", "pagination", "lang-selector",
}

var shortNoisePatterns = []string{"nav", "top", "side", "menu", "widget", "header", "footer"}

func wordBoundaryMatch(text, pattern string) bool {
	// ⚡ Bolt Optimization: Zero-allocation case-insensitive substring match with boundary checks
	tLen := len(text)
	pLen := len(pattern)
	if pLen > tLen {
		return false
	}
	for i := 0; i <= tLen-pLen; i++ {
		match := true
		for j := 0; j < pLen; j++ {
			c := text[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != pattern[j] {
				match = false
				break
			}
		}
		if match {
			beforeOK := i == 0 || text[i-1] == '-' || text[i-1] == '_'
			end := i + pLen
			afterOK := end == tLen || text[end] == '-' || text[end] == '_'
			if beforeOK && afterOK {
				return true
			}
		}
	}
	return false
}

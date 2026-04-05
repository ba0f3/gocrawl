package extractor

import (
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var noiseTags = map[string]struct{}{
	"script": {}, "style": {}, "noscript": {}, "iframe": {}, "svg": {}, "nav": {}, "aside": {},
	"footer": {}, "header": {}, "form": {}, "video": {}, "audio": {}, "canvas": {},
}

var noiseRoles = map[string]struct{}{
	"navigation": {}, "banner": {}, "complementary": {}, "contentinfo": {},
}

var noiseClassesExact = map[string]struct{}{
	"header": {}, "top": {}, "navbar": {}, "footer": {}, "bottom": {}, "sidebar": {},
	"modal": {}, "popup": {}, "overlay": {}, "ad": {}, "ads": {}, "advert": {},
	"lang-selector": {}, "language": {}, "social": {}, "social-media": {}, "social-links": {},
	"menu": {}, "navigation": {}, "breadcrumbs": {}, "breadcrumb": {}, "share": {},
	"widget": {}, "cookie": {}, "newsletter": {}, "subscribe": {}, "skip-link": {},
	"sr-only": {}, "visually-hidden": {}, "notification": {}, "alert": {}, "toast": {},
	"pagination": {}, "pager": {}, "signup": {}, "login-form": {}, "search-form": {},
	"related-posts": {}, "recommended": {},
}

var noiseIDsExact = map[string]struct{}{
	"header": {}, "footer": {}, "nav": {}, "sidebar": {}, "menu": {}, "modal": {},
	"popup": {}, "cookie": {}, "breadcrumbs": {}, "widget": {}, "ad": {}, "social": {},
	"share": {}, "newsletter": {}, "subscribe": {}, "comments": {}, "related": {}, "recommended": {},
}

var cookieConsentIDPrefixes = []string{
	"onetrust", "optanon", "ot-sdk", "cookiebot", "cybotcookiebot", "cc-", "cookie-law",
	"gdpr", "consent-", "cmp-", "sp_message", "qc-cmp", "trustarc", "evidon",
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
		if _, ok := noiseRoles[strings.ToLower(strings.TrimSpace(role))]; ok {
			return true
		}
	}
	if class, ok := sel.Attr("class"); ok {
		lc := strings.ToLower(class)
		for _, p := range cookieConsentIDPrefixes {
			if strings.Contains(lc, p) {
				return true
			}
		}
		for _, p := range longNoisePatterns {
			if strings.Contains(lc, p) {
				return true
			}
		}

		// ⚡ Bolt Optimization: Use unified zero-allocation token scanning
		// instead of multiple passes with strings.Fields, isAdClass, and hasNoiseClassPattern.
		start := -1
		for i, r := range lc {
			isSpace := unicode.IsSpace(r)
			if !isSpace && start == -1 {
				start = i
			} else if isSpace && start != -1 {
				tok := lc[start:i]
				start = -1

				if _, hit := noiseClassesExact[tok]; hit {
					return true
				}
				if strings.HasPrefix(tok, "footer") || strings.HasPrefix(tok, "header-") || strings.HasPrefix(tok, "nav-") {
					return true
				}
				if isAdClassToken(tok) {
					return true
				}

				t := tok
				if idx := strings.LastIndex(t, ":"); idx >= 0 {
					t = t[idx+1:]
				}
				for _, p := range shortNoisePatterns {
					if wordBoundaryMatch(t, p) {
						return true
					}
				}
			}
		}
		if start != -1 {
			tok := lc[start:]
			if _, hit := noiseClassesExact[tok]; hit {
				return true
			}
			if strings.HasPrefix(tok, "footer") || strings.HasPrefix(tok, "header-") || strings.HasPrefix(tok, "nav-") {
				return true
			}
			if isAdClassToken(tok) {
				return true
			}

			t := tok
			if idx := strings.LastIndex(t, ":"); idx >= 0 {
				t = t[idx+1:]
			}
			for _, p := range shortNoisePatterns {
				if wordBoundaryMatch(t, p) {
					return true
				}
			}
		}
	}
	if id, ok := sel.Attr("id"); ok {
		idLower := strings.ToLower(id)
		if _, hit := noiseIDsExact[idLower]; hit && !isStructuralID(idLower) {
			return true
		}
		for _, p := range cookieConsentIDPrefixes {
			if strings.HasPrefix(idLower, p) {
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
	for p := sel.Parent(); p.Length() > 0; p = p.Parent() {
		if IsNoise(p) {
			return true
		}
	}
	return false
}

func isStructuralID(id string) bool {
	for _, s := range structuralIDSuffixes {
		if strings.Contains(id, s) {
			return true
		}
	}
	return false
}

func isAdClassToken(tok string) bool {
	if tok == "ad" {
		return true
	}
	l := len(tok)
	if l >= 3 {
		if tok[0] == 'a' && tok[1] == 'd' && (tok[2] == '-' || tok[2] == '_') {
			return true
		}
		if (tok[l-3] == '-' || tok[l-3] == '_') && tok[l-2] == 'a' && tok[l-1] == 'd' {
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
	start := 0
	for {
		idx := strings.Index(text[start:], pattern)
		if idx < 0 {
			break
		}
		abs := start + idx
		beforeOK := abs == 0 || text[abs-1] == '-' || text[abs-1] == '_'
		end := abs + len(pattern)
		afterOK := end == len(text) || text[end] == '-' || text[end] == '_'
		if beforeOK && afterOK {
			return true
		}
		start = abs + 1
	}
	return false
}

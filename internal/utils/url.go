package utils

import (
	"net/url"
	"strings"
)

// ResolveHref performs an optimized, low-allocation fast-path resolution
// for common absolute and root-relative URLs. Absolute URLs are still validated
// with url.Parse, but are immediately returned on success to bypass further processing.
// Root-relative URLs avoid ResolveReference overhead by using simple string concatenation
// (which still allocates the output string).
func ResolveHref(baseURL *url.URL, href string) string {
	if href == "" || baseURL == nil {
		return ""
	}

	// Fast path: absolute URL
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		if _, err := url.Parse(href); err == nil {
			return href
		}
	} else if href[0] == '/' && (len(href) == 1 || href[1] != '/') {
		// Fast path: root-relative URL (starts with /, but not //)
		if baseURL.Scheme != "" && baseURL.Host != "" {
			if _, err := url.Parse(href); err == nil {
				userinfo := ""
				if baseURL.User != nil {
					userinfo = baseURL.User.String() + "@"
				}
				return baseURL.Scheme + "://" + userinfo + baseURL.Host + href
			}
		}
	}

	// Fallback for complex relative paths (e.g., ../path, ./path, ?query, #frag)
	// or when fast paths fail validation.
	linkURL, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(linkURL).String()
}

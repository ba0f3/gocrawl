package utils

import (
	"net/url"
	"strings"
)

// ResolveHref performs an optimized, zero-allocation fast-path resolution
// for common absolute and root-relative URLs to bypass url.Parse overhead.
func ResolveHref(baseURL *url.URL, href string) string {
	if href == "" || baseURL == nil {
		return ""
	}

	// Fast path: absolute URL
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	// Fast path: root-relative URL (starts with /, but not //)
	if href[0] == '/' && (len(href) == 1 || href[1] != '/') {
		return baseURL.Scheme + "://" + baseURL.Host + href
	}

	// Fallback for complex relative paths (e.g., ../path, ./path, ?query, #frag)
	linkURL, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(linkURL).String()
}

package utils

import (
	"net/url"
	"strings"
)

// ResolveHref performs an optimized, low-allocation fast-path resolution
// for common absolute and root-relative URLs. It avoids url.Parse allocations
// for absolute URLs (which have negligible cost) and avoids ResolveReference
// overhead for root-relative URLs (where concatenation still allocates the output string).
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
				return baseURL.Scheme + "://" + baseURL.Host + href
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

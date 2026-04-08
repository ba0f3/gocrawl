package crawler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/utils"
)

// TransportForCrawler returns the HTTP transport for Colly: Chrome-like TLS (uTLS) when enabled,
// otherwise a clone of the default transport, optionally wrapped with retry logic.
// Returns SafeTransport wrapped with retry if needed.
func TransportForCrawler(cfg *config.Config) http.RoundTripper {
	if cfg == nil {
		return utils.SafeTransport()
	}
	var base http.RoundTripper
	if cfg.Crawler.EnableChromeTLS {
		// newChromeHTTPTransport needs to inherit SafeTransport's DialContext
		t := newChromeHTTPTransport()
		if safe, ok := utils.SafeTransport().(*http.Transport); ok {
			t.DialContext = safe.DialContext
			t.Proxy = safe.Proxy
		}
		base = t
	} else {
		base = utils.SafeTransport()
	}
	if cfg.Crawler.CrawlMaxRetries > 0 {
		return &retryTransport{
			base: base,
			max:  cfg.Crawler.CrawlMaxRetries,
			baseDelay: func() time.Duration {
				d := cfg.Crawler.CrawlRetryBaseDelay
				if d <= 0 {
					return time.Second
				}
				return d
			}(),
		}
	}
	return base
}

// NewRetryTransport returns TransportForCrawler for backward compatibility.
func NewRetryTransport(cfg *config.Config) http.RoundTripper {
	return TransportForCrawler(cfg)
}

type retryTransport struct {
	base      http.RoundTripper
	max       int
	baseDelay time.Duration
}

func (rt *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastResp *http.Response
	var lastErr error
	for attempt := 0; attempt <= rt.max; attempt++ {
		resp, err := rt.base.RoundTrip(req)
		if err != nil {
			lastErr = err
			if attempt == rt.max {
				return nil, err
			}
			time.Sleep(rt.backoff(attempt, 0))
			continue
		}
		if resp.StatusCode != http.StatusTooManyRequests && (resp.StatusCode < 500 || resp.StatusCode > 599) {
			return resp, nil
		}
		lastResp = resp
		if attempt == rt.max {
			return resp, nil
		}
		wait := parseRetryAfter(resp.Header.Get("Retry-After"))
		if wait <= 0 {
			wait = rt.backoff(attempt, resp.StatusCode)
		}
		resp.Body.Close()
		time.Sleep(wait)
	}
	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}

func (rt *retryTransport) backoff(attempt int, status int) time.Duration {
	d := rt.baseDelay
	for i := 0; i < attempt; i++ {
		d *= 2
		if d > 30*time.Second {
			d = 30 * time.Second
			break
		}
	}
	return d
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if sec, err := strconv.Atoi(v); err == nil && sec >= 0 {
		return time.Duration(sec) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		now := time.Now()
		if t.After(now) {
			return time.Until(t)
		}
	}
	return 0
}

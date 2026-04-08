package crawler

import (
	"testing"
	"time"

	"gocrawl/internal/config"
)

func TestTransportForCrawler_ChromeTLS(t *testing.T) {
	cfg := &config.Config{Crawler: config.CrawlerConfig{EnableChromeTLS: true, CrawlMaxRetries: 0}}
	rt := TransportForCrawler(cfg)
	if rt == nil {
		t.Fatal("expected transport when Chrome TLS enabled")
	}
}

func TestTransportForCrawler_DefaultNil(t *testing.T) {
	cfg := &config.Config{Crawler: config.CrawlerConfig{CrawlMaxRetries: 0, EnableChromeTLS: false}}
	if TransportForCrawler(cfg) == nil {
		t.Fatal("expected SafeTransport when no chrome TLS and no retries")
	}
}

func TestTransportForCrawler_RetryOnly(t *testing.T) {
	cfg := &config.Config{Crawler: config.CrawlerConfig{CrawlMaxRetries: 2, CrawlRetryBaseDelay: 0}}
	rt := TransportForCrawler(cfg)
	r, ok := rt.(*retryTransport)
	if !ok {
		t.Fatalf("expected *retryTransport, got %T", rt)
	}
	if r.max != 2 {
		t.Fatalf("expected max retries 2, got %d", r.max)
	}
	if r.baseDelay != time.Second {
		t.Fatalf("expected default base delay %s, got %s", time.Second, r.baseDelay)
	}
}

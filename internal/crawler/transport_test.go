package crawler

import (
	"testing"

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
	if TransportForCrawler(cfg) != nil {
		t.Fatal("expected nil transport when no chrome TLS and no retries")
	}
}

func TestTransportForCrawler_RetryOnly(t *testing.T) {
	cfg := &config.Config{Crawler: config.CrawlerConfig{CrawlMaxRetries: 2, CrawlRetryBaseDelay: 0}}
	rt := TransportForCrawler(cfg)
	if rt == nil {
		t.Fatal("expected retry transport")
	}
}

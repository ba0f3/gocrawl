package crawler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/utils"
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
	rt := TransportForCrawler(cfg)
	if rt == nil {
		t.Fatal("expected SafeTransport when no chrome TLS and no retries")
	}

	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", rt)
	}

	if transport.DialContext == nil {
		t.Fatal("expected DialContext to be set")
	}

	_, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:80")
	if err == nil {
		t.Fatal("expected DialContext to block connection to 127.0.0.1, but it succeeded")
	}

	if !errors.Is(err, utils.ErrBlockedConnection) {
		t.Fatalf("expected error to be ErrBlockedConnection, got %v", err)
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

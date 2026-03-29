package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestGormStoreClaimNextQueuedJob(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "claimtest.db")
	s, err := NewGormStore("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.InitSchema(); err != nil {
		t.Fatal(err)
	}
	j := &CrawlJob{
		ID:          "job-1",
		URL:         "https://example.com",
		Status:      "queued",
		RequestJSON: `{"url":"https://example.com"}`,
		ExpiresAt:   time.Now().Add(time.Hour),
		UserID:      "u1",
	}
	if err := s.CreateCrawlJob(j); err != nil {
		t.Fatal(err)
	}
	claimed, err := s.ClaimNextQueuedJob()
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil {
		t.Fatal("expected claimed job")
	}
	if claimed.Status != "crawling" {
		t.Fatalf("status = %q want crawling", claimed.Status)
	}
	if claimed.RequestJSON == "" {
		t.Fatal("request json lost")
	}
	next, err := s.ClaimNextQueuedJob()
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Fatalf("expected no second job, got %v", next.ID)
	}
}

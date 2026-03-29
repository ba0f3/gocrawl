package db

import (
	"testing"
	"time"
)

func TestMemoryStoreClaimAndResults(t *testing.T) {
	s := NewMemoryStore()
	j := &CrawlJob{
		ID:          "j1",
		URL:         "https://example.com",
		Status:      "queued",
		RequestJSON: `{"url":"https://example.com"}`,
		ExpiresAt:   time.Now().Add(time.Hour),
		UserID:      "anon",
	}
	if err := s.CreateCrawlJob(j); err != nil {
		t.Fatal(err)
	}
	claimed, err := s.ClaimNextQueuedJob()
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || claimed.Status != "crawling" {
		t.Fatalf("claim: %+v", claimed)
	}
	if err := s.CreateCrawlResult(&CrawlResult{JobID: "j1", URL: "https://example.com", Markdown: "x"}); err != nil {
		t.Fatal(err)
	}
	res, err := s.GetCrawlResults("j1")
	if err != nil || len(res) != 1 {
		t.Fatalf("results: %v err %v", res, err)
	}
}

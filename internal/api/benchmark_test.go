package api_test

import (
	"testing"

	"gocrawl/internal/db"
)

func BenchmarkUpdateCrawlStatus_Nplus1_Memory(b *testing.B) {
	store := db.NewMemoryStore()
	job := &db.CrawlJob{ID: "test-job", Status: "queued", Completed: 0}
	store.CreateCrawlJob(job)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j, _ := store.GetCrawlJob("test-job")
		j.Status = "crawling"
		j.Completed = i
		store.UpdateCrawlJob(j)
	}
}

func BenchmarkUpdateCrawlStatus_Nplus1_SQLite(b *testing.B) {
	store, _ := db.NewGormStore("sqlite", ":memory:")
	store.InitSchema()
	job := &db.CrawlJob{ID: "test-job", Status: "queued", Completed: 0}
	store.CreateCrawlJob(job)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j, _ := store.GetCrawlJob("test-job")
		j.Status = "crawling"
		j.Completed = i
		store.UpdateCrawlJob(j)
	}
}

func BenchmarkUpdateJobProgress_Memory(b *testing.B) {
	store := db.NewMemoryStore()
	job := &db.CrawlJob{ID: "test-job", Status: "queued", Completed: 0}
	store.CreateCrawlJob(job)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.UpdateJobProgress("test-job", "crawling", i)
	}
}

func BenchmarkUpdateJobProgress_SQLite(b *testing.B) {
	store, _ := db.NewGormStore("sqlite", ":memory:")
	store.InitSchema()
	job := &db.CrawlJob{ID: "test-job", Status: "queued", Completed: 0}
	store.CreateCrawlJob(job)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.UpdateJobProgress("test-job", "crawling", i)
	}
}

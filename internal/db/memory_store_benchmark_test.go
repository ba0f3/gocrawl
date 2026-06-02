package db

import (
	"strconv"
	"testing"
	"time"
)

func BenchmarkMemoryStore_ClaimNextQueuedJob(b *testing.B) {
	store := NewMemoryStore()

	// Pre-fill with a mix of queued and completed jobs
	for i := 0; i < 10000; i++ {
		status := "queued"
		if i%2 == 0 {
			status = "completed"
		}

		store.CreateCrawlJob(&CrawlJob{
			ID:        strconv.Itoa(i),
			Status:    status,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.ClaimNextQueuedJob()

		// If it runs out of queued jobs, add one back so we don't just return nil
		if i%4000 == 0 && i > 0 {
            b.StopTimer()
            for j := 0; j < 1000; j++ {
                store.CreateCrawlJob(&CrawlJob{
                    ID:        "new" + strconv.Itoa(i) + "_" + strconv.Itoa(j),
                    Status:    "queued",
                    CreatedAt: time.Now(),
                })
            }
            b.StartTimer()
		}
	}
}

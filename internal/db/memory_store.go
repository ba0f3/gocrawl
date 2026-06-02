package db

import (
	"errors"
	"fmt"
	"sync"

	"gocrawl/internal/config"

	"github.com/google/uuid"
)

// ErrJobNotFound is returned by GetCrawlJob when the ID does not exist.
var ErrJobNotFound = errors.New("job not found")

// MemoryStore holds users unsupported (auth off) but implements crawl jobs and results in RAM.
type MemoryStore struct {
	mu     sync.Mutex
	jobs   map[string]*CrawlJob
	byJob  map[string][]*CrawlResult
	queued []string
}

// NewMemoryStore is used when ENABLE_AUTH=false so crawl queue and status work without Mongo/SQL.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs:   make(map[string]*CrawlJob),
		byJob:  make(map[string][]*CrawlResult),
		queued: make([]string, 0),
	}
}

func (m *MemoryStore) InitSchema() error { return nil }

func (m *MemoryStore) Close() error { return nil }

func (m *MemoryStore) StartCleanupRoutine(_ config.RetentionConfig) {}

func (m *MemoryStore) CreateUser(*User) error {
	return fmt.Errorf("database is not available")
}

func (m *MemoryStore) GetUserByUsername(string) (*User, error) {
	return nil, fmt.Errorf("database is not available")
}

func (m *MemoryStore) GetUserByAPIKey(string) (*User, error) {
	return nil, fmt.Errorf("database is not available")
}

func (m *MemoryStore) CreateCrawlJob(job *CrawlJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cpy := *job
	m.jobs[job.ID] = &cpy
	if cpy.Status == "queued" {
		m.queued = append(m.queued, cpy.ID)
	}
	return nil
}

func (m *MemoryStore) GetCrawlJob(id string) (*CrawlJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	cpy := *j
	return &cpy, nil
}

func (m *MemoryStore) UpdateCrawlJob(job *CrawlJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldJob, ok := m.jobs[job.ID]
	if !ok {
		return ErrJobNotFound
	}

	cpy := *job
	m.jobs[job.ID] = &cpy

	if oldJob.Status != "queued" && cpy.Status == "queued" {
		m.queued = append(m.queued, cpy.ID)
	}
	return nil
}

func (m *MemoryStore) UpdateJobProgress(jobID string, status string, completed int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}

	oldStatus := j.Status
	j.Status = status
	j.Completed = completed

	if oldStatus != "queued" && status == "queued" {
		m.queued = append(m.queued, jobID)
	}
	return nil
}

func (m *MemoryStore) ClaimNextQueuedJob() (*CrawlJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// ⚡ Bolt Optimization: O(1) queue for fast job claiming instead of O(N) map traversal
	for len(m.queued) > 0 {
		id := m.queued[0]
		m.queued = m.queued[1:]

		if j, ok := m.jobs[id]; ok && j.Status == "queued" {
			j.Status = "crawling"
			cpy := *j
			return &cpy, nil
		}
	}

	return nil, nil
}

func (m *MemoryStore) CreateCrawlResult(result *CrawlResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cpy := *result
	if cpy.ID == "" {
		cpy.ID = uuid.New().String()
	}
	m.byJob[cpy.JobID] = append(m.byJob[cpy.JobID], &cpy)
	return nil
}

func (m *MemoryStore) CreateCrawlResults(results []*CrawlResult) error {
	if len(results) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, result := range results {
		cpy := *result
		if cpy.ID == "" {
			cpy.ID = uuid.New().String()
		}
		m.byJob[cpy.JobID] = append(m.byJob[cpy.JobID], &cpy)
	}
	return nil
}

func (m *MemoryStore) GetCrawlResults(jobID string) ([]*CrawlResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	src := m.byJob[jobID]
	out := make([]*CrawlResult, len(src))
	for i := range src {
		c := *src[i]
		out[i] = &c
	}
	return out, nil
}

func (m *MemoryStore) JobCountsByStatus() (queued, crawling, completed, failed int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		switch j.Status {
		case "queued":
			queued++
		case "crawling":
			crawling++
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}
	return queued, crawling, completed, failed, nil
}

package api

import (
	"log"
	"sync"

	"gocrawl/internal/config"
	"gocrawl/internal/crawler"
	"gocrawl/internal/db"
)

// CrawlManager manages crawl jobs
type CrawlManager struct {
	db  *db.Database
	cfg *config.Config
	sync.Mutex
}

// NewCrawlManager creates a new crawl manager
func NewCrawlManager(db *db.Database, cfg *config.Config) *CrawlManager {
	return &CrawlManager{
		db:  db,
		cfg: cfg,
	}
}

// StartCrawl starts an asynchronous crawl job
func (cm *CrawlManager) StartCrawl(jobID string, req *CrawlRequestBody) {
	cm.Lock()
	defer cm.Unlock()

	log.Printf("Starting crawl for job ID: %s", jobID)

	// Update status to running
	cm.updateCrawlStatus(jobID, "crawling", 0)

	// For now, just crawl the single URL provided
	// TODO: Implement multi-page crawling with depth, link following, etc.
	crawlReq := req.ScrapeOptions
	if crawlReq == nil {
		crawlReq = &crawler.CrawlRequest{
			URL:             req.URL,
			OnlyMainContent: true,
			Formats:         []string{"markdown", "html", "rawHtml"},
			Timeout:         30,
		}
	} else if crawlReq.URL == "" {
		crawlReq.URL = req.URL
	}

	// Perform crawl for the main URL
	result, err := crawler.CrawlURL(crawlReq, cm.cfg)
	if err != nil {
		log.Printf("Error performing crawl: %v", err)
		cm.updateCrawlStatus(jobID, "failed", 0)
		return
	}

	results := []*crawler.CrawlResult{result}

	// Save results
	for _, result := range results {
		cr := db.CrawlResult{
			JobID:    jobID,
			URL:      result.Metadata["sourceURL"],
			Markdown: result.Markdown,
			HTML:     result.HTML,
			RawHTML:  result.RawHTML,
			Links:    result.Links,
			Metadata: result.Metadata,
		}
		if err := cm.db.CreateCrawlResult(&cr); err != nil {
			log.Printf("Error saving crawl result: %v", err)
		}
	}

	// Update job status
	cm.updateCrawlStatus(jobID, "completed", len(results))

	log.Printf("Crawl completed for job ID: %s", jobID)
}

// updateCrawlStatus updates the status of a crawl job
func (cm *CrawlManager) updateCrawlStatus(jobID string, status string, completed int) {
	job, err := cm.db.GetCrawlJob(jobID)
	if err != nil {
		log.Printf("Error retrieving crawl job for status update: %v", err)
		return
	}

	job.Status = status
	job.Completed = completed
	if err := cm.db.UpdateCrawlJob(job); err != nil {
		log.Printf("Error updating crawl job status: %v", err)
	}
}


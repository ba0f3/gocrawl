package api

import (
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/crawler"
	"gocrawl/internal/db"

	"github.com/gocolly/colly/v2"
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

	// Perform multi-page crawl
	results := cm.performCrawling(req, jobID)

	// Update total count
	cm.updateCrawlTotal(jobID, len(results))

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

// updateCrawlTotal updates the total count for a crawl job
func (cm *CrawlManager) updateCrawlTotal(jobID string, total int) {
	job, err := cm.db.GetCrawlJob(jobID)
	if err != nil {
		log.Printf("Error retrieving crawl job for total update: %v", err)
		return
	}

	job.Total = total
	if err := cm.db.UpdateCrawlJob(job); err != nil {
		log.Printf("Error updating crawl job total: %v", err)
	}
}

// performCrawling performs a multi-page scrape based on the request
func (cm *CrawlManager) performCrawling(req *CrawlRequestBody, jobID string) []*crawler.ScrapeResult {
	results := make([]*crawler.ScrapeResult, 0)
	visited := make(map[string]bool)
	completedCount := 0

	// Parse base URL
	baseURL, err := url.Parse(req.URL)
	if err != nil {
		log.Printf("Error parsing base URL: %v", err)
		return results
	}

	// Create colly collector
	c := colly.NewCollector(
		colly.MaxDepth(req.MaxDepth),
		colly.Async(true),
	)

	// Set concurrency
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: req.MaxConcurrency,
		Delay:       time.Duration(req.Delay) * time.Millisecond,
	})

	// Configure based on request options
	if !req.AllowExternalLinks {
		c.AllowedDomains = append(c.AllowedDomains, baseURL.Host)
	}

	// Handle found links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absURL := e.Request.AbsoluteURL(link)

		// Apply filters
		if cm.shouldScrapeURL(absURL, baseURL, req) && !visited[absURL] {
			visited[absURL] = true
			if len(visited) <= req.Limit {
				e.Request.Visit(absURL)
			}
		}
	})

	// Process each page
	c.OnScraped(func(r *colly.Response) {
		if completedCount >= req.Limit {
			return
		}

		// Create scrape request for this page
		scrapeReq := req.ScrapeOptions
		if scrapeReq == nil {
			scrapeReq = &crawler.ScrapeRequest{
				OnlyMainContent: true,
				Formats:         []string{"markdown", "html", "rawHtml"},
			}
		}
		scrapeReq.URL = r.Request.URL.String()

		// Scrape the page
		result, err := crawler.ScrapeURL(scrapeReq, cm.cfg)
		if err != nil {
			log.Printf("Error scraping page %s: %v", r.Request.URL, err)
			return
		}

		results = append(results, result)
		completedCount++

		// Update progress periodically
		if completedCount%10 == 0 {
			cm.updateCrawlStatus(jobID, "crawling", completedCount)
		}
	})

	// Start scraping
	c.Visit(req.URL)
	c.Wait()

	return results
}

// shouldScrapeURL determines if a URL should be scraped based on filters
func (cm *CrawlManager) shouldScrapeURL(absURL string, baseURL *url.URL, req *CrawlRequestBody) bool {
	parsedURL, err := url.Parse(absURL)
	if err != nil {
		return false
	}

	// Check if external link
	if !req.AllowExternalLinks && parsedURL.Host != baseURL.Host {
		return false
	}

	// Check subdomain
	if !req.AllowSubdomains && parsedURL.Host != baseURL.Host {
		return false
	}

	// Check include paths
	if len(req.IncludePaths) > 0 {
		included := false
		for _, path := range req.IncludePaths {
			if strings.Contains(parsedURL.Path, path) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	// Check exclude paths
	for _, path := range req.ExcludePaths {
		if strings.Contains(parsedURL.Path, path) {
			return false
		}
	}

	return true
}

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/crawler"
	"gocrawl/internal/db"
	"gocrawl/internal/utils"

	"github.com/gocolly/colly/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer represents the MCP server instance
type MCPServer struct {
	server    *mcp.Server
	db        db.Store
	cfg       *config.Config
	crawlMu   sync.RWMutex
	crawlJobs map[string]*CrawlJob
}

// CrawlJob represents a crawling job
type CrawlJob struct {
	mu        sync.RWMutex
	ID        string
	URL       string
	Status    string
	Progress  int
	Total     int
	StartTime time.Time
	EndTime   *time.Time
	Error     string
}

// ScrapeArgs represents the arguments for the scrape tool
type ScrapeArgs struct {
	URL             string   `json:"url" mcp:"URL to scrape"`
	OnlyMainContent *bool    `json:"onlyMainContent,omitempty" mcp:"Extract only main content (omit for default)"`
	Formats         []string `json:"formats" mcp:"Output formats (markdown, html, rawHtml)"`
	Timeout         int      `json:"timeout" mcp:"Timeout in seconds"`
}

// CrawlArgs represents the arguments for the crawl tool
type CrawlArgs struct {
	URL      string `json:"url" mcp:"Starting URL for crawl"`
	MaxDepth int    `json:"maxDepth" mcp:"Maximum crawl depth"`
	MaxPages int    `json:"maxPages" mcp:"Maximum pages to crawl"`
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(database db.Store, cfg *config.Config) (*MCPServer, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "gocrawl-mcp",
		Version: "1.0.0",
	}, nil)

	s := &MCPServer{
		server:    server,
		db:        database,
		cfg:       cfg,
		crawlJobs: make(map[string]*CrawlJob),
	}

	// Register tools
	s.registerTools()

	return s, nil
}

// registerTools registers all MCP tools
func (s *MCPServer) registerTools() {
	// Scrape tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "scrape",
		Description: "Scrape a single web page and extract content",
	}, s.handleScrape)

	// Crawl tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "crawl",
		Description: "Start a crawl job for multiple pages",
	}, s.handleCrawl)

	// Stats tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "stats",
		Description: "Get crawl queue statistics",
	}, s.handleStats)
}

// handleScrape handles the scrape tool
func (s *MCPServer) handleScrape(ctx context.Context, _ *mcp.CallToolRequest, args ScrapeArgs) (*mcp.CallToolResult, any, error) {
	// Convert parameters to CrawlRequest
	req := &crawler.ScrapeRequest{
		URL:             args.URL,
		OnlyMainContent: args.OnlyMainContent,
		Formats:         args.Formats,
		Timeout:         args.Timeout,
	}

	// Set defaults
	if len(req.Formats) == 0 {
		req.Formats = []string{"markdown"}
	}
	if req.Timeout == 0 {
		req.Timeout = 30
	}

	// Perform the scrape
	result, err := crawler.ScrapeURLWithContext(ctx, req, s.cfg)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Error scraping URL: %v", err),
				},
			},
			IsError: true,
		}, nil, nil
	}

	// Return the result
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(resultJSON),
			},
		},
	}, nil, nil
}

// handleCrawl handles the crawl tool
func (s *MCPServer) handleCrawl(ctx context.Context, _ *mcp.CallToolRequest, args CrawlArgs) (*mcp.CallToolResult, any, error) {
	// Set defaults
	maxDepth := args.MaxDepth
	if maxDepth == 0 {
		maxDepth = 2
	}

	maxPages := args.MaxPages
	if maxPages == 0 {
		maxPages = 10
	}

	// Create a new crawl job
	jobID := fmt.Sprintf("crawl_%d", time.Now().Unix())
	job := &CrawlJob{
		ID:        jobID,
		URL:       args.URL,
		Status:    "pending",
		Progress:  0,
		Total:     maxPages,
		StartTime: time.Now(),
	}

	s.crawlMu.Lock()
	s.crawlJobs[jobID] = job
	s.crawlMu.Unlock()

	// Start the crawl in a goroutine
	go s.performCrawl(ctx, job, maxDepth, maxPages)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Crawl job started with ID: %s", jobID),
			},
		},
	}, nil, nil
}

// performCrawl performs the actual crawling
func (s *MCPServer) performCrawl(ctx context.Context, job *CrawlJob, maxDepth, maxPages int) {
	job.mu.Lock()
	job.Status = "running"
	job.mu.Unlock()

	baseURL, err := url.Parse(job.URL)
	if err != nil {
		job.mu.Lock()
		job.Status = "failed"
		job.Error = fmt.Sprintf("invalid URL: %v", err)
		now := time.Now()
		job.EndTime = &now
		job.mu.Unlock()
		return
	}

	c := colly.NewCollector(
		colly.MaxDepth(maxDepth),
		colly.Async(true),
	)

	// Restrict to the same domain
	c.AllowedDomains = []string{baseURL.Host}

	var crawlMu sync.Mutex
	visited := make(map[string]bool)

	// ⚡ Bolt Optimization: Hoist expected origin calculation out of the OnHTML hot loop
	expectedOrigin := baseURL.Scheme + "://" + baseURL.Host

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		link := e.Attr("href")
		absURL := utils.ResolveHref(e.Request.URL, link)

		// ⚡ Bolt Optimization: Pre-check visited map and page limits before parsing new URL
		crawlMu.Lock()
		if visited[absURL] || len(visited) >= maxPages {
			crawlMu.Unlock()
			return
		}
		crawlMu.Unlock()

		// ⚡ Bolt Optimization: Safe low-allocation fast-path for domain validation.
		// Avoids instantiating `url.URL` structs for the thousands of links found on pages.
		isAllowedOrigin := false
		if strings.HasPrefix(absURL, expectedOrigin) {
			// Ensure we are not matching a sub-string of a different domain (e.g., example.com.org)
			if len(absURL) == len(expectedOrigin) || absURL[len(expectedOrigin)] == '/' || absURL[len(expectedOrigin)] == '?' || absURL[len(expectedOrigin)] == '#' {
				isAllowedOrigin = true
			}
		}

		if !isAllowedOrigin {
			// Fallback for complex URLs (auth, unusual formatting)
			parsedURL, err := url.Parse(absURL)
			if err != nil || parsedURL.Host != baseURL.Host {
				return
			}
		}

		crawlMu.Lock()
		if !visited[absURL] {
			visited[absURL] = true
			crawlMu.Unlock()
			_ = e.Request.Visit(absURL)
		} else {
			crawlMu.Unlock()
		}
	})

	c.OnScraped(func(r *colly.Response) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		scrapeOpts := crawler.ScrapeRequest{
			URL:            r.Request.URL.String(),
			Formats:        []string{"markdown", "html", "rawHtml"},
			PreFetchedBody: r.Body,
		}
		if r.Headers != nil {
			scrapeOpts.PreFetchedHeaders = r.Headers.Clone()
		}

		result, err := crawler.ScrapeURLWithContext(ctx, &scrapeOpts, s.cfg)
		if err != nil {
			log.Printf("Error scraping page %s: %v", utils.SanitizeForLog(r.Request.URL.String()), utils.SanitizeForLog(err.Error()))
			return
		}

		log.Printf("Successfully scraped %s: %d chars markdown, %d links",
			utils.SanitizeForLog(r.Request.URL.String()), len(result.Markdown), len(result.Links))

		crawlMu.Lock()
		job.mu.Lock()
		job.Progress++
		progress := job.Progress
		job.mu.Unlock()
		crawlMu.Unlock()

		log.Printf("Crawl progress: %d/%d pages (visited %s)", progress, job.Total, utils.SanitizeForLog(r.Request.URL.String()))
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Error visiting %s: %v", utils.SanitizeForLog(r.Request.URL.String()), utils.SanitizeForLog(err.Error()))
	})

	// Add transport if configured
	if s.cfg != nil {
		if t := crawler.TransportForCrawler(s.cfg); t != nil {
			c.WithTransport(t)
		}
	}

	crawlMu.Lock()
	visited[job.URL] = true
	crawlMu.Unlock()

	if err := c.Visit(job.URL); err != nil {
		job.mu.Lock()
		job.Status = "failed"
		job.Error = fmt.Sprintf("failed to visit initial URL: %v", err)
		now := time.Now()
		job.EndTime = &now
		job.mu.Unlock()
		return
	}

	// Wait for completion or context cancellation
	done := make(chan struct{})
	go func() {
		c.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		job.mu.Lock()
		job.Status = "failed"
		job.Error = "crawl canceled"
		job.mu.Unlock()
	case <-done:
		job.mu.Lock()
		job.Status = "completed"
		job.mu.Unlock()
	}

	now := time.Now()
	job.mu.Lock()
	job.EndTime = &now
	job.mu.Unlock()
}

// StatsArgs represents empty arguments for the stats tool
type StatsArgs struct{}

// handleStats handles the stats tool
func (s *MCPServer) handleStats(ctx context.Context, _ *mcp.CallToolRequest, _ StatsArgs) (*mcp.CallToolResult, any, error) {
	stats := map[string]interface{}{
		"activeJobs":    0,
		"pendingJobs":   0,
		"completedJobs": 0,
		"failedJobs":    0,
		"jobs":          []map[string]interface{}{},
	}
	if s.db != nil {
		q, cr, co, f, err := s.db.JobCountsByStatus()
		if err == nil {
			stats["pendingJobs"] = q
			stats["activeJobs"] = cr
			stats["completedJobs"] = co
			stats["failedJobs"] = f
		}
	}

	s.crawlMu.RLock()
	defer s.crawlMu.RUnlock()
	for _, job := range s.crawlJobs {
		job.mu.RLock()
		jobInfo := map[string]interface{}{
			"id":       job.ID,
			"url":      job.URL,
			"status":   job.Status,
			"progress": job.Progress,
			"total":    job.Total,
			"started":  job.StartTime.Format(time.RFC3339),
		}
		if job.EndTime != nil {
			jobInfo["ended"] = job.EndTime.Format(time.RFC3339)
		}
		if job.Error != "" {
			jobInfo["error"] = job.Error
		}

		job.mu.RUnlock()
		stats["jobs"] = append(stats["jobs"].([]map[string]interface{}), jobInfo)
	}

	stats["serverTime"] = time.Now().Format(time.RFC3339)

	// Return the stats
	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(statsJSON),
			},
		},
	}, nil, nil
}

// Start starts the MCP server using the provided transport
func (s *MCPServer) Start(ctx context.Context, transport mcp.Transport) error {
	return s.server.Run(ctx, transport)
}

// GetServer returns the underlying MCP server
func (s *MCPServer) GetServer() *mcp.Server {
	return s.server
}

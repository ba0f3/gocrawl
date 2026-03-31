package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/crawler"
	"gocrawl/internal/db"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer represents the MCP server instance
type MCPServer struct {
	server    *mcp.Server
	db        db.Store
	cfg       *config.Config
	crawlJobs map[string]*CrawlJob
}

// CrawlJob represents a crawling job
type CrawlJob struct {
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
	s.crawlJobs[jobID] = job

	// Start the crawl in a goroutine
	go s.performCrawl(job, maxDepth, maxPages)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Crawl job started with ID: %s", jobID),
			},
		},
	}, nil, nil
}

// performCrawl performs the actual crawling
func (s *MCPServer) performCrawl(job *CrawlJob, maxDepth, maxPages int) {
	job.Status = "running"

	// TODO: Implement actual crawling logic
	// For now, we'll simulate progress
	for i := 0; i < maxPages; i++ {
		time.Sleep(1 * time.Second)
		job.Progress = i + 1

		// Simulate sending SSE update
		log.Printf("Crawl progress: %d/%d pages", job.Progress, job.Total)
	}

	job.Status = "completed"
	now := time.Now()
	job.EndTime = &now
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

	for _, job := range s.crawlJobs {
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

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

	"github.com/modelcontextprotocol/go-sdk/pkg/server"
	"github.com/modelcontextprotocol/go-sdk/pkg/types"
)

// MCPServer represents the MCP server instance
type MCPServer struct {
	server   *server.MCPServer
	db       *db.Database
	cfg      *config.Config
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

// NewMCPServer creates a new MCP server instance
func NewMCPServer(database *db.Database, cfg *config.Config) (*MCPServer, error) {
	mcpServer := server.NewMCPServer(
		"gocrawl-mcp",
		"1.0.0",
		server.WithLogging(),
	)

	s := &MCPServer{
		server:    mcpServer,
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
	s.server.AddTool(
		"scrape",
		"Scrape a single web page and extract content",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "URL to scrape",
				},
				"onlyMainContent": map[string]interface{}{
					"type":        "boolean",
					"description": "Extract only main content",
					"default":     true,
				},
				"formats": map[string]interface{}{
					"type":        "array",
					"description": "Output formats (markdown, html, rawHtml)",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"markdown", "html", "rawHtml"},
					},
					"default": []string{"markdown"},
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in seconds",
					"default":     30,
				},
			},
			"required": []string{"url"},
		},
		s.handleScrape,
	)

	// Crawl tool
	s.server.AddTool(
		"crawl",
		"Start a crawl job for multiple pages",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Starting URL for crawl",
				},
				"maxDepth": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum crawl depth",
					"default":     2,
				},
				"maxPages": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum pages to crawl",
					"default":     10,
				},
			},
			"required": []string{"url"},
		},
		s.handleCrawl,
	)

	// Stats tool
	s.server.AddTool(
		"stats",
		"Get crawl queue statistics",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		s.handleStats,
	)
}

// handleScrape handles the scrape tool
func (s *MCPServer) handleScrape(ctx context.Context, params map[string]interface{}) (*types.ToolResult, error) {
	url, ok := params["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter is required")
	}

	// Convert parameters to CrawlRequest
	req := &crawler.CrawlRequest{
		URL: url,
	}

	if onlyMain, ok := params["onlyMainContent"].(bool); ok {
		req.OnlyMainContent = onlyMain
	}

	if formats, ok := params["formats"].([]interface{}); ok {
		req.Formats = make([]string, len(formats))
		for i, f := range formats {
			req.Formats[i] = f.(string)
		}
	} else {
		req.Formats = []string{"markdown"}
	}

	if timeout, ok := params["timeout"].(float64); ok {
		req.Timeout = int(timeout)
	}

	// Perform the scrape
	result, err := crawler.CrawlURL(req, s.cfg)
	if err != nil {
		return &types.ToolResult{
			Content: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": fmt.Sprintf("Error scraping URL: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Return the result
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}

	return &types.ToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": string(resultJSON),
			},
		},
	}, nil
}

// handleCrawl handles the crawl tool
func (s *MCPServer) handleCrawl(ctx context.Context, params map[string]interface{}) (*types.ToolResult, error) {
	url, ok := params["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter is required")
	}

	maxDepth := 2
	if depth, ok := params["maxDepth"].(float64); ok {
		maxDepth = int(depth)
	}

	maxPages := 10
	if pages, ok := params["maxPages"].(float64); ok {
		maxPages = int(pages)
	}

	// Create a new crawl job
	jobID := fmt.Sprintf("crawl_%d", time.Now().Unix())
	job := &CrawlJob{
		ID:        jobID,
		URL:       url,
		Status:    "pending",
		Progress:  0,
		Total:     maxPages,
		StartTime: time.Now(),
	}
	s.crawlJobs[jobID] = job

	// Start the crawl in a goroutine
	go s.performCrawl(job, maxDepth, maxPages)

	return &types.ToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Crawl job started with ID: %s", jobID),
			},
		},
	}, nil
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

// handleStats handles the stats tool
func (s *MCPServer) handleStats(ctx context.Context, params map[string]interface{}) (*types.ToolResult, error) {
	stats := map[string]interface{}{
		"activeJobs":    0,
		"pendingJobs":   0,
		"completedJobs": 0,
		"failedJobs":    0,
		"jobs":          []map[string]interface{}{},
	}

	// Count jobs by status
	for _, job := range s.crawlJobs {
		switch job.Status {
		case "running":
			stats["activeJobs"] = stats["activeJobs"].(int) + 1
		case "pending":
			stats["pendingJobs"] = stats["pendingJobs"].(int) + 1
		case "completed":
			stats["completedJobs"] = stats["completedJobs"].(int) + 1
		case "failed":
			stats["failedJobs"] = stats["failedJobs"].(int) + 1
		}

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
		return nil, err
	}

	return &types.ToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": string(statsJSON),
			},
		},
	}, nil
}

// Start starts the MCP server
func (s *MCPServer) Start() error {
	return s.server.Start()
}

// GetServer returns the underlying MCP server
func (s *MCPServer) GetServer() *server.MCPServer {
	return s.server
}

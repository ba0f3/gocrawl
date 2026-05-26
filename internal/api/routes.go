package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/crawler"
	"gocrawl/internal/db"
	"gocrawl/internal/mcp"
	"gocrawl/internal/user"
	"gocrawl/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	DB           db.Store
	Cfg          *config.Config
	crawlManager *CrawlManager
	SSEManager   *mcp.SSEManager
	MCPServer    *mcp.MCPServer
}

func NewHandler(store db.Store, cfg *config.Config) *Handler {
	sseManager := mcp.NewSSEManager()
	mcpServer, _ := mcp.NewMCPServer(store, cfg)

	return &Handler{
		DB:           store,
		Cfg:          cfg,
		crawlManager: NewCrawlManager(store, cfg),
		SSEManager:   sseManager,
		MCPServer:    mcpServer,
	}
}

// StartCrawlWorkers starts the configured number of background crawl workers.
func (h *Handler) StartCrawlWorkers() {
	h.crawlManager.StartWorkers(h.Cfg.Crawler.CrawlWorkers)
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func writeJSON(c *gin.Context, statusCode int, data interface{}, err error) {
	resp := ApiResponse{}
	if err != nil {
		resp.Success = false
		if statusCode >= http.StatusInternalServerError {
			resp.Error = "An internal server error occurred"
		} else {
			resp.Error = err.Error()
		}
	} else {
		resp.Success = true
		resp.Data = data
	}
	c.JSON(statusCode, resp)
}

func (h *Handler) Register(c *gin.Context) {
	var creds struct {
		Username string `json:"username" binding:"required,max=64"`
		Password string `json:"password" binding:"required,max=72"`
	}
	if err := c.ShouldBindJSON(&creds); err != nil {
		writeJSON(c, http.StatusBadRequest, nil, err)
		return
	}

	u, err := user.Register(h.DB, creds.Username, creds.Password)
	if err != nil {
		writeJSON(c, http.StatusInternalServerError, nil, err)
		return
	}

	writeJSON(c, http.StatusCreated, u, nil)
}

func (h *Handler) Login(c *gin.Context) {
	var creds struct {
		Username string `json:"username" binding:"required,max=64"`
		Password string `json:"password" binding:"required,max=72"`
	}
	if err := c.ShouldBindJSON(&creds); err != nil {
		writeJSON(c, http.StatusBadRequest, nil, err)
		return
	}

	u, err := user.Login(h.DB, creds.Username, creds.Password)
	if err != nil {
		writeJSON(c, http.StatusUnauthorized, nil, err)
		return
	}

	writeJSON(c, http.StatusOK, u, nil)
}

func (h *Handler) GenerateAPIKey(c *gin.Context) {
	// This should be protected and only accessible to logged-in users
	// For simplicity, we are not implementing full session management here
	writeJSON(c, http.StatusNotImplemented, nil, fmt.Errorf("not implemented"))
}

func (h *Handler) Scrape(c *gin.Context) {
	var req crawler.ScrapeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeJSON(c, http.StatusBadRequest, nil, err)
		return
	}

	result, err := crawler.ScrapeURLWithContext(c.Request.Context(), &req, h.Cfg)
	if err != nil {
		writeJSON(c, http.StatusInternalServerError, nil, err)
		return
	}

	writeJSON(c, http.StatusOK, result, nil)
}

// CrawlRequest represents the request body for /api/v1/crawl
type CrawlRequestBody struct {
	URL                string                 `json:"url"`
	ExcludePaths       []string               `json:"excludePaths,omitempty"`
	IncludePaths       []string               `json:"includePaths,omitempty"`
	MaxDepth           int                    `json:"maxDepth,omitempty"`
	MaxDiscoveryDepth  int                    `json:"maxDiscoveryDepth,omitempty"`
	IgnoreSitemap      bool                   `json:"ignoreSitemap,omitempty"`
	IgnoreQueryParams  bool                   `json:"ignoreQueryParameters,omitempty"`
	Limit              int                    `json:"limit,omitempty"`
	AllowBackwardLinks bool                   `json:"allowBackwardLinks,omitempty"`
	CrawlEntireDomain  bool                   `json:"crawlEntireDomain,omitempty"`
	AllowExternalLinks bool                   `json:"allowExternalLinks,omitempty"`
	AllowSubdomains    bool                   `json:"allowSubdomains,omitempty"`
	Delay              int                    `json:"delay,omitempty"`
	MaxConcurrency     int                    `json:"maxConcurrency,omitempty"`
	ScrapeOptions      *crawler.ScrapeRequest `json:"scrapeOptions,omitempty"`
	// LinkSelector is a single CSS selector for link discovery (alias for one entry in linkSelectors).
	LinkSelector string `json:"linkSelector,omitempty"`
	// LinkSelectors restricts which elements are used to discover outbound links (CSS, must match anchors). Empty uses built-in article/main + all links heuristics.
	LinkSelectors     []string `json:"linkSelectors,omitempty"`
	ZeroDataRetention bool     `json:"zeroDataRetention,omitempty"`
}

// CrawlResponse represents the response for /api/v1/crawl
type CrawlResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
	URL     string `json:"url"`
}

func (h *Handler) Crawl(c *gin.Context) {
	var userID string
	if user, ok := c.Request.Context().Value("user").(*db.User); ok && user != nil {
		userID = user.ID
	} else {
		if h.Cfg.Security.EnableAuth {
			writeJSON(c, http.StatusUnauthorized, nil, fmt.Errorf("unauthorized"))
			return
		}
		userID = uuid.New().String()
	}

	var req CrawlRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		writeJSON(c, http.StatusBadRequest, nil, err)
		return
	}

	if req.MaxDepth == 0 {
		req.MaxDepth = 10
	}
	if req.Limit == 0 {
		req.Limit = 10000
	}
	if req.MaxConcurrency == 0 {
		req.MaxConcurrency = 10
	}

	payload, err := json.Marshal(&req)
	if err != nil {
		writeJSON(c, http.StatusInternalServerError, nil, err)
		return
	}

	jobID := uuid.New().String()
	crawlJob := &db.CrawlJob{
		ID:          jobID,
		URL:         req.URL,
		Status:      "queued",
		Total:       0,
		Completed:   0,
		CreditsUsed: 0,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		UserID:      userID,
		CreatedAt:   time.Now(),
		RequestJSON: string(payload),
	}

	if err := h.DB.CreateCrawlJob(crawlJob); err != nil {
		writeJSON(c, http.StatusInternalServerError, nil, err)
		return
	}

	c.JSON(http.StatusOK, CrawlResponse{
		Success: true,
		ID:      jobID,
		URL:     req.URL,
	})
}

// CrawlStatusResponse represents the response for /api/v1/crawl/{id}
type CrawlStatusResponse struct {
	Status      string                 `json:"status"`
	Total       int                    `json:"total"`
	Completed   int                    `json:"completed"`
	CreditsUsed int                    `json:"creditsUsed"`
	ExpiresAt   string                 `json:"expiresAt"`
	Next        string                 `json:"next,omitempty"`
	Data        []crawler.ScrapeResult `json:"data,omitempty"`
}

func (h *Handler) GetCrawlStatus(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeJSON(c, http.StatusBadRequest, nil, fmt.Errorf("job ID is required"))
		return
	}

	job, err := h.DB.GetCrawlJob(jobID)
	if err != nil {
		writeJSON(c, http.StatusNotFound, nil, fmt.Errorf("job not found"))
		return
	}

	// Prevent IDOR: Verify the authenticated user owns this job
	if h.Cfg.Security.EnableAuth {
		if u, ok := c.Request.Context().Value("user").(*db.User); ok && u != nil {
			if job.UserID != u.ID {
				// Return 404 instead of 403 to prevent job ID enumeration
				writeJSON(c, http.StatusNotFound, nil, fmt.Errorf("job not found"))
				return
			}
		} else {
			writeJSON(c, http.StatusUnauthorized, nil, fmt.Errorf("unauthorized"))
			return
		}
	}

	results, err := h.DB.GetCrawlResults(jobID)
	if err != nil {
		writeJSON(c, http.StatusInternalServerError, nil, err)
		return
	}

	apiResults := make([]crawler.ScrapeResult, len(results))
	for i, result := range results {
		apiResults[i] = crawler.ScrapeResult{
			Markdown: result.Markdown,
			HTML:     result.HTML,
			RawHTML:  result.RawHTML,
			Links:    result.Links,
			Metadata: result.Metadata,
		}
	}

	c.JSON(http.StatusOK, CrawlStatusResponse{
		Status:      job.Status,
		Total:       job.Total,
		Completed:   job.Completed,
		CreditsUsed: job.CreditsUsed,
		ExpiresAt:   job.ExpiresAt.Format(time.RFC3339),
		Data:        apiResults,
	})
}

// SSEScrape streams scrape results over SSE (used by MCP when Accept: text/event-stream).
func (h *Handler) SSEScrape(c *gin.Context) {
	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "Streaming unsupported!")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sseClient := &mcp.SSEClient{
		ID:     "scrape_client",
		Events: make(chan mcp.SSEEvent, 10),
		Done:   make(chan bool),
	}

	h.SSEManager.AddClient(sseClient)
	defer h.SSEManager.RemoveClient(sseClient.ID)

	req := c.Request
	go func() {
		var scrapeReq crawler.ScrapeRequest
		limitedBody := io.LimitReader(req.Body, 1<<20) // 1MB limit to prevent DoS
		if err := json.NewDecoder(limitedBody).Decode(&scrapeReq); err != nil {
			log.Printf("Error decoding request: %v", utils.SanitizeForLog(err.Error()))
			sseClient.Events <- mcp.SSEEvent{Type: "error", Data: map[string]interface{}{"message": "Invalid request body"}}
			sseClient.Done <- true
			return
		}

		if len(scrapeReq.Formats) == 0 {
			scrapeReq.Formats = []string{"markdown"}
		}
		if scrapeReq.Timeout == 0 {
			scrapeReq.Timeout = 30
		}

		result, err := crawler.ScrapeURLWithContext(req.Context(), &scrapeReq, h.Cfg)
		if err != nil {
			log.Printf("Error scraping URL: %v", utils.SanitizeForLog(err.Error()))
			sseClient.Events <- mcp.SSEEvent{Type: "error", Data: map[string]interface{}{"message": "Failed to scrape URL"}}
			sseClient.Done <- true
			return
		}

		event := mcp.SSEEvent{
			Type: "scrape_result",
			Data: map[string]interface{}{
				"url":      scrapeReq.URL,
				"title":    result.Metadata["title"],
				"markdown": result.Markdown,
				"html":     result.HTML,
				"metadata": result.Metadata,
			},
			Timestamp: time.Now(),
		}
		sseClient.Events <- event
		sseClient.Done <- true
	}()

	for {
		select {
		case event := <-sseClient.Events:
			if message, err := mcp.FormatSSEMessage(event); err == nil {
				fmt.Fprint(w, message)
				flusher.Flush()
			}
		case <-sseClient.Done:
			return
		case <-req.Context().Done():
			return
		}
	}
}

func (h *Handler) SSE(c *gin.Context) {
	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "Streaming unsupported!")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(w, "data: {\"type\": \"connected\", \"message\": \"SSE connection established\"}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Fprintf(w, "data: {\"type\": \"heartbeat\", \"timestamp\": \"%s\"}\n\n", time.Now().Format(time.RFC3339))
			flusher.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// MCPScrape handles MCP scraping requests
func (h *Handler) MCPScrape(c *gin.Context) {
	if c.GetHeader("Accept") == "text/event-stream" {
		h.SSEScrape(c)
		return
	}
	var req crawler.ScrapeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeJSON(c, http.StatusBadRequest, nil, err)
		return
	}

	result, err := crawler.ScrapeURLWithContext(c.Request.Context(), &req, h.Cfg)
	if err != nil {
		writeJSON(c, http.StatusInternalServerError, nil, err)
		return
	}

	writeJSON(c, http.StatusOK, result, nil)
}

// MCPCrawl handles MCP crawling requests
func (h *Handler) MCPCrawl(c *gin.Context) {
	writeJSON(c, http.StatusNotImplemented, nil, fmt.Errorf("crawl job management not implemented yet"))
}

// MCPStats handles MCP queue statistics requests
func (h *Handler) MCPStats(c *gin.Context) {
	stats := map[string]interface{}{
		"activeJobs":    0,
		"pendingJobs":   0,
		"completedJobs": 0,
		"failedJobs":    0,
		"serverTime":    time.Now().Format(time.RFC3339),
	}

	writeJSON(c, http.StatusOK, stats, nil)
}

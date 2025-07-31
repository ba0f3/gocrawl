package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/crawler"
	"gocrawl/internal/db"
	"gocrawl/internal/mcp"
	"gocrawl/internal/user"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	DB           *db.Database
	Cfg          *config.Config
	crawlManager *CrawlManager
	SSEManager   *mcp.SSEManager
	MCPServer    *mcp.MCPServer
}

func NewHandler(database *db.Database, cfg *config.Config) *Handler {
	sseManager := mcp.NewSSEManager()
	mcpServer, _ := mcp.NewMCPServer(database, cfg)

	return &Handler{
		DB:           database,
		Cfg:          cfg,
		crawlManager: NewCrawlManager(database, cfg),
		SSEManager:   sseManager,
		MCPServer:    mcpServer,
	}
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func writeResponse(w http.ResponseWriter, statusCode int, data interface{}, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := ApiResponse{}
	if err != nil {
		resp.Success = false
		resp.Error = err.Error()
	} else {
		resp.Success = true
		resp.Data = data
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeResponse(w, http.StatusBadRequest, nil, err)
		return
	}

	u, err := user.Register(h.DB, creds.Username, creds.Password)
	if err != nil {
		writeResponse(w, http.StatusInternalServerError, nil, err)
		return
	}

	writeResponse(w, http.StatusCreated, u, nil)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeResponse(w, http.StatusBadRequest, nil, err)
		return
	}

	u, err := user.Login(h.DB, creds.Username, creds.Password)
	if err != nil {
		writeResponse(w, http.StatusUnauthorized, nil, err)
		return
	}

	writeResponse(w, http.StatusOK, u, nil)
}

func (h *Handler) GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	// This should be protected and only accessible to logged-in users
	// For simplicity, we are not implementing full session management here
	writeResponse(w, http.StatusNotImplemented, nil, fmt.Errorf("not implemented"))
}

func (h *Handler) Scrape(w http.ResponseWriter, r *http.Request) {
	var req crawler.CrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeResponse(w, http.StatusBadRequest, nil, err)
		return
	}

	result, err := crawler.CrawlURL(&req, h.Cfg)
	if err != nil {
		writeResponse(w, http.StatusInternalServerError, nil, err)
		return
	}

	writeResponse(w, http.StatusOK, result, nil)
}

// CrawlRequest represents the request body for /api/v1/crawl
type CrawlRequestBody struct {
	URL                 string                `json:"url"`
	ExcludePaths        []string              `json:"excludePaths,omitempty"`
	IncludePaths        []string              `json:"includePaths,omitempty"`
	MaxDepth            int                   `json:"maxDepth,omitempty"`
	MaxDiscoveryDepth   int                   `json:"maxDiscoveryDepth,omitempty"`
	IgnoreSitemap       bool                  `json:"ignoreSitemap,omitempty"`
	IgnoreQueryParams   bool                  `json:"ignoreQueryParameters,omitempty"`
	Limit               int                   `json:"limit,omitempty"`
	AllowBackwardLinks  bool                  `json:"allowBackwardLinks,omitempty"`
	CrawlEntireDomain   bool                  `json:"crawlEntireDomain,omitempty"`
	AllowExternalLinks  bool                  `json:"allowExternalLinks,omitempty"`
	AllowSubdomains     bool                  `json:"allowSubdomains,omitempty"`
	Delay               int                   `json:"delay,omitempty"`
	MaxConcurrency      int                   `json:"maxConcurrency,omitempty"`
	ScrapeOptions       *crawler.CrawlRequest `json:"scrapeOptions,omitempty"`
	ZeroDataRetention   bool                  `json:"zeroDataRetention,omitempty"`
}

// CrawlResponse represents the response for /api/v1/crawl
type CrawlResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
	URL     string `json:"url"`
}

func (h *Handler) Crawl(w http.ResponseWriter, r *http.Request) {
// Get user from context (set by auth middleware)
	var userObjID primitive.ObjectID
	if user, ok := r.Context().Value("user").(*db.User); ok && user != nil {
		userObjID = user.ID
	} else {
		if !h.Cfg.Security.DisableAuth {
			writeResponse(w, http.StatusUnauthorized, nil, fmt.Errorf("unauthorized"))
			return
		}
		// Use default user ID when auth is disabled
		userObjID = primitive.NewObjectID()
	}

	var req CrawlRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeResponse(w, http.StatusBadRequest, nil, err)
		return
	}

	// Set defaults
	if req.MaxDepth == 0 {
		req.MaxDepth = 10
	}
	if req.Limit == 0 {
		req.Limit = 10000
	}
	if req.MaxConcurrency == 0 {
		req.MaxConcurrency = 10
	}

	// Create a new crawl job
	jobID := uuid.New().String()
	crawlJob := &db.CrawlJob{
		ID:          jobID,
		URL:         req.URL,
		Status:      "crawling",
		Total:       0,
		Completed:   0,
		CreditsUsed: 0,
		ExpiresAt:   time.Now().Add(24 * time.Hour), // Jobs expire after 24 hours
		UserID:      userObjID,
		CreatedAt:   time.Now(),
	}

	// Save job to database
	if err := h.DB.CreateCrawlJob(crawlJob); err != nil {
		writeResponse(w, http.StatusInternalServerError, nil, err)
		return
	}

	// Start crawl asynchronously
	go h.crawlManager.StartCrawl(jobID, &req)

	// Return job ID immediately
	response := CrawlResponse{
		Success: true,
		ID:      jobID,
		URL:     req.URL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// CrawlStatusResponse represents the response for /api/v1/crawl/{id}
type CrawlStatusResponse struct {
	Status      string                 `json:"status"`
	Total       int                    `json:"total"`
	Completed   int                    `json:"completed"`
	CreditsUsed int                    `json:"creditsUsed"`
	ExpiresAt   string                 `json:"expiresAt"`
	Next        string                 `json:"next,omitempty"`
	Data        []crawler.CrawlResult  `json:"data,omitempty"`
}

func (h *Handler) GetCrawlStatus(w http.ResponseWriter, r *http.Request) {
	// Get job ID from URL parameters
	vars := mux.Vars(r)
	jobID := vars["id"]

	if jobID == "" {
		writeResponse(w, http.StatusBadRequest, nil, fmt.Errorf("job ID is required"))
		return
	}

	// Get crawl job from database
	job, err := h.DB.GetCrawlJob(jobID)
	if err != nil {
		writeResponse(w, http.StatusNotFound, nil, fmt.Errorf("job not found"))
		return
	}

	// Get crawl results
	results, err := h.DB.GetCrawlResults(jobID)
	if err != nil {
		writeResponse(w, http.StatusInternalServerError, nil, err)
		return
	}

	// Convert database results to API format
	apiResults := make([]crawler.CrawlResult, len(results))
	for i, result := range results {
		apiResults[i] = crawler.CrawlResult{
			Markdown: result.Markdown,
			HTML:     result.HTML,
			RawHTML:  result.RawHTML,
			Links:    result.Links,
			Metadata: result.Metadata,
		}
	}

	response := CrawlStatusResponse{
		Status:      job.Status,
		Total:       job.Total,
		Completed:   job.Completed,
		CreditsUsed: job.CreditsUsed,
		ExpiresAt:   job.ExpiresAt.Format(time.RFC3339),
		Data:        apiResults,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// SSE handles Server-Sent Events connections
func (h *Handler) SSEScrape(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
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

	go func() {
		var req crawler.CrawlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("Error decoding request: %v", err)
			sseClient.Events <- mcp.SSEEvent{Type: "error", Data: map[string]interface{}{"message": err.Error()}}
			return
		}

		// Set defaults
		if len(req.Formats) == 0 {
			req.Formats = []string{"markdown"}
		}
		if req.Timeout == 0 {
			req.Timeout = 30
		}

		// Perform the scrape
		result, err := crawler.CrawlURL(&req, h.Cfg)
		if err != nil {
			log.Printf("Error scraping URL: %v", err)
			sseClient.Events <- mcp.SSEEvent{Type: "error", Data: map[string]interface{}{"message": err.Error()}}
			return
		}

		// Stream the result
		event := mcp.SSEEvent{
			Type: "scrape_result", 
			Data: map[string]interface{}{
				"url": req.URL,
				"title": result.Metadata["title"],
				"markdown": result.Markdown,
				"html": result.HTML,
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
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handler) SSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial connection message
	fmt.Fprintf(w, "data: {\"type\": \"connected\", \"message\": \"SSE connection established\"}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send heartbeat
			fmt.Fprintf(w, "data: {\"type\": \"heartbeat\", \"timestamp\": \"%s\"}\n\n", time.Now().Format(time.RFC3339))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// MCPScrape handles MCP scraping requests
func (h *Handler) MCPScrape(w http.ResponseWriter, r *http.Request) {
	sendSSE := r.Header.Get("Accept") == "text/event-stream"

	if sendSSE {
		h.SSEScrape(w, r)
		return
	}
	var req crawler.CrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeResponse(w, http.StatusBadRequest, nil, err)
		return
	}

	result, err := crawler.CrawlURL(&req, h.Cfg)
	if err != nil {
		writeResponse(w, http.StatusInternalServerError, nil, err)
		return
	}

	writeResponse(w, http.StatusOK, result, nil)
}

// MCPCrawl handles MCP crawling requests
func (h *Handler) MCPCrawl(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement crawl job management
	writeResponse(w, http.StatusNotImplemented, nil, fmt.Errorf("crawl job management not implemented yet"))
}

// MCPStats handles MCP queue statistics requests
func (h *Handler) MCPStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"activeJobs": 0,
		"pendingJobs": 0,
		"completedJobs": 0,
		"failedJobs": 0,
		"serverTime": time.Now().Format(time.RFC3339),
	}

	// TODO: Implement actual statistics gathering
	writeResponse(w, http.StatusOK, stats, nil)
}

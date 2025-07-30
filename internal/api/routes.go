package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/crawler"
	"gocrawl/internal/db"
	"gocrawl/internal/user"
)

type Handler struct {
	DB  *db.Database
	Cfg *config.Config
}

func NewHandler(database *db.Database, cfg *config.Config) *Handler {
	return &Handler{DB: database, Cfg: cfg}
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

func (h *Handler) Crawl(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, http.StatusNotImplemented, nil, fmt.Errorf("not implemented"))
}

func (h *Handler) GetCrawlStatus(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, http.StatusNotImplemented, nil, fmt.Errorf("not implemented"))
}

// SSE handles Server-Sent Events connections
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

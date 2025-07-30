package api

import (
	"encoding/json"
	"fmt"
	"net/http"

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

package api

import (
	"os"

	"gocrawl/internal/config"

	"github.com/gin-gonic/gin"
)

// NewRouter builds the Gin engine with /v1 routes and middleware. SSE is mounted
// at /v1/sse with logging and auth only (no CORS/rate-limit on that path), matching
// the previous mux layout.
func NewRouter(h *Handler, cfg *config.Config) *gin.Engine {
	if os.Getenv("GIN_MODE") == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/v1")
	v1.Use(GinLoggingMiddleware(), GinCORSMiddleware(cfg.Security.AllowedOrigins))
	if cfg.RateLimit.Requests > 0 && cfg.RateLimit.Window > 0 {
		v1.Use(GinRateLimitMiddleware(cfg.RateLimit))
	}

	v1.POST("/auth/register", h.Register)
	v1.POST("/auth/login", h.Login)
	v1.POST("/auth/api-key", h.GenerateAPIKey)

	mcpG := v1.Group("/mcp")
	mcpG.Use(GinAuthMiddleware(h.DB, cfg.Security.EnableAuth))
	mcpG.POST("/scrape", h.MCPScrape)
	mcpG.POST("/crawl", h.MCPCrawl)
	mcpG.GET("/stats", h.MCPStats)

	protected := v1.Group("")
	protected.Use(GinAuthMiddleware(h.DB, cfg.Security.EnableAuth))
	protected.POST("/scrape", h.Scrape)
	protected.POST("/crawl", h.Crawl)
	protected.GET("/crawl/:id", h.GetCrawlStatus)

	if cfg.SSE.Enable {
		sse := r.Group("/v1/sse")
		sse.Use(GinLoggingMiddleware(), GinAuthMiddleware(h.DB, cfg.Security.EnableAuth))
		sse.GET("", h.SSE)
	}

	return r
}

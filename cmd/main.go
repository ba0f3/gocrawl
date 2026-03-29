package main

import (
	"fmt"
	"log"
	"net/http"

	"gocrawl/internal/api"
	"gocrawl/internal/config"
	"gocrawl/internal/db"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath("./")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	var store db.Store
	if cfg.Security.EnableAuth {
		var err error
		store, err = db.OpenStore(cfg)
		if err != nil {
			log.Fatal("Failed to initialize database:", err)
		}
		defer func() {
			if err := store.Close(); err != nil {
				log.Printf("database close: %v", err)
			}
		}()

		if err := store.InitSchema(); err != nil {
			log.Fatal("Failed to init database schema:", err)
		}

		store.StartCleanupRoutine(cfg.Retention)
	} else {
		log.Println("Authentication disabled - running without database")
		store = db.NilStore{}
	}

	handler := api.NewHandler(store, cfg)
	handler.StartCrawlWorkers()

	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/v1").Subrouter()

	apiRouter.HandleFunc("/auth/register", handler.Register).Methods("POST")
	apiRouter.HandleFunc("/auth/login", handler.Login).Methods("POST")
	apiRouter.HandleFunc("/auth/api-key", handler.GenerateAPIKey).Methods("POST")

	apiRouter.Use(api.LoggingMiddleware)
	apiRouter.Use(api.CORSMiddleware)
	if cfg.RateLimit.Requests > 0 && cfg.RateLimit.Window > 0 {
		apiRouter.Use(api.RateLimitMiddleware(cfg.RateLimit))
	}

	if cfg.SSE.Enable {
		sseRouter := router.PathPrefix("/v1/sse").Subrouter()
		sseRouter.Use(api.LoggingMiddleware)
		sseRouter.Use(api.AuthMiddleware(store, cfg.Security.EnableAuth))
		sseRouter.HandleFunc("", handler.SSE).Methods("GET")
		log.Println("SSE endpoint enabled at /v1/sse")
	}

	mcpRouter := apiRouter.PathPrefix("/mcp").Subrouter()
	mcpRouter.Use(api.AuthMiddleware(store, cfg.Security.EnableAuth))
	mcpRouter.HandleFunc("/scrape", handler.MCPScrape).Methods("POST")
	mcpRouter.HandleFunc("/crawl", handler.MCPCrawl).Methods("POST")
	mcpRouter.HandleFunc("/stats", handler.MCPStats).Methods("GET")

	protectedRouter := apiRouter.PathPrefix("").Subrouter()
	protectedRouter.Use(api.AuthMiddleware(store, cfg.Security.EnableAuth))
	protectedRouter.HandleFunc("/scrape", handler.Scrape).Methods("POST")
	protectedRouter.HandleFunc("/crawl", handler.Crawl).Methods("POST")
	protectedRouter.HandleFunc("/crawl/{id}", handler.GetCrawlStatus).Methods("GET")

	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

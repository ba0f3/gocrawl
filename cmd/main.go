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
	// Load configuration
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

	var database *db.Database

	// Initialize database only if authentication is enabled
	if !cfg.Security.DisableAuth {
		var err error
		database, err = db.Init(cfg.Database.MongoURI, cfg.Database.DBName)
		if err != nil {
			log.Fatal("Failed to initialize database:", err)
		}
		defer database.Close()

		// Initialize indexes
		err = database.InitIndexes()
		if err != nil {
			log.Fatal("Failed to create indexes:", err)
		}

		// Start cleanup routine
		go database.StartCleanupRoutine(cfg.Retention)
	} else {
		log.Println("Authentication disabled - running without database")
	}

	// Create handler with dependencies
	handler := api.NewHandler(database, cfg)

	// Setup routes
	router := mux.NewRouter()

	// API routes
	apiRouter := router.PathPrefix("/v1").Subrouter()

	// User management routes (always available)
	apiRouter.HandleFunc("/auth/register", handler.Register).Methods("POST")
	apiRouter.HandleFunc("/auth/login", handler.Login).Methods("POST")
	apiRouter.HandleFunc("/auth/api-key", handler.GenerateAPIKey).Methods("POST")

	// Apply middleware
	apiRouter.Use(api.LoggingMiddleware)
	apiRouter.Use(api.CORSMiddleware)

	// Apply authentication middleware only to protected routes if auth is enabled
	if !cfg.Security.DisableAuth {
		protectedRouter := apiRouter.PathPrefix("").Subrouter()
		protectedRouter.Use(api.AuthMiddleware(database, cfg.Security.DisableAuth))
		protectedRouter.HandleFunc("/scrape", handler.Scrape).Methods("POST")
		protectedRouter.HandleFunc("/crawl", handler.Crawl).Methods("POST")
		protectedRouter.HandleFunc("/crawl/{id}", handler.GetCrawlStatus).Methods("GET")
	} else {
		// If auth is disabled, add routes without auth middleware
		apiRouter.HandleFunc("/scrape", handler.Scrape).Methods("POST")
		apiRouter.HandleFunc("/crawl", handler.Crawl).Methods("POST")
		apiRouter.HandleFunc("/crawl/{id}", handler.GetCrawlStatus).Methods("GET")
	}

	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

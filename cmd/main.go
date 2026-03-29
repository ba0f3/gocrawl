package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"gocrawl/internal/api"
	"gocrawl/internal/config"
	"gocrawl/internal/db"

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
		log.Println("Authentication disabled - using in-memory crawl job store (register/login unavailable)")
		store = db.NewMemoryStore()
	}

	handler := api.NewHandler(store, cfg)
	handler.StartCrawlWorkers()

	engine := api.NewRouter(handler, cfg)
	if cfg.SSE.Enable {
		log.Println("SSE endpoint enabled at /v1/sse")
	}

	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown: %v", err)
	}
}

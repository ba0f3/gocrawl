package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for our application
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Security  SecurityConfig
	Crawler   CrawlerConfig
	Retention RetentionConfig
	RateLimit RateLimitConfig
	SSE       SSEConfig
}

type ServerConfig struct {
	Port string
	Host string
}

type DatabaseConfig struct {
	MongoURI string
	DBName   string
}

type SecurityConfig struct {
	JWTSecret  string
	EnableAuth bool
}

type CrawlerConfig struct {
	MaxConcurrentCrawls int
	CrawlTimeout        time.Duration
	UserAgent           string
	Proxies             []string
	EnableProxyRotation bool
}

type RetentionConfig struct {
	DataRetentionDays int
	CleanupInterval   time.Duration
}

type RateLimitConfig struct {
	Requests int
	Window   time.Duration
}

type SSEConfig struct {
	Enable bool
}

// Load reads configuration from viper
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: viper.GetString("PORT"),
			Host: viper.GetString("HOST"),
		},
		Database: DatabaseConfig{
			MongoURI: viper.GetString("MONGO_URI"),
			DBName:   viper.GetString("DB_NAME"),
		},
		Security: SecurityConfig{
			JWTSecret:  viper.GetString("JWT_SECRET"),
			EnableAuth: viper.GetBool("ENABLE_AUTH"),
		},
		Crawler: CrawlerConfig{
			MaxConcurrentCrawls: viper.GetInt("MAX_CONCURRENT_CRAWLS"),
			CrawlTimeout:        viper.GetDuration("CRAWL_TIMEOUT"),
			UserAgent:           viper.GetString("USER_AGENT"),
			Proxies:             viper.GetStringSlice("PROXIES"),
			EnableProxyRotation: viper.GetBool("ENABLE_PROXY_ROTATION"),
		},
		Retention: RetentionConfig{
			DataRetentionDays: viper.GetInt("DATA_RETENTION_DAYS"),
			CleanupInterval:   viper.GetDuration("CLEANUP_INTERVAL"),
		},
		RateLimit: RateLimitConfig{
			Requests: viper.GetInt("RATE_LIMIT_REQUESTS"),
			Window:   viper.GetDuration("RATE_LIMIT_WINDOW"),
		},
		SSE: SSEConfig{
			Enable: viper.GetBool("ENABLE_SSE"),
		},
	}

	// Set defaults
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Database.MongoURI == "" {
		cfg.Database.MongoURI = "mongodb://localhost:27017"
	}
	if cfg.Database.DBName == "" {
		cfg.Database.DBName = "gocrawl"
	}
	if cfg.Crawler.UserAgent == "" {
		cfg.Crawler.UserAgent = "GoCrawl/1.0"
	}
	if cfg.Crawler.MaxConcurrentCrawls == 0 {
		cfg.Crawler.MaxConcurrentCrawls = 10
	}

	return cfg, nil
}

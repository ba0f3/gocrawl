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
	Driver     string // mongo (default), postgres, sqlite
	MongoURI   string
	DBName     string
	DSN        string // postgres connection string or sqlite file path
	SQLitePath string // optional alias for sqlite file when DSN is empty
}

type SecurityConfig struct {
	JWTSecret  string
	EnableAuth bool
}

type CrawlerConfig struct {
	MaxConcurrentCrawls int // per-collector parallelism; also used as crawl worker count when CrawlWorkers is 0
	CrawlWorkers        int // worker goroutines draining the queued crawl job table
	CrawlTimeout        time.Duration
	UserAgent           string
	Proxies             []string
	EnableProxyRotation bool
	CrawlMaxRetries     int
	CrawlRetryBaseDelay time.Duration
	CrawlMinDelay       time.Duration // minimum delay between requests to the same host (global floor with per-request delay)
	ChromedpWSURL       string        // e.g. ws://lightpanda:9222/devtools/browser/... — enables JS fallback scrape
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
			Driver:     viper.GetString("DATABASE_DRIVER"),
			MongoURI:   viper.GetString("MONGO_URI"),
			DBName:     viper.GetString("DB_NAME"),
			DSN:        viper.GetString("DATABASE_DSN"),
			SQLitePath: viper.GetString("SQLITE_PATH"),
		},
		Security: SecurityConfig{
			JWTSecret:  viper.GetString("JWT_SECRET"),
			EnableAuth: viper.GetBool("ENABLE_AUTH"),
		},
		Crawler: CrawlerConfig{
			MaxConcurrentCrawls: viper.GetInt("MAX_CONCURRENT_CRAWLS"),
			CrawlWorkers:        viper.GetInt("CRAWL_WORKERS"),
			CrawlTimeout:        viper.GetDuration("CRAWL_TIMEOUT"),
			UserAgent:           viper.GetString("USER_AGENT"),
			Proxies:             viper.GetStringSlice("PROXIES"),
			EnableProxyRotation: viper.GetBool("ENABLE_PROXY_ROTATION"),
			CrawlMaxRetries:     viper.GetInt("CRAWL_MAX_RETRIES"),
			CrawlRetryBaseDelay: viper.GetDuration("CRAWL_RETRY_BASE_DELAY"),
			CrawlMinDelay:       viper.GetDuration("CRAWL_MIN_DELAY"),
			ChromedpWSURL:       viper.GetString("LIGHTPANDA_WS_URL"),
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
	if cfg.Crawler.CrawlWorkers == 0 {
		cfg.Crawler.CrawlWorkers = cfg.Crawler.MaxConcurrentCrawls
	}
	if cfg.Crawler.CrawlMaxRetries == 0 {
		cfg.Crawler.CrawlMaxRetries = 3
	}
	if cfg.Crawler.CrawlRetryBaseDelay == 0 {
		cfg.Crawler.CrawlRetryBaseDelay = time.Second
	}

	return cfg, nil
}

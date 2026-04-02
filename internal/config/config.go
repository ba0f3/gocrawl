package config

import (
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for our application
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Security  SecurityConfig
	Crawler   CrawlerConfig
	LLM       LLMConfig
	Retention RetentionConfig
	RateLimit RateLimitConfig
	SSE       SSEConfig
}

// LLMConfig enables optional OpenAI-compatible summarization (cheap local or hosted models).
type LLMConfig struct {
	Enabled bool
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

type ServerConfig struct {
	Port string
	Host string
	// HTTP server tuning (0 = use stdlib default where applicable; WriteTimeout 0 disables limit).
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	ShutdownTimeout   time.Duration
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
	LightpandaHTTPURL   string        // e.g. http://lightpanda:9222 — resolved to webSocketDebuggerUrl on first use
	ChromedpMaxConcurrent int         // max concurrent chromedp sessions (default 8 when unset/zero)
	ChromedpNavWait       time.Duration // sleep after load + settle before hydration polling (SPA paint time)
	ChromedpLoadWaitTimeout time.Duration // max time to wait for document.readyState === "complete"
	ChromedpHydrationPollEvery time.Duration
	ChromedpHydrationMaxPolls    int
	ChromedpHydrationMinTextRunes int
	ChromedpFallbackStatusCodes   []int // HTTP status codes that trigger chromedp when auto fallback is on
	ChromedpAutoFallback          bool  // when false, only forceBrowser uses chromedp
	// EnableChromeTLS uses uTLS (Chrome ClientHello) for Colly HTTP(S) fetches to reduce TLS fingerprint blocks.
	EnableChromeTLS bool
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
			Port:              viper.GetString("PORT"),
			Host:              viper.GetString("HOST"),
			ReadHeaderTimeout: viper.GetDuration("SERVER_READ_HEADER_TIMEOUT"),
			ReadTimeout:       viper.GetDuration("SERVER_READ_TIMEOUT"),
			WriteTimeout:      viper.GetDuration("SERVER_WRITE_TIMEOUT"),
			IdleTimeout:       viper.GetDuration("SERVER_IDLE_TIMEOUT"),
			MaxHeaderBytes:    viper.GetInt("SERVER_MAX_HEADER_BYTES"),
			ShutdownTimeout:   viper.GetDuration("SERVER_SHUTDOWN_TIMEOUT"),
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
		LLM: LLMConfig{
			Enabled: viper.GetBool("LLM_ENABLED"),
			BaseURL: viper.GetString("LLM_BASE_URL"),
			APIKey:  viper.GetString("LLM_API_KEY"),
			Model:   viper.GetString("LLM_MODEL"),
			Timeout: viper.GetDuration("LLM_TIMEOUT"),
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
			LightpandaHTTPURL:   viper.GetString("LIGHTPANDA_HTTP_URL"),
			ChromedpMaxConcurrent: viper.GetInt("CHROMEDP_MAX_CONCURRENT"),
			ChromedpNavWait:            viper.GetDuration("CHROMEDP_NAV_WAIT"),
			ChromedpLoadWaitTimeout:    viper.GetDuration("CHROMEDP_LOAD_WAIT_TIMEOUT"),
			ChromedpHydrationPollEvery: viper.GetDuration("CHROMEDP_HYDRATION_POLL_INTERVAL"),
			ChromedpHydrationMaxPolls:    viper.GetInt("CHROMEDP_HYDRATION_MAX_POLLS"),
			ChromedpHydrationMinTextRunes: viper.GetInt("CHROMEDP_HYDRATION_MIN_TEXT_RUNES"),
			ChromedpAutoFallback:          true,
			EnableChromeTLS:               viper.GetBool("ENABLE_CHROME_TLS"),
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
		cfg.Server.Port = "8151"
	}
	if cfg.Server.ReadHeaderTimeout == 0 {
		cfg.Server.ReadHeaderTimeout = 10 * time.Second
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 60 * time.Second
	}
	// WriteTimeout 0: no limit (long scrapes / streaming)
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 120 * time.Second
	}
	if cfg.Server.MaxHeaderBytes == 0 {
		cfg.Server.MaxHeaderBytes = 1 << 20
	}
	if cfg.Server.ShutdownTimeout == 0 {
		cfg.Server.ShutdownTimeout = 30 * time.Second
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
	if cfg.Crawler.EnableChromeTLS && cfg.Crawler.UserAgent == "GoCrawl/1.0" {
		cfg.Crawler.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
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
	if viper.IsSet("CHROMEDP_AUTO_FALLBACK") {
		cfg.Crawler.ChromedpAutoFallback = viper.GetBool("CHROMEDP_AUTO_FALLBACK")
	}
	if s := viper.GetString("CHROMEDP_FALLBACK_STATUS_CODES"); s != "" {
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if v, err := strconv.Atoi(p); err == nil {
				cfg.Crawler.ChromedpFallbackStatusCodes = append(cfg.Crawler.ChromedpFallbackStatusCodes, v)
			}
		}
	}
	if !viper.IsSet("CHROMEDP_NAV_WAIT") {
		cfg.Crawler.ChromedpNavWait = 500 * time.Millisecond
	}
	if cfg.Crawler.ChromedpLoadWaitTimeout == 0 {
		cfg.Crawler.ChromedpLoadWaitTimeout = 30 * time.Second
	}
	if cfg.Crawler.ChromedpHydrationMaxPolls == 0 {
		cfg.Crawler.ChromedpHydrationMaxPolls = 22
	}
	if cfg.Crawler.ChromedpHydrationPollEvery == 0 {
		cfg.Crawler.ChromedpHydrationPollEvery = 300 * time.Millisecond
	}
	if cfg.Crawler.ChromedpHydrationMinTextRunes == 0 {
		cfg.Crawler.ChromedpHydrationMinTextRunes = 80
	}
	if cfg.LLM.Timeout == 0 {
		cfg.LLM.Timeout = 120 * time.Second
	}

	return cfg, nil
}

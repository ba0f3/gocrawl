package db

import (
	"fmt"
	"time"

	"gocrawl/internal/config"
)

// Store abstracts persistence for users, crawl jobs, and results.
type Store interface {
	CreateUser(user *User) error
	GetUserByUsername(username string) (*User, error)
	GetUserByAPIKey(apiKey string) (*User, error)
	CreateCrawlJob(job *CrawlJob) error
	GetCrawlJob(id string) (*CrawlJob, error)
	UpdateCrawlJob(job *CrawlJob) error
	ClaimNextQueuedJob() (*CrawlJob, error)
	CreateCrawlResult(result *CrawlResult) error
	CreateCrawlResults(results []*CrawlResult) error
	GetCrawlResults(jobID string) ([]*CrawlResult, error)
	InitSchema() error
	StartCleanupRoutine(cfg config.RetentionConfig)
	Close() error
	JobCountsByStatus() (queued, crawling, completed, failed int, err error)
}

// OpenStore connects using config.Database (driver mongo | postgres | sqlite).
func OpenStore(cfg *config.Config) (Store, error) {
	driver := cfg.Database.Driver
	if driver == "" {
		driver = "mongo"
	}
	switch driver {
	case "mongo":
		if cfg.Database.MongoURI == "" {
			return nil, fmt.Errorf("mongo driver requires MONGO_URI")
		}
		return NewMongoStore(cfg.Database.MongoURI, cfg.Database.DBName)
	case "postgres":
		if cfg.Database.DSN == "" {
			return nil, fmt.Errorf("postgres driver requires DATABASE_DSN")
		}
		return NewGormStore("postgres", cfg.Database.DSN)
	case "sqlite":
		path := cfg.Database.DSN
		if path == "" {
			path = cfg.Database.SQLitePath
		}
		if path == "" {
			return nil, fmt.Errorf("sqlite driver requires DATABASE_DSN or SQLITE_PATH")
		}
		return NewGormStore("sqlite", path)
	default:
		return nil, fmt.Errorf("unsupported database driver: %q", cfg.Database.Driver)
	}
}

// NilStore is used when auth is disabled and no database is configured.
type NilStore struct{}

func (NilStore) CreateUser(*User) error { return fmt.Errorf("database is not available") }
func (NilStore) GetUserByUsername(string) (*User, error) {
	return nil, fmt.Errorf("database is not available")
}
func (NilStore) GetUserByAPIKey(string) (*User, error) {
	return nil, fmt.Errorf("database is not available")
}
func (NilStore) CreateCrawlJob(*CrawlJob) error { return nil }
func (NilStore) GetCrawlJob(id string) (*CrawlJob, error) {
	return &CrawlJob{ID: id, Status: "unknown", CreatedAt: time.Now()}, nil
}
func (NilStore) UpdateCrawlJob(*CrawlJob) error                 { return nil }
func (NilStore) ClaimNextQueuedJob() (*CrawlJob, error)         { return nil, nil }
func (NilStore) CreateCrawlResult(*CrawlResult) error           { return nil }
func (NilStore) CreateCrawlResults([]*CrawlResult) error        { return nil }
func (NilStore) GetCrawlResults(string) ([]*CrawlResult, error) { return nil, nil }
func (NilStore) InitSchema() error                              { return nil }
func (NilStore) StartCleanupRoutine(config.RetentionConfig)     {}
func (NilStore) Close() error                                   { return nil }
func (NilStore) JobCountsByStatus() (int, int, int, int, error) {
	return 0, 0, 0, 0, nil
}

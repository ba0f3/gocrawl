package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gocrawl/internal/config"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormStore implements Store with GORM (Postgres or SQLite).
type GormStore struct {
	db      *gorm.DB
	dialect string
	claimMu sync.Mutex // serializes job claims on SQLite
}

type gormUserRow struct {
	ID        string `gorm:"primaryKey;size:32"`
	Username  string `gorm:"uniqueIndex;size:255"`
	Password  string `gorm:"size:255"`
	APIKey    string `gorm:"uniqueIndex;size:64"`
	CreatedAt time.Time
}

func (gormUserRow) TableName() string { return "users" }

type gormCrawlJobRow struct {
	ID          string `gorm:"primaryKey;size:64"`
	URL         string `gorm:"size:4096"`
	Status      string `gorm:"index;size:32"`
	Total       int
	Completed   int
	CreditsUsed int
	ExpiresAt   time.Time
	CreatedAt   time.Time `gorm:"index"`
	UserID      string    `gorm:"index;size:64"`
	RequestJSON string    `gorm:"type:text"`
}

func (gormCrawlJobRow) TableName() string { return "crawl_jobs" }

type gormCrawlResultRow struct {
	ID        string    `gorm:"primaryKey;size:32"`
	JobID     string    `gorm:"index;size:64"`
	URL       string    `gorm:"size:4096"`
	Markdown  string    `gorm:"type:text"`
	HTML      string    `gorm:"type:text"`
	RawHTML   string    `gorm:"type:text"`
	LinksJSON string    `gorm:"type:text"`
	MetaJSON  string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"index"`
}

func (gormCrawlResultRow) TableName() string { return "crawl_results" }

// NewGormStore opens a SQL database (dialect: postgres | sqlite).
func NewGormStore(dialect, dsn string) (*GormStore, error) {
	var dialector gorm.Dialector
	switch dialect {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("gorm: unsupported dialect %q", dialect)
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if dialect == "sqlite" {
		if err := db.Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
			return nil, err
		}
		if err := db.Exec("PRAGMA busy_timeout = 5000;").Error; err != nil {
			return nil, err
		}
	}
	return &GormStore{db: db, dialect: dialect}, nil
}

func (s *GormStore) InitSchema() error {
	return s.db.AutoMigrate(&gormUserRow{}, &gormCrawlJobRow{}, &gormCrawlResultRow{})
}

func (s *GormStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *GormStore) StartCleanupRoutine(cfg config.RetentionConfig) {
	if cfg.CleanupInterval <= 0 {
		return
	}
	ticker := time.NewTicker(cfg.CleanupInterval)
	go func() {
		for range ticker.C {
			s.cleanupOldData(cfg.DataRetentionDays)
		}
	}()
}

func (s *GormStore) cleanupOldData(retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	if err := s.db.Where("created_at < ?", cutoff).Delete(&gormCrawlResultRow{}).Error; err != nil {
		log.Printf("Error cleaning up crawl results: %v", err)
	}
	if err := s.db.Where("created_at < ?", cutoff).Delete(&gormCrawlJobRow{}).Error; err != nil {
		log.Printf("Error cleaning up crawl jobs: %v", err)
	}
}

func (s *GormStore) CreateUser(user *User) error {
	user.CreatedAt = time.Now()
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	row := gormUserRow{
		ID:        user.ID,
		Username:  user.Username,
		Password:  user.Password,
		APIKey:    user.APIKey,
		CreatedAt: user.CreatedAt,
	}
	return s.db.Create(&row).Error
}

func (s *GormStore) CreateCrawlResults(results []*CrawlResult) error {
	if len(results) == 0 {
		return nil
	}

	rows := make([]gormCrawlResultRow, 0, len(results))
	for _, result := range results {
		result.CreatedAt = time.Now()
		if result.ID == "" {
			result.ID = uuid.New().String()
		}
		linksB, _ := json.Marshal(result.Links)
		metaB, _ := json.Marshal(result.Metadata)
		rows = append(rows, gormCrawlResultRow{
			ID:        result.ID,
			JobID:     result.JobID,
			URL:       result.URL,
			Markdown:  result.Markdown,
			HTML:      result.HTML,
			RawHTML:   result.RawHTML,
			LinksJSON: string(linksB),
			MetaJSON:  string(metaB),
			CreatedAt: result.CreatedAt,
		})
	}

	return s.db.CreateInBatches(rows, 100).Error
}

func randomID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (s *GormStore) GetUserByUsername(username string) (*User, error) {
	var row gormUserRow
	if err := s.db.Where("username = ?", username).First(&row).Error; err != nil {
		return nil, err
	}
	return &User{
		ID:        row.ID,
		Username:  row.Username,
		Password:  row.Password,
		APIKey:    row.APIKey,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (s *GormStore) GetUserByAPIKey(apiKey string) (*User, error) {
	var row gormUserRow
	if err := s.db.Where("api_key = ?", apiKey).First(&row).Error; err != nil {
		return nil, err
	}
	return &User{
		ID:        row.ID,
		Username:  row.Username,
		Password:  row.Password,
		APIKey:    row.APIKey,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (s *GormStore) CreateCrawlJob(job *CrawlJob) error {
	job.CreatedAt = time.Now()
	row := gormCrawlJobRow{
		ID:          job.ID,
		URL:         job.URL,
		Status:      job.Status,
		Total:       job.Total,
		Completed:   job.Completed,
		CreditsUsed: job.CreditsUsed,
		ExpiresAt:   job.ExpiresAt,
		CreatedAt:   job.CreatedAt,
		UserID:      job.UserID,
		RequestJSON: job.RequestJSON,
	}
	return s.db.Create(&row).Error
}

func jobRowToJob(row *gormCrawlJobRow) *CrawlJob {
	if row == nil {
		return nil
	}
	return &CrawlJob{
		ID:          row.ID,
		URL:         row.URL,
		Status:      row.Status,
		Total:       row.Total,
		Completed:   row.Completed,
		CreditsUsed: row.CreditsUsed,
		ExpiresAt:   row.ExpiresAt,
		CreatedAt:   row.CreatedAt,
		UserID:      row.UserID,
		RequestJSON: row.RequestJSON,
	}
}

func (s *GormStore) GetCrawlJob(id string) (*CrawlJob, error) {
	var row gormCrawlJobRow
	if err := s.db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return jobRowToJob(&row), nil
}

func (s *GormStore) UpdateCrawlJob(job *CrawlJob) error {
	return s.db.Model(&gormCrawlJobRow{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
		"url":          job.URL,
		"status":       job.Status,
		"total":        job.Total,
		"completed":    job.Completed,
		"credits_used": job.CreditsUsed,
		"expires_at":   job.ExpiresAt,
		"user_id":      job.UserID,
		"request_json": job.RequestJSON,
	}).Error
}

func (s *GormStore) ClaimNextQueuedJob() (*CrawlJob, error) {
	if s.dialect == "sqlite" {
		s.claimMu.Lock()
		defer s.claimMu.Unlock()
	}
	var out *CrawlJob
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var job gormCrawlJobRow
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status = ?", "queued").
			Order("created_at ASC")
		if err := q.First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if err := tx.Model(&gormCrawlJobRow{}).Where("id = ?", job.ID).Update("status", "crawling").Error; err != nil {
			return err
		}
		job.Status = "crawling"
		out = jobRowToJob(&job)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *GormStore) CreateCrawlResult(result *CrawlResult) error {
	result.CreatedAt = time.Now()
	if result.ID == "" {
		result.ID = uuid.New().String()
	}
	linksB, _ := json.Marshal(result.Links)
	metaB, _ := json.Marshal(result.Metadata)
	row := gormCrawlResultRow{
		ID:        result.ID,
		JobID:     result.JobID,
		URL:       result.URL,
		Markdown:  result.Markdown,
		HTML:      result.HTML,
		RawHTML:   result.RawHTML,
		LinksJSON: string(linksB),
		MetaJSON:  string(metaB),
		CreatedAt: result.CreatedAt,
	}
	return s.db.Create(&row).Error
}

func (s *GormStore) GetCrawlResults(jobID string) ([]*CrawlResult, error) {
	var rows []gormCrawlResultRow
	if err := s.db.Where("job_id = ?", jobID).Order("created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*CrawlResult, 0, len(rows))
	for _, row := range rows {
		var links []string
		_ = json.Unmarshal([]byte(row.LinksJSON), &links)
		meta := map[string]string{}
		_ = json.Unmarshal([]byte(row.MetaJSON), &meta)
		out = append(out, &CrawlResult{
			ID:        row.ID,
			JobID:     row.JobID,
			URL:       row.URL,
			Markdown:  row.Markdown,
			HTML:      row.HTML,
			RawHTML:   row.RawHTML,
			Links:     links,
			Metadata:  meta,
			CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func (s *GormStore) JobCountsByStatus() (queued, crawling, completed, failed int, err error) {
	var counts []struct {
		Status string `gorm:"column:status"`
		Cnt    int64  `gorm:"column:cnt"`
	}
	err = s.db.Raw("SELECT status, COUNT(*) AS cnt FROM crawl_jobs GROUP BY status").Scan(&counts).Error
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for _, c := range counts {
		switch c.Status {
		case "queued":
			queued = int(c.Cnt)
		case "crawling":
			crawling = int(c.Cnt)
		case "completed":
			completed = int(c.Cnt)
		case "failed":
			failed = int(c.Cnt)
		}
	}
	return queued, crawling, completed, failed, nil
}

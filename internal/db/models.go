package db

import "time"

// User is persisted in all database backends (ID is hex ObjectID for Mongo, UUID for SQL).
type User struct {
	ID        string    `json:"id" bson:"_id,omitempty"`
	Username  string    `json:"username" bson:"username"`
	Password  string    `json:"-" bson:"password"`
	APIKey    string    `json:"apiKey" bson:"apiKey"`
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
}

// CrawlJob represents a crawl job.
type CrawlJob struct {
	ID          string    `json:"id" bson:"_id,omitempty"`
	URL         string    `json:"url" bson:"url"`
	Status      string    `json:"status" bson:"status"`
	Total       int       `json:"total" bson:"total"`
	Completed   int       `json:"completed" bson:"completed"`
	CreditsUsed int       `json:"creditsUsed" bson:"creditsUsed"`
	ExpiresAt   time.Time `json:"expiresAt" bson:"expiresAt"`
	CreatedAt   time.Time `json:"createdAt" bson:"createdAt"`
	UserID      string    `json:"userId" bson:"userId"`
	// RequestJSON holds the serialized api.CrawlRequestBody for queued jobs.
	RequestJSON string `json:"-" bson:"requestJson,omitempty"`
}

// CrawlResult is one page result for a job.
type CrawlResult struct {
	ID        string            `json:"id" bson:"_id,omitempty"`
	JobID     string            `json:"jobId" bson:"jobId"`
	URL       string            `json:"url" bson:"url"`
	Markdown  string            `json:"markdown" bson:"markdown"`
	HTML      string            `json:"html" bson:"html"`
	RawHTML   string            `json:"rawHtml" bson:"rawHtml"`
	Links     []string          `json:"links" bson:"links"`
	Metadata  map[string]string `json:"metadata" bson:"metadata"`
	CreatedAt time.Time         `json:"createdAt" bson:"createdAt"`
}

package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"gocrawl/internal/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Database wrapper for MongoDB
type Database struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// CrawlJob represents a crawl job in MongoDB
type CrawlJob struct {
	ID          string             `bson:"_id,omitempty" json:"id"`
	URL         string             `bson:"url" json:"url"`
	Status      string             `bson:"status" json:"status"`
	Total       int                `bson:"total" json:"total"`
	Completed   int                `bson:"completed" json:"completed"`
	CreditsUsed int                `bson:"creditsUsed" json:"creditsUsed"`
	ExpiresAt   time.Time          `bson:"expiresAt" json:"expiresAt"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UserID      primitive.ObjectID `bson:"userId" json:"userId"`
}

// CrawlResult represents a single crawl result in MongoDB
type CrawlResult struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	JobID     string             `bson:"jobId" json:"jobId"`
	URL       string             `bson:"url" json:"url"`
	Markdown  string             `bson:"markdown" json:"markdown"`
	HTML      string             `bson:"html" json:"html"`
	RawHTML   string             `bson:"rawHtml" json:"rawHtml"`
	Links     []string           `bson:"links" json:"links"`
	Metadata  map[string]string  `bson:"metadata" json:"metadata"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

// User represents a user in MongoDB
type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username string             `bson:"username" json:"username"`
	Password string             `bson:"password" json:"-"`
	APIKey   string             `bson:"apiKey" json:"apiKey"`
	CreatedAt time.Time         `bson:"createdAt" json:"createdAt"`
}

// Init initializes the MongoDB connection
func Init(mongoURI, dbName string) (*Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Test the connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(dbName)
	
	return &Database{
		Client: client,
		DB:     db,
	}, nil
}

// InitIndexes creates necessary indexes
func (d *Database) InitIndexes() error {
	ctx := context.Background()

	// Create index on users collection
	usersCollection := d.DB.Collection("users")
	_, err := usersCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "apiKey", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create users indexes: %w", err)
	}

	// Create index on crawl_jobs collection
	jobsCollection := d.DB.Collection("crawl_jobs")
	_, err = jobsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "userId", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "createdAt", Value: 1}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create crawl_jobs indexes: %w", err)
	}

	// Create index on crawl_results collection
	resultsCollection := d.DB.Collection("crawl_results")
	_, err = resultsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "jobId", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "createdAt", Value: 1}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create crawl_results indexes: %w", err)
	}

	return nil
}

// Close closes the MongoDB connection
func (d *Database) Close() error {
	if d == nil || d.Client == nil {
		// Nothing to close when database is nil
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.Client.Disconnect(ctx)
}

// StartCleanupRoutine starts a goroutine to clean up old data
func (d *Database) StartCleanupRoutine(cfg config.RetentionConfig) {
	ticker := time.NewTicker(cfg.CleanupInterval)
	go func() {
		for range ticker.C {
			d.cleanupOldData(cfg.DataRetentionDays)
		}
	}()
}

func (d *Database) cleanupOldData(retentionDays int) {
	ctx := context.Background()
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Delete old crawl results
	_, err := d.DB.Collection("crawl_results").DeleteMany(ctx, bson.M{
		"createdAt": bson.M{"$lt": cutoff},
	})
	if err != nil {
		log.Printf("Error cleaning up crawl results: %v", err)
	}

	// Delete old crawl jobs
	_, err = d.DB.Collection("crawl_jobs").DeleteMany(ctx, bson.M{
		"createdAt": bson.M{"$lt": cutoff},
	})
	if err != nil {
		log.Printf("Error cleaning up crawl jobs: %v", err)
	}
}

// User operations

// CreateUser creates a new user
func (d *Database) CreateUser(user *User) error {
	if d == nil || d.DB == nil {
		// When database is disabled, return error
		return fmt.Errorf("database is not available")
	}
	user.CreatedAt = time.Now()
	_, err := d.DB.Collection("users").InsertOne(context.Background(), user)
	return err
}

// GetUserByUsername retrieves a user by username
func (d *Database) GetUserByUsername(username string) (*User, error) {
	if d == nil || d.DB == nil {
		// When database is disabled, return error
		return nil, fmt.Errorf("database is not available")
	}
	var user User
	err := d.DB.Collection("users").FindOne(context.Background(), bson.M{
		"username": username,
	}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByAPIKey retrieves a user by API key
func (d *Database) GetUserByAPIKey(apiKey string) (*User, error) {
	if d == nil || d.DB == nil {
		// When database is disabled, return error
		return nil, fmt.Errorf("database is not available")
	}
	var user User
	err := d.DB.Collection("users").FindOne(context.Background(), bson.M{
		"apiKey": apiKey,
	}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CrawlJob operations

// CreateCrawlJob creates a new crawl job
func (d *Database) CreateCrawlJob(job *CrawlJob) error {
	if d == nil || d.DB == nil {
		// When database is disabled, just return success
		return nil
	}
	job.CreatedAt = time.Now()
	_, err := d.DB.Collection("crawl_jobs").InsertOne(context.Background(), job)
	return err
}

// GetCrawlJob retrieves a crawl job by ID
func (d *Database) GetCrawlJob(id string) (*CrawlJob, error) {
	if d == nil || d.DB == nil {
		// Return a dummy job when database is disabled
		return &CrawlJob{
			ID:        id,
			Status:    "unknown",
			CreatedAt: time.Now(),
		}, nil
	}
	var job CrawlJob
	err := d.DB.Collection("crawl_jobs").FindOne(context.Background(), bson.M{
		"_id": id,
	}).Decode(&job)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateCrawlJob updates a crawl job
func (d *Database) UpdateCrawlJob(job *CrawlJob) error {
	if d == nil || d.DB == nil {
		// When database is disabled, just return success
		return nil
	}
	_, err := d.DB.Collection("crawl_jobs").UpdateOne(
		context.Background(),
		bson.M{"_id": job.ID},
		bson.M{"$set": job},
	)
	return err
}

// CrawlResult operations

// CreateCrawlResult creates a new crawl result
func (d *Database) CreateCrawlResult(result *CrawlResult) error {
	if d == nil || d.DB == nil {
		// When database is disabled, just return success
		return nil
	}
	result.CreatedAt = time.Now()
	_, err := d.DB.Collection("crawl_results").InsertOne(context.Background(), result)
	return err
}

// GetCrawlResults retrieves crawl results by job ID
func (d *Database) GetCrawlResults(jobID string) ([]*CrawlResult, error) {
	if d == nil || d.DB == nil {
		// Return empty results when database is disabled
		return []*CrawlResult{}, nil
	}
	cursor, err := d.DB.Collection("crawl_results").Find(context.Background(), bson.M{
		"jobId": jobID,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var results []*CrawlResult
	for cursor.Next(context.Background()) {
		var result CrawlResult
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		results = append(results, &result)
	}

	return results, nil
}

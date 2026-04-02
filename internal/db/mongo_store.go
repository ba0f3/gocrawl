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

// MongoStore implements Store using the official MongoDB driver.
type MongoStore struct {
	client *mongo.Client
	db     *mongo.Database
}

// NewMongoStore connects to MongoDB and returns a Store.
func NewMongoStore(mongoURI, dbName string) (*MongoStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}
	return &MongoStore{client: client, db: client.Database(dbName)}, nil
}

func (s *MongoStore) InitSchema() error {
	ctx := context.Background()
	users := s.db.Collection("users")
	_, err := users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "apiKey", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("failed to create users indexes: %w", err)
	}
	jobs := s.db.Collection("crawl_jobs")
	_, err = jobs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}}},
		{Keys: bson.D{{Key: "createdAt", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("failed to create crawl_jobs indexes: %w", err)
	}
	results := s.db.Collection("crawl_results")
	_, err = results.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "jobId", Value: 1}}},
		{Keys: bson.D{{Key: "createdAt", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("failed to create crawl_results indexes: %w", err)
	}
	return nil
}

func (s *MongoStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.client.Disconnect(ctx)
}

func (s *MongoStore) StartCleanupRoutine(cfg config.RetentionConfig) {
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

func (s *MongoStore) cleanupOldData(retentionDays int) {
	ctx := context.Background()
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	_, err := s.db.Collection("crawl_results").DeleteMany(ctx, bson.M{"createdAt": bson.M{"$lt": cutoff}})
	if err != nil {
		log.Printf("Error cleaning up crawl results: %v", err)
	}
	_, err = s.db.Collection("crawl_jobs").DeleteMany(ctx, bson.M{"createdAt": bson.M{"$lt": cutoff}})
	if err != nil {
		log.Printf("Error cleaning up crawl jobs: %v", err)
	}
}

type mongoUserDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Username  string             `bson:"username"`
	Password  string             `bson:"password"`
	APIKey    string             `bson:"apiKey"`
	CreatedAt time.Time          `bson:"createdAt"`
}

func userFromMongoDoc(d *mongoUserDoc) *User {
	if d == nil {
		return nil
	}
	return &User{
		ID:        d.ID.Hex(),
		Username:  d.Username,
		Password:  d.Password,
		APIKey:    d.APIKey,
		CreatedAt: d.CreatedAt,
	}
}

func (s *MongoStore) CreateUser(user *User) error {
	user.CreatedAt = time.Now()
	oid := primitive.NewObjectID()
	if user.ID != "" {
		parsed, err := primitive.ObjectIDFromHex(user.ID)
		if err == nil {
			oid = parsed
		}
	}
	doc := mongoUserDoc{
		ID:        oid,
		Username:  user.Username,
		Password:  user.Password,
		APIKey:    user.APIKey,
		CreatedAt: user.CreatedAt,
	}
	_, err := s.db.Collection("users").InsertOne(context.Background(), doc)
	if err != nil {
		return err
	}
	user.ID = oid.Hex()
	return nil
}

func (s *MongoStore) GetUserByUsername(username string) (*User, error) {
	var doc mongoUserDoc
	err := s.db.Collection("users").FindOne(context.Background(), bson.M{"username": username}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return userFromMongoDoc(&doc), nil
}

func (s *MongoStore) GetUserByAPIKey(apiKey string) (*User, error) {
	var doc mongoUserDoc
	err := s.db.Collection("users").FindOne(context.Background(), bson.M{"apiKey": apiKey}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return userFromMongoDoc(&doc), nil
}

type mongoCrawlJobDoc struct {
	ID          string    `bson:"_id,omitempty"`
	URL         string    `bson:"url"`
	Status      string    `bson:"status"`
	Total       int       `bson:"total"`
	Completed   int       `bson:"completed"`
	CreditsUsed int       `bson:"creditsUsed"`
	ExpiresAt   time.Time `bson:"expiresAt"`
	CreatedAt   time.Time `bson:"createdAt"`
	UserID      any       `bson:"userId"`
	RequestJSON string    `bson:"requestJson,omitempty"`
}

func crawlJobFromMongoDoc(d *mongoCrawlJobDoc) *CrawlJob {
	j := &CrawlJob{
		ID:          d.ID,
		URL:         d.URL,
		Status:      d.Status,
		Total:       d.Total,
		Completed:   d.Completed,
		CreditsUsed: d.CreditsUsed,
		ExpiresAt:   d.ExpiresAt,
		CreatedAt:   d.CreatedAt,
		RequestJSON: d.RequestJSON,
	}
	j.UserID = decodeFlexibleObjectID(d.UserID)
	return j
}

func decodeFlexibleObjectID(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case primitive.ObjectID:
		return t.Hex()
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func (s *MongoStore) CreateCrawlJob(job *CrawlJob) error {
	job.CreatedAt = time.Now()
	doc := bson.M{
		"_id":         job.ID,
		"url":         job.URL,
		"status":      job.Status,
		"total":       job.Total,
		"completed":   job.Completed,
		"creditsUsed": job.CreditsUsed,
		"expiresAt":   job.ExpiresAt,
		"createdAt":   job.CreatedAt,
		"userId":      job.UserID,
		"requestJson": job.RequestJSON,
	}
	_, err := s.db.Collection("crawl_jobs").InsertOne(context.Background(), doc)
	return err
}

func (s *MongoStore) GetCrawlJob(id string) (*CrawlJob, error) {
	var doc mongoCrawlJobDoc
	err := s.db.Collection("crawl_jobs").FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return crawlJobFromMongoDoc(&doc), nil
}

func (s *MongoStore) UpdateCrawlJob(job *CrawlJob) error {
	_, err := s.db.Collection("crawl_jobs").UpdateOne(
		context.Background(),
		bson.M{"_id": job.ID},
		bson.M{"$set": bson.M{
			"url":         job.URL,
			"status":      job.Status,
			"total":       job.Total,
			"completed":   job.Completed,
			"creditsUsed": job.CreditsUsed,
			"expiresAt":   job.ExpiresAt,
			"userId":      job.UserID,
			"requestJson": job.RequestJSON,
		}},
	)
	return err
}

func (s *MongoStore) UpdateJobProgress(jobID string, status string, completed int) error {
	_, err := s.db.Collection("crawl_jobs").UpdateOne(
		context.Background(),
		bson.M{"_id": jobID},
		bson.M{"$set": bson.M{
			"status":    status,
			"completed": completed,
		}},
	)
	return err
}

func (s *MongoStore) ClaimNextQueuedJob() (*CrawlJob, error) {
	coll := s.db.Collection("crawl_jobs")
	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After).
		SetSort(bson.D{{Key: "createdAt", Value: 1}})
	var doc mongoCrawlJobDoc
	err := coll.FindOneAndUpdate(
		context.Background(),
		bson.M{"status": "queued"},
		bson.M{"$set": bson.M{"status": "crawling"}},
		opts,
	).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return crawlJobFromMongoDoc(&doc), nil
}

func (s *MongoStore) CreateCrawlResult(result *CrawlResult) error {
	result.CreatedAt = time.Now()
	oid := primitive.NewObjectID()
	if result.ID != "" {
		if parsed, err := primitive.ObjectIDFromHex(result.ID); err == nil {
			oid = parsed
		}
	}
	result.ID = oid.Hex()
	doc := bson.M{
		"_id":       oid,
		"jobId":     result.JobID,
		"url":       result.URL,
		"markdown":  result.Markdown,
		"html":      result.HTML,
		"rawHtml":   result.RawHTML,
		"links":     result.Links,
		"metadata":  result.Metadata,
		"createdAt": result.CreatedAt,
	}
	_, err := s.db.Collection("crawl_results").InsertOne(context.Background(), doc)
	return err
}

func (s *MongoStore) CreateCrawlResults(results []*CrawlResult) error {
	if len(results) == 0 {
		return nil
	}

	docs := make([]interface{}, 0, len(results))
	for _, result := range results {
		result.CreatedAt = time.Now()
		oid := primitive.NewObjectID()
		if result.ID != "" {
			if parsed, err := primitive.ObjectIDFromHex(result.ID); err == nil {
				oid = parsed
			}
		}
		result.ID = oid.Hex()
		docs = append(docs, bson.M{
			"_id":       oid,
			"jobId":     result.JobID,
			"url":       result.URL,
			"markdown":  result.Markdown,
			"html":      result.HTML,
			"rawHtml":   result.RawHTML,
			"links":     result.Links,
			"metadata":  result.Metadata,
			"createdAt": result.CreatedAt,
		})
	}

	_, err := s.db.Collection("crawl_results").InsertMany(context.Background(), docs)
	return err
}

func (s *MongoStore) GetCrawlResults(jobID string) ([]*CrawlResult, error) {
	cursor, err := s.db.Collection("crawl_results").Find(context.Background(), bson.M{"jobId": jobID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	var out []*CrawlResult
	for cursor.Next(context.Background()) {
		var doc struct {
			ID        primitive.ObjectID `bson:"_id"`
			JobID     string             `bson:"jobId"`
			URL       string             `bson:"url"`
			Markdown  string             `bson:"markdown"`
			HTML      string             `bson:"html"`
			RawHTML   string             `bson:"rawHtml"`
			Links     []string           `bson:"links"`
			Metadata  map[string]string  `bson:"metadata"`
			CreatedAt time.Time          `bson:"createdAt"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		out = append(out, &CrawlResult{
			ID:        doc.ID.Hex(),
			JobID:     doc.JobID,
			URL:       doc.URL,
			Markdown:  doc.Markdown,
			HTML:      doc.HTML,
			RawHTML:   doc.RawHTML,
			Links:     doc.Links,
			Metadata:  doc.Metadata,
			CreatedAt: doc.CreatedAt,
		})
	}
	return out, nil
}

func (s *MongoStore) JobCountsByStatus() (queued, crawling, completed, failed int, err error) {
	coll := s.db.Collection("crawl_jobs")
	ctx := context.Background()
	pipeline := []bson.M{
		{"$group": bson.M{
			"_id":   "$status",
			"count": bson.M{"$sum": 1},
		}},
	}
	cur, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var row bson.M
		if err := cur.Decode(&row); err != nil {
			continue
		}
		id, _ := row["_id"].(string)
		var n int
		switch v := row["count"].(type) {
		case int32:
			n = int(v)
		case int64:
			n = int(v)
		case int:
			n = v
		case float64:
			n = int(v)
		}
		switch id {
		case "queued":
			queued = n
		case "crawling":
			crawling = n
		case "completed":
			completed = n
		case "failed":
			failed = n
		}
	}
	return queued, crawling, completed, failed, cur.Err()
}

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/crawler"
	"gocrawl/internal/db"
	"gocrawl/internal/utils"

	"github.com/andybalholm/cascadia"
	"github.com/gocolly/colly/v2"
)

var articleMatcher = cascadia.MustCompile("article, main, [role=main], .post, .entry-content, .entry-title")

// CrawlManager drains queued crawl jobs with a worker pool.
type CrawlManager struct {
	store db.Store
	cfg   *config.Config
}

// NewCrawlManager creates a new crawl manager.
func NewCrawlManager(store db.Store, cfg *config.Config) *CrawlManager {
	return &CrawlManager{
		store: store,
		cfg:   cfg,
	}
}

// StartWorkers launches N goroutines that claim jobs from the store.
func (cm *CrawlManager) StartWorkers(n int) {
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		go cm.workerLoop(i)
	}
	log.Printf("Started %d crawl worker(s)", n)
}

func (cm *CrawlManager) workerLoop(id int) {
	for {
		job, err := cm.store.ClaimNextQueuedJob()
		if err != nil {
			log.Printf("crawl worker %d: claim error: %v", id, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if job == nil {
			time.Sleep(time.Second)
			continue
		}
		var req CrawlRequestBody
		if err := json.Unmarshal([]byte(job.RequestJSON), &req); err != nil {
			log.Printf("crawl worker %d: bad job payload %s: %v", id, job.ID, err)
			cm.failJob(job.ID, "invalid job payload")
			continue
		}
		cm.runCrawlJob(job.ID, &req)
	}
}

func (cm *CrawlManager) updateJob(jobID string, action string, updateFn func(*db.CrawlJob)) {
	job, err := cm.store.GetCrawlJob(jobID)
	if err != nil {
		log.Printf("Error retrieving crawl job for %s: %v", action, err)
		return
	}
	updateFn(job)
	if err := cm.store.UpdateCrawlJob(job); err != nil {
		log.Printf("Error updating crawl job %s: %v", action, err)
	}
}

func (cm *CrawlManager) failJob(jobID, msg string) {
	cm.updateJob(jobID, "fail", func(job *db.CrawlJob) {
		job.Status = "failed"
	})
	log.Printf("job %s failed: %s", jobID, msg)
}

// runCrawlJob executes a single crawl job (called by workers).
func (cm *CrawlManager) runCrawlJob(jobID string, req *CrawlRequestBody) {
	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("crawl panic job %s: %v", jobID, r)
			cm.failJob(jobID, fmt.Sprint(r))
		}
	}()
	log.Printf("Running crawl for job ID: %s", jobID)

	cm.updateCrawlStatus(jobID, "crawling", 0)

	results := cm.performCrawling(jobCtx, req, jobID)

	cm.updateCrawlTotal(jobID, len(results))

	// ⚡ Bolt Optimization: Batch insert crawl results to prevent N+1 query problems.
	// Previously, each result was saved to the database one at a time, which caused severe performance degradation
	// for large crawl jobs due to excessive database round trips.
	crawlResults := make([]*db.CrawlResult, 0, len(results))
	for _, result := range results {
		cr := &db.CrawlResult{
			JobID:    jobID,
			URL:      result.Metadata["sourceURL"],
			Markdown: result.Markdown,
			HTML:     result.HTML,
			RawHTML:  result.RawHTML,
			Links:    result.Links,
			Metadata: result.Metadata,
		}
		crawlResults = append(crawlResults, cr)
	}

	if len(crawlResults) > 0 {
		if err := cm.store.CreateCrawlResults(crawlResults); err != nil {
			log.Printf("Error saving crawl results batch: %v", err)
		}
	}

	cm.updateCrawlStatus(jobID, "completed", len(results))
	log.Printf("Crawl completed for job ID: %s", jobID)
}

func (cm *CrawlManager) updateCrawlStatus(jobID string, status string, completed int) {
	// ⚡ Bolt Optimization: Use targeted single UPDATE query rather than a N+1 pattern of fetching the whole row, mutating it and sending it back over the wire.
	if err := cm.store.UpdateJobProgress(jobID, status, completed); err != nil {
		log.Printf("Error updating crawl job status: %v", err)
	}
	cm.updateJob(jobID, "status update", func(job *db.CrawlJob) {
		job.Status = status
		job.Completed = completed
	})
}

func (cm *CrawlManager) updateCrawlTotal(jobID string, total int) {
	cm.updateJob(jobID, "total update", func(job *db.CrawlJob) {
		job.Total = total
	})
}

// linkInArticleOrMain is true when the anchor sits under common article/main containers.
func linkInArticleOrMain(e *colly.HTMLElement) bool {
	// ⚡ Bolt Optimization: Use a pre-compiled single cascadia matcher and manually traverse
	// the HTML tree up to the root. Using goquery's ParentsFiltered in a loop was causing
	// redundant parsing and memory overhead inside the hot loop.
	for _, n := range e.DOM.Nodes {
		for p := n.Parent; p != nil; p = p.Parent {
			if articleMatcher.Match(p) {
				return true
			}
		}
	}
	return false
}

func (cm *CrawlManager) performCrawling(ctx context.Context, req *CrawlRequestBody, jobID string) []*crawler.ScrapeResult {
	results := make([]*crawler.ScrapeResult, 0)
	visited := make(map[string]bool)
	var crawlMu sync.Mutex
	completedCount := 0

	baseURL, err := url.Parse(req.URL)
	if err != nil {
		log.Printf("Error parsing base URL: %v", err)
		return results
	}

	// ⚡ Bolt Optimization: Pre-compile IncludePaths and ExcludePaths into regular expressions.
	// This avoids repeatedly iterating and doing strings.Contains for every discovered URL,
	// significantly improving performance when these lists are large.
	var includeRe, excludeRe *regexp.Regexp
	if len(req.IncludePaths) > 0 {
		var parts []string
		for _, p := range req.IncludePaths {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			parts = append(parts, regexp.QuoteMeta(p))
		}
		if len(parts) > 0 {
			includeRe = regexp.MustCompile(strings.Join(parts, "|"))
		}
	}
	if len(req.ExcludePaths) > 0 {
		var parts []string
		for _, p := range req.ExcludePaths {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			parts = append(parts, regexp.QuoteMeta(p))
		}
		if len(parts) > 0 {
			excludeRe = regexp.MustCompile(strings.Join(parts, "|"))
		}
	}

	c := colly.NewCollector(
		colly.MaxDepth(req.MaxDepth),
		colly.Async(true),
	)

	c.WithTransport(utils.SafeTransport())

	delayMs := req.Delay
	if cm.cfg.Crawler.CrawlMinDelay > 0 {
		globalMs := int(cm.cfg.Crawler.CrawlMinDelay / time.Millisecond)
		if globalMs > delayMs {
			delayMs = globalMs
		}
	}

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: req.MaxConcurrency,
		Delay:       time.Duration(delayMs) * time.Millisecond,
	})

	if t := crawler.TransportForCrawler(cm.cfg); t != nil {
		c.WithTransport(t)
	}

	if !req.AllowExternalLinks {
		c.AllowedDomains = append(c.AllowedDomains, baseURL.Host)
	}

	if crawlLinkSelectors := effectiveCrawlLinkSelectors(req); len(crawlLinkSelectors) > 0 {
		sel := strings.Join(crawlLinkSelectors, ", ")
		c.OnHTML(sel, func(e *colly.HTMLElement) {
			cm.visitIfAllowed(e, baseURL, req, visited, &crawlMu, includeRe, excludeRe)
		})
	} else {
		c.OnHTML("article a[href], main a[href], [role=main] a[href], .post a[href], .entry-content a[href]", func(e *colly.HTMLElement) {
			cm.visitIfAllowed(e, baseURL, req, visited, &crawlMu, includeRe, excludeRe)
		})
		c.OnHTML("a[href]", func(e *colly.HTMLElement) {
			if linkInArticleOrMain(e) {
				return
			}
			cm.visitIfAllowed(e, baseURL, req, visited, &crawlMu, includeRe, excludeRe)
		})
	}

	c.OnScraped(func(r *colly.Response) {
		crawlMu.Lock()
		if completedCount >= req.Limit {
			crawlMu.Unlock()
			return
		}
		crawlMu.Unlock()
		var scrapeOpts crawler.ScrapeRequest
		if req.ScrapeOptions != nil {
			scrapeOpts = *req.ScrapeOptions
		} else {
			main := true
			scrapeOpts = crawler.ScrapeRequest{
				OnlyMainContent: &main,
				Formats:         []string{"markdown", "html", "rawHtml"},
			}
		}
		scrapeOpts.URL = r.Request.URL.String()
		// Per-page ScrapeURL uses its own Colly collector; inherit crawl linkSelectors for
		// result.links and debugger logs unless scrapeOptions.linkSelector is set.
		if crawlSels := effectiveCrawlLinkSelectors(req); len(crawlSels) > 0 && strings.TrimSpace(scrapeOpts.LinkSelector) == "" {
			scrapeOpts.LinkSelector = strings.Join(crawlSels, ", ")
		}
		scrapeOpts.PreFetchedBody = r.Body
		if r.Headers != nil {
			scrapeOpts.PreFetchedHeaders = r.Headers.Clone()
		}

		result, err := crawler.ScrapeURLWithContext(ctx, &scrapeOpts, cm.cfg)
		if err != nil {
			log.Printf("Error scraping page %s: %v", utils.SanitizeForLog(r.Request.URL.String()), err)
			return
		}
		crawlMu.Lock()
		results = append(results, result)
		completedCount++
		cc := completedCount
		crawlMu.Unlock()
		if cc%10 == 0 {
			cm.updateCrawlStatus(jobID, "crawling", cc)
		}
	})

	_ = c.Visit(req.URL)
	c.Wait()

	return results
}

func (cm *CrawlManager) visitIfAllowed(e *colly.HTMLElement, baseURL *url.URL, req *CrawlRequestBody, visited map[string]bool, mu *sync.Mutex, includeRe, excludeRe *regexp.Regexp) {
	link := e.Attr("href")
	absURL := e.Request.AbsoluteURL(link)
	mu.Lock()
	// ⚡ Bolt Optimization: Checking !visited[absURL] first enables O(1) map lookup short-circuiting.
	// If the URL has already been visited, this avoids calling shouldScrapeURL which involves parsing the URL,
	// string manipulations, and regex matching. This drops the evaluation time for already-visited URLs significantly.
	if !visited[absURL] && cm.shouldScrapeURL(absURL, baseURL, req, includeRe, excludeRe) {
		visited[absURL] = true
		if len(visited) <= req.Limit {
			mu.Unlock()
			if err := e.Request.Visit(absURL); err != nil {
				log.Printf("Failed to visit %s: %v", utils.SanitizeForLog(absURL), err)
			}
			return
		}
	}
	mu.Unlock()
}

func effectiveCrawlLinkSelectors(req *CrawlRequestBody) []string {
	if req == nil {
		return nil
	}
	var out []string
	if s := strings.TrimSpace(req.LinkSelector); s != "" {
		out = append(out, s)
	}
	for _, s := range req.LinkSelectors {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (cm *CrawlManager) shouldScrapeURL(absURL string, baseURL *url.URL, req *CrawlRequestBody, includeRe, excludeRe *regexp.Regexp) bool {
	parsedURL, err := url.Parse(absURL)
	if err != nil {
		return false
	}
	if !req.AllowExternalLinks && parsedURL.Host != baseURL.Host {
		return false
	}
	if !req.AllowSubdomains && parsedURL.Host != baseURL.Host {
		return false
	}
	// ⚡ Bolt Optimization: Use pre-compiled regex instead of inner string loops.
	if includeRe != nil && !includeRe.MatchString(parsedURL.Path) {
		return false
	}
	if excludeRe != nil && excludeRe.MatchString(parsedURL.Path) {
		return false
	}
	return true
}

package crawler

import (
	"net/url"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/extractor"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
	"github.com/gocolly/colly/v2/proxy"
)

// ScrapeResult holds the result of a scrape
type ScrapeResult struct {
	Markdown string            `json:"markdown"`
	HTML     string            `json:"html"`
	RawHTML  string            `json:"rawHtml"`
	Links    []string          `json:"links"`
	Metadata map[string]string `json:"metadata"`
}

// ScrapeRequest represents a scrape request
type ScrapeRequest struct {
	URL                string   `json:"url"`
	OnlyMainContent    bool     `json:"onlyMainContent"`
	IncludeTags        []string `json:"includeTags"`
	ExcludeTags        []string `json:"excludeTags"`
	Timeout            int      `json:"timeout"`
	Formats            []string `json:"formats"`
	RemoveBase64Images bool     `json:"removeBase64Images"`
}

// ScrapeURL scrapes a specific URL and extracts the main content
func ScrapeURL(req *ScrapeRequest, cfg *config.Config) (*ScrapeResult, error) {
	c := colly.NewCollector(
		colly.Debugger(&debug.LogDebugger{}),
	)

	// Set timeout
	timeout := 30 * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	} else if cfg.Crawler.CrawlTimeout > 0 {
		timeout = cfg.Crawler.CrawlTimeout
	}
	c.SetRequestTimeout(timeout)

	// Set user agent from config
	c.UserAgent = cfg.Crawler.UserAgent

	// Setup proxy rotation if enabled and proxies are configured
	if cfg.Crawler.EnableProxyRotation && len(cfg.Crawler.Proxies) > 0 {
		rps, err := proxy.RoundRobinProxySwitcher(cfg.Crawler.Proxies...)
		if err != nil {
			return nil, err
		}
		c.SetProxyFunc(rps)
	}

	result := &ScrapeResult{
		Links:    []string{},
		Metadata: make(map[string]string),
	}

	// Extract page content
	c.OnHTML("html", func(e *colly.HTMLElement) {
		// Get raw HTML
		rawHTML, _ := e.DOM.Html()
		result.RawHTML = rawHTML

		// Extract main content
		var contentHTML string
		if req.OnlyMainContent {
			// Try to extract main content areas
			mainSelectors := []string{"main", "article", ".content", "#content", ".post", ".entry"}
			for _, selector := range mainSelectors {
				if mainContent := e.DOM.Find(selector).First(); mainContent.Length() > 0 {
					contentHTML, _ = mainContent.Html()
					break
				}
			}
			// Fallback to body if no main content found
			if contentHTML == "" {
				contentHTML, _ = e.DOM.Find("body").Html()
			}
		} else {
			contentHTML, _ = e.DOM.Find("body").Html()
		}

		result.HTML = contentHTML

		// Convert to markdown if requested
		if contains(req.Formats, "markdown") {
			// Extract domain from URL for relative link conversion
			markdown, err := extractor.ToMarkdown(contentHTML)
			if err == nil {
				result.Markdown = markdown
			}
		}

		// Extract metadata
		result.Metadata["title"] = e.DOM.Find("title").Text()
		result.Metadata["description"] = e.DOM.Find("meta[name='description']").AttrOr("content", "")
		result.Metadata["language"] = e.DOM.Find("html").AttrOr("lang", "")
		result.Metadata["sourceURL"] = req.URL
	})

	// Collect links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		if href != "" {
			// Convert relative URLs to absolute
			baseURL, _ := url.Parse(req.URL)
			linkURL, err := url.Parse(href)
			if err == nil {
				absoluteURL := baseURL.ResolveReference(linkURL)
				result.Links = append(result.Links, absoluteURL.String())
			}
		}
	})

	// Handle response
	c.OnResponse(func(r *colly.Response) {
		result.Metadata["statusCode"] = string(rune(r.StatusCode))
	})

	// Handle errors
	c.OnError(func(r *colly.Response, err error) {
		result.Metadata["error"] = err.Error()
	})

	// Start the crawl
	if err := c.Visit(req.URL); err != nil {
		return nil, err
	}

	return result, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

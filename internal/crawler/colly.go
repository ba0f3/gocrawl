package crawler

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/extractor"

	"github.com/PuerkitoBio/goquery"
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
	// ContentSelector / ContentSelectors restrict which nodes become HTML/markdown (tried in order). When set, they override OnlyMainContent defaults.
	ContentSelector  string   `json:"contentSelector,omitempty"`
	ContentSelectors []string `json:"contentSelectors,omitempty"`
	// LinkSelector limits which anchors are collected as links (default "a[href]").
	LinkSelector string `json:"linkSelector,omitempty"`
}

const minMainMarkdownRunes = 80

func scrapeNeedsChromedpFallback(result *ScrapeResult, visitErr error, onlyMain bool) bool {
	if visitErr != nil {
		return true
	}
	if result == nil {
		return true
	}
	if errMsg := result.Metadata["error"]; errMsg != "" {
		return true
	}
	if onlyMain && len([]rune(strings.TrimSpace(result.Markdown))) < minMainMarkdownRunes {
		return true
	}
	return false
}

func finalizeScrape(req *ScrapeRequest, cfg *config.Config, timeout time.Duration, result *ScrapeResult, visitErr error) (*ScrapeResult, error) {
	if cfg != nil && cfg.Crawler.ChromedpWSURL != "" && scrapeNeedsChromedpFallback(result, visitErr, req.OnlyMainContent) {
		html, err := ScrapeHTMLViaChromedp(cfg, req, timeout)
		if err == nil {
			r := buildResultFromMainHTML(html, req)
			r.Metadata["extractor"] = "chromedp"
			return r, nil
		}
	}
	if visitErr != nil {
		return nil, visitErr
	}
	return result, nil
}

func buildResultFromMainHTML(contentHTML string, req *ScrapeRequest) *ScrapeResult {
	result := &ScrapeResult{
		Links:    []string{},
		Metadata: map[string]string{"sourceURL": req.URL},
	}
	result.HTML = contentHTML
	result.RawHTML = contentHTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"root\">" + contentHTML + "</div>"))
	if err == nil {
		doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			if href == "" {
				return
			}
			baseURL, _ := url.Parse(req.URL)
			linkURL, err := url.Parse(href)
			if err == nil {
				result.Links = append(result.Links, baseURL.ResolveReference(linkURL).String())
			}
		})
	}
	if contains(req.Formats, "markdown") {
		md, err := extractor.ToMarkdown(contentHTML)
		if err == nil {
			result.Markdown = md
		}
	}
	return result
}

// ScrapeURL scrapes a specific URL and extracts the main content
func ScrapeURL(req *ScrapeRequest, cfg *config.Config) (*ScrapeResult, error) {
	c := colly.NewCollector(
		colly.Debugger(&debug.LogDebugger{}),
	)

	timeout := 30 * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	} else if cfg != nil && cfg.Crawler.CrawlTimeout > 0 {
		timeout = cfg.Crawler.CrawlTimeout
	}
	c.SetRequestTimeout(timeout)

	if cfg != nil {
		c.UserAgent = cfg.Crawler.UserAgent
	} else {
		c.UserAgent = "GoCrawl/1.0"
	}

	if cfg != nil && cfg.Crawler.EnableProxyRotation && len(cfg.Crawler.Proxies) > 0 {
		rps, err := proxy.RoundRobinProxySwitcher(cfg.Crawler.Proxies...)
		if err != nil {
			return nil, err
		}
		c.SetProxyFunc(rps)
	}
	if cfg != nil {
		if t := NewRetryTransport(cfg); t != nil {
			c.WithTransport(t)
		}
	}

	result := &ScrapeResult{
		Links:    []string{},
		Metadata: make(map[string]string),
	}

	c.OnHTML("html", func(e *colly.HTMLElement) {
		rawHTML, _ := e.DOM.Html()
		result.RawHTML = rawHTML

		var contentHTML string
		if userSel := UserContentSelectors(req); len(userSel) > 0 {
			for _, selector := range userSel {
				if mainContent := e.DOM.Find(selector).First(); mainContent.Length() > 0 {
					contentHTML, _ = mainContent.Html()
					break
				}
			}
			if contentHTML == "" {
				contentHTML, _ = e.DOM.Find("body").Html()
			}
		} else if req.OnlyMainContent {
			for _, selector := range MainContentSelectors() {
				if mainContent := e.DOM.Find(selector).First(); mainContent.Length() > 0 {
					contentHTML, _ = mainContent.Html()
					break
				}
			}
			if contentHTML == "" {
				contentHTML, _ = e.DOM.Find("body").Html()
			}
		} else {
			contentHTML, _ = e.DOM.Find("body").Html()
		}

		result.HTML = contentHTML

		if contains(req.Formats, "markdown") {
			markdown, err := extractor.ToMarkdown(contentHTML)
			if err == nil {
				result.Markdown = markdown
			}
		}

		result.Metadata["title"] = e.DOM.Find("title").Text()
		result.Metadata["description"] = e.DOM.Find("meta[name='description']").AttrOr("content", "")
		result.Metadata["language"] = e.DOM.Find("html").AttrOr("lang", "")
		result.Metadata["sourceURL"] = req.URL
	})

	linkSel := EffectiveLinkSelector(req)
	c.OnHTML(linkSel, func(e *colly.HTMLElement) {
		href := e.Attr("href")
		if href != "" {
			baseURL, _ := url.Parse(req.URL)
			linkURL, err := url.Parse(href)
			if err == nil {
				absoluteURL := baseURL.ResolveReference(linkURL)
				result.Links = append(result.Links, absoluteURL.String())
			}
		}
	})

	c.OnResponse(func(r *colly.Response) {
		result.Metadata["statusCode"] = strconv.Itoa(r.StatusCode)
	})

	c.OnError(func(r *colly.Response, err error) {
		result.Metadata["error"] = err.Error()
	})

	visitErr := c.Visit(req.URL)
	return finalizeScrape(req, cfg, timeout, result, visitErr)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

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
	Markdown string            `json:"markdown,omitempty"`
	HTML     string            `json:"html,omitempty"`
	RawHTML  string            `json:"rawHtml,omitempty"`
	Links    []string          `json:"links,omitempty"`
	Metadata map[string]string `json:"metadata"`
}

// ScrapeRequest represents a scrape request
type ScrapeRequest struct {
	URL string `json:"url"`
	// OnlyMainContent: omit or true = use main/article heuristics; false = full <body> (see EffectiveOnlyMainContent).
	OnlyMainContent    *bool    `json:"onlyMainContent,omitempty"`
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

// wantsFormat reports whether the client asked for this output.
// If formats is empty, all of markdown/html/rawHtml are included (legacy behavior).
func wantsFormat(req *ScrapeRequest, format string) bool {
	if req == nil || len(req.Formats) == 0 {
		return true
	}
	return contains(req.Formats, format)
}

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
	if cfg != nil && cfg.Crawler.ChromedpWSURL != "" && scrapeNeedsChromedpFallback(result, visitErr, EffectiveOnlyMainContent(req)) {
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
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"root\">" + contentHTML + "</div>"))
	if err == nil {
		root := doc.Find("#root").First()
		seen := make(map[string]struct{})
		if linkSelExplicit(req) {
			doc.Find(strings.TrimSpace(req.LinkSelector)).Each(func(_ int, s *goquery.Selection) {
				appendResolvedHref(s, req.URL, &result.Links, seen)
			})
		} else {
			root.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
				appendResolvedHref(s, req.URL, &result.Links, seen)
			})
		}
	}
	if wantsFormat(req, "markdown") {
		md, err := extractor.ToMarkdown(contentHTML)
		if err == nil {
			result.Markdown = md
		}
	}
	if wantsFormat(req, "html") {
		result.HTML = contentHTML
	}
	if wantsFormat(req, "rawHtml") {
		result.RawHTML = contentHTML
	}
	return result
}

// linkSelExplicit is true when the client set linkSelector (full-document link query).
func linkSelExplicit(req *ScrapeRequest) bool {
	return req != nil && strings.TrimSpace(req.LinkSelector) != ""
}

// pickContentHTML returns extracted HTML and the goquery subtree used (for scoped links).
func pickContentHTML(e *colly.HTMLElement, req *ScrapeRequest) (contentHTML string, scope *goquery.Selection) {
	body := e.DOM.Find("body").First()
	if len(UserContentSelectors(req)) > 0 {
		for _, selector := range UserContentSelectors(req) {
			mainContent := e.DOM.Find(selector).First()
			if mainContent.Length() > 0 {
				contentHTML, _ = mainContent.Html()
				return contentHTML, mainContent
			}
		}
		contentHTML, _ = body.Html()
		return contentHTML, body
	}
	if EffectiveOnlyMainContent(req) {
		for _, selector := range MainContentSelectors() {
			mainContent := e.DOM.Find(selector).First()
			if mainContent.Length() > 0 {
				contentHTML, _ = mainContent.Html()
				return contentHTML, mainContent
			}
		}
		contentHTML, _ = body.Html()
		return contentHTML, body
	}
	contentHTML, _ = body.Html()
	return contentHTML, body
}

func appendResolvedHref(s *goquery.Selection, pageURL string, links *[]string, seen map[string]struct{}) {
	href, _ := s.Attr("href")
	if href == "" {
		return
	}
	baseURL, _ := url.Parse(pageURL)
	linkURL, err := url.Parse(href)
	if err != nil {
		return
	}
	abs := baseURL.ResolveReference(linkURL).String()
	if _, ok := seen[abs]; ok {
		return
	}
	seen[abs] = struct{}{}
	*links = append(*links, abs)
}

func collectScrapeLinks(e *colly.HTMLElement, req *ScrapeRequest, scope *goquery.Selection, links *[]string) {
	seen := make(map[string]struct{})
	if linkSelExplicit(req) {
		e.DOM.Find(strings.TrimSpace(req.LinkSelector)).Each(func(_ int, s *goquery.Selection) {
			appendResolvedHref(s, req.URL, links, seen)
		})
		return
	}
	if scope != nil && scope.Length() > 0 {
		scope.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
			appendResolvedHref(s, req.URL, links, seen)
		})
	}
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
		if wantsFormat(req, "rawHtml") {
			rawHTML, _ := e.DOM.Html()
			result.RawHTML = rawHTML
		}

		contentHTML, scope := pickContentHTML(e, req)
		if wantsFormat(req, "html") {
			result.HTML = contentHTML
		}

		collectScrapeLinks(e, req, scope, &result.Links)

		if wantsFormat(req, "markdown") {
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

package crawler

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/extractor"
	"gocrawl/internal/llm"

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
	Summary  string            `json:"summary,omitempty"`
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
	// ExcludeSelectors are CSS selectors whose subtrees are excluded from scored extraction (webclaw-style).
	ExcludeSelectors []string `json:"excludeSelectors,omitempty"`
	// UseAdvancedExtractor: nil defaults to matching onlyMainContent; set false to use legacy selector-only extraction.
	UseAdvancedExtractor *bool `json:"useAdvancedExtractor,omitempty"`
	// ExtractJsData runs inline scripts in a sandbox (goja) and appends structured __* blob text (webclaw-style).
	ExtractJsData bool `json:"extractJsData,omitempty"`
	// ForceBrowser skips the HTTP fetch and uses chromedp only when LIGHTPANDA_WS_URL or LIGHTPANDA_HTTP_URL is set.
	ForceBrowser bool `json:"forceBrowser,omitempty"`
	// Summarize asks the optional LLM layer to add a short plain-text summary (requires LLM_* config).
	Summarize *bool `json:"summarize,omitempty"`
	// SummaryMaxSentences caps the LLM summary length (default 3).
	SummaryMaxSentences int `json:"summaryMaxSentences,omitempty"`
	// SummaryModel overrides the configured default LLM model for this request.
	SummaryModel string `json:"summaryModel,omitempty"`
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

func finalizeScrape(req *ScrapeRequest, cfg *config.Config, timeout time.Duration, result *ScrapeResult, visitErr error, pageBody []byte) (*ScrapeResult, error) {
	if cfg != nil && ChromedpConfigured(cfg) && cfg.Crawler.ChromedpAutoFallback {
		if ok, why := ShouldChromedpFallback(visitErr, result, EffectiveOnlyMainContent(req), cfg.Crawler.ChromedpFallbackStatusCodes, pageBody); ok {
			html, err := ScrapeHTMLViaChromedp(cfg, req, timeout)
			if err == nil {
				html, doc := refineChromedpHTML(html, req)
				r := buildResultFromMainHTMLWithDoc(html, req, doc)
				applyJsExtract(req, []byte(html), r)
				summarizeResult(req, cfg, r)
				r.Metadata["extractor"] = "chromedp"
				r.Metadata["chromedpTrigger"] = why
				return r, nil
			}
		}
	}
	if visitErr != nil {
		return nil, visitErr
	}
	return result, nil
}

func scrapeViaChromedpOnly(req *ScrapeRequest, cfg *config.Config, timeout time.Duration) (*ScrapeResult, error) {
	if !ChromedpConfigured(cfg) {
		return nil, fmt.Errorf("forceBrowser requires LIGHTPANDA_WS_URL or LIGHTPANDA_HTTP_URL")
	}
	html, err := ScrapeHTMLViaChromedp(cfg, req, timeout)
	if err != nil {
		return nil, err
	}
	html, doc := refineChromedpHTML(html, req)
	r := buildResultFromMainHTMLWithDoc(html, req, doc)
	applyJsExtract(req, []byte(html), r)
	summarizeResult(req, cfg, r)
	r.Metadata["extractor"] = "chromedp"
	r.Metadata["chromedpTrigger"] = "force_browser"
	return r, nil
}

func summarizeResult(req *ScrapeRequest, cfg *config.Config, result *ScrapeResult) {
	if cfg == nil || result == nil || req == nil {
		return
	}
	if strings.TrimSpace(result.Summary) != "" {
		return
	}
	if !cfg.LLM.Enabled || req.Summarize == nil || !*req.Summarize {
		return
	}
	md := result.Markdown
	if md == "" {
		return
	}
	n := req.SummaryMaxSentences
	if n <= 0 {
		n = 3
	}
	model := strings.TrimSpace(req.SummaryModel)
	if model == "" {
		model = cfg.LLM.Model
	}
	s, err := llm.SummarizeMarkdown(context.Background(), cfg, model, md, n)
	if err != nil {
		log.Printf("summarize: %v", err)
		return
	}
	result.Summary = strings.TrimSpace(s)
	if result.Metadata == nil {
		result.Metadata = make(map[string]string)
	}
	if model != "" {
		result.Metadata["summaryModel"] = model
	}
}

func buildResultFromMainHTML(contentHTML string, req *ScrapeRequest) *ScrapeResult {
	return buildResultFromMainHTMLWithDoc(contentHTML, req, nil)
}

func buildResultFromMainHTMLWithDoc(contentHTML string, req *ScrapeRequest, fullDoc *goquery.Document) *ScrapeResult {
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
			if fullDoc != nil && EffectiveUseAdvancedExtractor(req) {
				md = extractor.RecoverMarkdownH1(fullDoc, md)
			}
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

const maxPageBodyCopy = 2 << 20 // cap bytes copied for CSR/challenge heuristics

// ScrapeURL scrapes a specific URL and extracts the main content
func ScrapeURL(req *ScrapeRequest, cfg *config.Config) (*ScrapeResult, error) {
	timeout := 30 * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	} else if cfg != nil && cfg.Crawler.CrawlTimeout > 0 {
		timeout = cfg.Crawler.CrawlTimeout
	}

	if req != nil && req.ForceBrowser {
		return scrapeViaChromedpOnly(req, cfg, timeout)
	}

	c := colly.NewCollector(
		colly.Debugger(&debug.LogDebugger{}),
	)

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
		if t := TransportForCrawler(cfg); t != nil {
			c.WithTransport(t)
		}
	}

	result := &ScrapeResult{
		Links:    []string{},
		Metadata: make(map[string]string),
	}

	var pageBody []byte
	c.OnResponse(func(r *colly.Response) {
		result.Metadata["statusCode"] = strconv.Itoa(r.StatusCode)
		n := len(r.Body)
		if n > maxPageBodyCopy {
			n = maxPageBodyCopy
		}
		if n > 0 {
			pageBody = append([]byte(nil), r.Body[:n]...)
		}
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		if wantsFormat(req, "rawHtml") {
			rawHTML, _ := e.DOM.Html()
			result.RawHTML = rawHTML
		}

		contentHTML, scope, fullDoc := extractContentForScrape(e, req)
		if wantsFormat(req, "html") {
			result.HTML = contentHTML
		}

		collectScrapeLinks(e, req, scope, &result.Links)

		if wantsFormat(req, "markdown") {
			markdown, err := extractor.ToMarkdown(contentHTML)
			if err == nil {
				if fullDoc != nil && EffectiveUseAdvancedExtractor(req) {
					markdown = extractor.RecoverMarkdownH1(fullDoc, markdown)
				}
				result.Markdown = markdown
			}
		}

		result.Metadata["title"] = e.DOM.Find("title").Text()
		result.Metadata["description"] = e.DOM.Find("meta[name='description']").AttrOr("content", "")
		result.Metadata["language"] = e.DOM.Find("html").AttrOr("lang", "")
		result.Metadata["sourceURL"] = req.URL
		if e.Response != nil {
			applyJsExtract(req, e.Response.Body, result)
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		result.Metadata["error"] = err.Error()
	})

	visitErr := c.Visit(req.URL)
	res, err := finalizeScrape(req, cfg, timeout, result, visitErr, pageBody)
	summarizeResult(req, cfg, res)
	return res, err
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

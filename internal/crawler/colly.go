package crawler

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gocrawl/internal/config"
	"gocrawl/internal/extractor"
	"gocrawl/internal/llm"
	"gocrawl/internal/utils"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
	"github.com/gocolly/colly/v2/proxy"
	"golang.org/x/net/html"
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
	URL            string `json:"url"`
	PreFetchedBody []byte `json:"-"`
	// PreFetchedHeaders is the Colly/crawl response headers when PreFetchedBody is set (improves WAF detection for chromedp fallback).
	PreFetchedHeaders http.Header `json:"-"`
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

func finalizeScrape(ctx context.Context, req *ScrapeRequest, cfg *config.Config, timeout time.Duration, result *ScrapeResult, visitErr error, pageBody []byte, respHeaders http.Header) (*ScrapeResult, error) {
	if cfg != nil && ChromedpConfigured(cfg) && cfg.Crawler.ChromedpAutoFallback {
		pageURL := ""
		if req != nil {
			pageURL = req.URL
		}
		if ok, why := ShouldChromedpFallback(&FallbackCriteria{
			VisitErr:    visitErr,
			Result:      result,
			OnlyMain:    EffectiveOnlyMainContent(req),
			StatusCodes: cfg.Crawler.ChromedpFallbackStatusCodes,
			PageBody:    pageBody,
			URL:         pageURL,
			Headers:     respHeaders,
		}); ok {
			html, err := ScrapeHTMLViaChromedp(cfg, req, timeout)
			if err == nil {
				html, doc := refineChromedpHTML(html, req)
				r := buildResultFromMainHTMLWithDoc(html, req, doc)
				applyJsExtract(req, []byte(html), r)
				summarizeResult(ctx, req, cfg, r)
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

func scrapeViaChromedpOnly(ctx context.Context, req *ScrapeRequest, cfg *config.Config, timeout time.Duration) (*ScrapeResult, error) {
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
	summarizeResult(ctx, req, cfg, r)
	r.Metadata["extractor"] = "chromedp"
	r.Metadata["chromedpTrigger"] = "force_browser"
	return r, nil
}

func summarizeResult(ctx context.Context, req *ScrapeRequest, cfg *config.Config, result *ScrapeResult) {
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
	if ctx == nil {
		ctx = context.Background()
	}
	s, err := llm.SummarizeMarkdown(ctx, cfg, model, md, n)
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
		baseURL, _ := url.Parse(req.URL)
		if linkSelExplicit(req) {
			doc.Find(strings.TrimSpace(req.LinkSelector)).Each(func(_ int, s *goquery.Selection) {
				if href, ok := s.Attr("href"); ok {
					appendResolvedHref(href, baseURL, &result.Links, seen)
				}
			})
		} else {
			// ⚡ Bolt Optimization: Manually traverse x/net/html tree
			// to avoid goquery.Find multi-selector allocation overhead.
			if root.Length() > 0 {
				traverseAndCollectAnchors(root.Nodes, baseURL, &result.Links, seen)
			}
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

func appendResolvedHref(href string, baseURL *url.URL, links *[]string, seen map[string]struct{}) {
	if href == "" || baseURL == nil {
		return
	}

	// ⚡ Bolt Optimization: Use zero-allocation fast-path resolution
	// for absolute and root-relative URLs.
	abs := utils.ResolveHref(baseURL, href)
	if abs == "" {
		return
	}

	if _, ok := seen[abs]; ok {
		return
	}
	seen[abs] = struct{}{}
	*links = append(*links, abs)
}

func traverseAndCollectAnchors(nodes []*html.Node, baseURL *url.URL, links *[]string, seen map[string]struct{}) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					appendResolvedHref(a.Val, baseURL, links, seen)
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
}

func collectScrapeLinks(e *colly.HTMLElement, req *ScrapeRequest, scope *goquery.Selection, links *[]string) {
	seen := make(map[string]struct{})
	baseURL, _ := url.Parse(req.URL)
	if linkSelExplicit(req) {
		e.DOM.Find(strings.TrimSpace(req.LinkSelector)).Each(func(_ int, s *goquery.Selection) {
			if href, ok := s.Attr("href"); ok {
				appendResolvedHref(href, baseURL, links, seen)
			}
		})
		return
	}
	if scope != nil && scope.Length() > 0 {
		// ⚡ Bolt Optimization: Manually traverse x/net/html tree
		// to avoid goquery.Find multi-selector allocation overhead.
		traverseAndCollectAnchors(scope.Nodes, baseURL, links, seen)
	}
}

const maxPageBodyCopy = 2 << 20 // cap bytes copied for CSR/challenge heuristics

// ScrapeURL scrapes a specific URL and extracts the main content
func ScrapeURL(req *ScrapeRequest, cfg *config.Config) (*ScrapeResult, error) {
	return ScrapeURLWithContext(context.Background(), req, cfg)
}

func ScrapeURLWithContext(ctx context.Context, req *ScrapeRequest, cfg *config.Config) (*ScrapeResult, error) {
	timeout := 30 * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	} else if cfg != nil && cfg.Crawler.CrawlTimeout > 0 {
		timeout = cfg.Crawler.CrawlTimeout
	}

	if req != nil && req.ForceBrowser {
		return scrapeViaChromedpOnly(ctx, req, cfg, timeout)
	}

	if req != nil && len(req.PreFetchedBody) > 0 {
		result := &ScrapeResult{
			Links:    []string{},
			Metadata: make(map[string]string),
		}

		// Build a fake response and HTMLElement to reuse the logic
		res := &colly.Response{
			Body: req.PreFetchedBody,
			Request: &colly.Request{
				URL: mustParseURL(req.URL),
			},
		}

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(req.PreFetchedBody))
		if err == nil {
			e := &colly.HTMLElement{
				DOM:      doc.Selection,
				Response: res,
				Request:  res.Request,
			}
			populateScrapeResultFromBody(e, req, result)
		}

		return finalizeScrape(ctx, req, cfg, timeout, result, nil, req.PreFetchedBody, req.PreFetchedHeaders)
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

	if t := TransportForCrawler(cfg); t != nil {
		c.WithTransport(t)
	}

	if cfg != nil && cfg.Crawler.EnableProxyRotation && len(cfg.Crawler.Proxies) > 0 {
		rps, err := proxy.RoundRobinProxySwitcher(cfg.Crawler.Proxies...)
		if err != nil {
			return nil, err
		}

		// Wrap proxy to ensure it resolves and passes SSRF check
		c.SetProxyFunc(func(req *http.Request) (*url.URL, error) {
			p, err := rps(req)
			if err != nil {
				return nil, err
			}
			if p != nil {
				// The safe dialer will still check the proxy IP during DialContext.
			}
			return p, nil
		})
	}

	result := &ScrapeResult{
		Links:    []string{},
		Metadata: make(map[string]string),
	}

	var pageBody []byte
	var pageHeaders http.Header
	c.OnResponse(func(r *colly.Response) {
		result.Metadata["statusCode"] = strconv.Itoa(r.StatusCode)
		if r.Headers != nil {
			pageHeaders = r.Headers.Clone()
		}
		n := len(r.Body)
		if n > maxPageBodyCopy {
			n = maxPageBodyCopy
		}
		if n > 0 {
			pageBody = append([]byte(nil), r.Body[:n]...)
		}
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		populateScrapeResultFromBody(e, req, result)
	})

	c.OnError(func(r *colly.Response, err error) {
		result.Metadata["error"] = err.Error()
	})

	visitErr := c.Visit(req.URL)
	res, err := finalizeScrape(ctx, req, cfg, timeout, result, visitErr, pageBody, pageHeaders)
	summarizeResult(ctx, req, cfg, res)
	return res, err
}

func populateScrapeResultFromBody(e *colly.HTMLElement, req *ScrapeRequest, result *ScrapeResult) {
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

	title, desc, lang := extractMetadataFast(e.DOM.Nodes)
	result.Metadata["title"] = title
	result.Metadata["description"] = desc
	result.Metadata["language"] = lang
	result.Metadata["sourceURL"] = req.URL
	if e.Response != nil {
		applyJsExtract(req, e.Response.Body, result)
	}
}

func mustParseURL(u string) *url.URL {
	pu, _ := url.Parse(u)
	return pu
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// extractMetadataFast manually traverses the x/net/html tree to extract metadata
// without allocating goquery Selection objects. It stops traversing when it hits
// the <body> tag to significantly speed up processing on large documents.
func extractMetadataFast(nodes []*html.Node) (title, desc, lang string) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if title != "" && desc != "" && lang != "" {
			return
		}
		if n.Type == html.ElementNode {
			if n.Data == "body" {
				return
			}
			if title == "" && n.Data == "title" {
				var buf strings.Builder
				var extractText func(*html.Node)
				extractText = func(nn *html.Node) {
					if nn.Type == html.TextNode {
						buf.WriteString(nn.Data)
					}
					for c := nn.FirstChild; c != nil; c = c.NextSibling {
						extractText(c)
					}
				}
				extractText(n)
				title = buf.String()
			} else if desc == "" && n.Data == "meta" {
				var isDesc bool
				var content string
				for _, a := range n.Attr {
					if a.Key == "name" && strings.EqualFold(a.Val, "description") {
						isDesc = true
					} else if a.Key == "content" {
						content = a.Val
					}
				}
				if isDesc {
					desc = content
				}
			} else if lang == "" && n.Data == "html" {
				for _, a := range n.Attr {
					if a.Key == "lang" {
						lang = a.Val
						break
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return title, desc, lang
}

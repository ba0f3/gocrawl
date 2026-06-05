package crawler

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	isantibot "github.com/ba0f3/is-antibot-go"
	"gocrawl/internal/utils"
	"golang.org/x/net/html"
)

const chromedpHTMLScanMax = 256 * 1024

const spaMountMaxTextRunes = 120
const spaMountMinInnerHTML = 400
const spaFrameworkBodyMaxText = 150
const spaFrameworkMinDocBytes = 2500

// DefaultChromedpFallbackStatusCodes is returned when config does not override the list.
func DefaultChromedpFallbackStatusCodes() []int {
	return []int{401, 403, 429, 503}
}

// challengeHTMLMarkers are matched case-insensitively against a prefix of the HTML sample.
var challengeHTMLMarkers = []string{
	"cf-browser-verification",
	"cloudflare",
	"checking your browser",
	"turnstile",
	"hcaptcha",
	"recaptcha",
	"verify you are human",
	"attention required",
	"captcha",
	"bot detection",
	"access denied",
	"just a moment",
	"ddos protection",
	"ray id",
}

// FallbackCriteria encapsulates the input for ShouldChromedpFallback heuristics.
type FallbackCriteria struct {
	VisitErr    error
	Result      *ScrapeResult
	OnlyMain    bool
	StatusCodes []int
	PageBody    []byte
	URL         string
	Headers     http.Header
}

// ShouldChromedpFallback reports whether automatic chromedp retry is warranted and a short tag for metadata.
func ShouldChromedpFallback(fc *FallbackCriteria) (bool, string) {
	if fc.VisitErr != nil {
		return true, "visit_error"
	}
	if fc.Result == nil {
		return true, "nil_result"
	}
	if errMsg := fc.Result.Metadata["error"]; errMsg != "" {
		return true, "colly_error"
	}
	codes := fc.StatusCodes
	if len(codes) == 0 {
		codes = DefaultChromedpFallbackStatusCodes()
	}
	if codeStr := fc.Result.Metadata["statusCode"]; codeStr != "" {
		if code, err := strconv.Atoi(codeStr); err == nil {
			for _, c := range codes {
				if c == code {
					return true, "status_" + codeStr
				}
			}
		}
	}
	if yes, tag := antibotWAFTrigger(fc); yes {
		return true, tag
	}
	sample := htmlSampleForHeuristics(fc.Result, fc.PageBody)
	if looksLikeChallengePage(sample) {
		return true, "challenge_html"
	}
	htmlForCSR := string(fc.PageBody)
	if htmlForCSR == "" {
		htmlForCSR = sample
	}
	if tag, ok := detectCSRFrameworkOrSPAShell(htmlForCSR); ok {
		return true, tag
	}
	if fc.OnlyMain && len([]rune(strings.TrimSpace(fc.Result.Markdown))) < minMainMarkdownRunes {
		return true, "thin_markdown"
	}
	return false, ""
}

// antibotWAFTrigger uses github.com/ba0f3/is-antibot-go to detect WAF / bot-challenge responses
// so chromedp can retry when Colly received a block or interstitial page with a normal HTTP status.
func antibotWAFTrigger(fc *FallbackCriteria) (bool, string) {
	if fc == nil {
		return false, ""
	}
	html := htmlSampleForHeuristics(fc.Result, fc.PageBody)
	status := 0
	if fc.Result != nil {
		if s := fc.Result.Metadata["statusCode"]; s != "" {
			status, _ = strconv.Atoi(s)
		}
	}
	out := isantibot.Detect(isantibot.Input{
		Headers:    fc.Headers,
		HTML:       html,
		URL:        fc.URL,
		StatusCode: status,
	})
	if !out.Detected {
		return false, ""
	}
	prov := "unknown"
	if out.Provider != nil && *out.Provider != "" {
		prov = *out.Provider
	}
	return true, "antibot_" + prov
}

func htmlSampleForHeuristics(result *ScrapeResult, pageBody []byte) string {
	if len(pageBody) > 0 {
		s := string(pageBody)
		if len(s) > chromedpHTMLScanMax {
			return s[:chromedpHTMLScanMax]
		}
		return s
	}
	if result == nil {
		return ""
	}
	if result.RawHTML != "" {
		s := result.RawHTML
		if len(s) > chromedpHTMLScanMax {
			return s[:chromedpHTMLScanMax]
		}
		return s
	}
	if result.HTML != "" {
		s := result.HTML
		if len(s) > chromedpHTMLScanMax {
			return s[:chromedpHTMLScanMax]
		}
		return s
	}
	return ""
}

func looksLikeChallengePage(sample string) bool {
	if sample == "" {
		return false
	}
	// ⚡ Bolt Optimization: Zero-allocation string scanning instead of strings.ToLower + strings.Contains
	return utils.HasAnyLowercasePattern(sample, challengeHTMLMarkers)
}

// spaFrameworkHTMLMarkers are lowercase substrings typical of CSR shells (React, Next, Vue, Nuxt, Angular, SvelteKit, Remix, Astro, Vite).
var spaFrameworkHTMLMarkers = []string{
	// Next.js / React (SSR payload, RSC, chunks)
	"__next_data__",
	"__next_f",
	"_next/static",
	// Nuxt / Vue
	"__nuxt__",
	"__nuxt_data__",
	"data-v-", // Vue scoped styles
	// React DOM
	"data-reactroot",
	"data-react-helmet",
	"react-dom.production",
	"react-dom.development",
	"react-dom@",
	// Angular
	"ng-version",
	" ng-app=",
	// SvelteKit / Svelte
	"__sveltekit",
	"data-sveltekit",
	// Remix
	"__remix",
	// Astro islands
	"astro-island",
	// Vite client (common for Vue/React SPAs)
	"/@vite/",
	"vite/client",
}

func hasUICSRFrameworkMarker(sample string) bool {
	// ⚡ Bolt Optimization: Zero-allocation string scanning instead of strings.ToLower + strings.Contains
	return utils.HasAnyLowercasePattern(sample, spaFrameworkHTMLMarkers)
}

// detectCSRFrameworkOrSPAShell returns a metadata tag and true when the HTML looks like a UI-framework CSR shell
// (React, Next.js, Vue, Nuxt, Angular, SvelteKit, Remix, Astro, Vite, etc.).
func detectCSRFrameworkOrSPAShell(htmlStr string) (tag string, ok bool) {
	if htmlStr == "" {
		return "", false
	}
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", false
	}

	var rootNode *html.Node
	var bodyNode *html.Node

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "body" {
				bodyNode = n
			}
			if rootNode == nil {
				for _, attr := range n.Attr {
					if attr.Key == "id" && (attr.Val == "root" || attr.Val == "__next" || attr.Val == "__nuxt" || attr.Val == "app") {
						rootNode = n
					} else if attr.Key == "data-reactroot" || attr.Key == "data-ng-app" || attr.Key == "ng-version" {
						rootNode = n
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if rootNode != nil {
		var textRunes int
		var innerHTMLBytes int

		var walkChildren func(*html.Node)
		walkChildren = func(n *html.Node) {
			if n.Type == html.TextNode {
				textRunes += utf8.RuneCountInString(strings.TrimSpace(n.Data))
				innerHTMLBytes += len(n.Data)
			} else if n.Type == html.ElementNode {
				// Estimate tag length to approximate innerHTML size without rendering
				innerHTMLBytes += len(n.Data)*2 + 5 // <tag></tag>
				for _, a := range n.Attr {
					innerHTMLBytes += len(a.Key) + len(a.Val) + 4 //  key="val"
				}
			} else if n.Type == html.CommentNode {
				innerHTMLBytes += len(n.Data) + 7 // <!-- -->
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walkChildren(c)
			}
		}

		for c := rootNode.FirstChild; c != nil; c = c.NextSibling {
			walkChildren(c)
		}

		if textRunes <= spaMountMaxTextRunes && innerHTMLBytes >= spaMountMinInnerHTML {
			return "spa_shell", true
		}
	}

	sample := htmlStr
	if len(htmlStr) > chromedpHTMLScanMax {
		sample = htmlStr[:chromedpHTMLScanMax]
	}
	if !hasUICSRFrameworkMarker(sample) {
		return "", false
	}

	if bodyNode == nil {
		if len(htmlStr) >= spaFrameworkMinDocBytes {
			return "csr_framework", true
		}
		return "", false
	}

	var bodyTextRunes int
	var walkBodyText func(*html.Node)
	walkBodyText = func(n *html.Node) {
		if n.Type == html.TextNode {
			bodyTextRunes += utf8.RuneCountInString(strings.TrimSpace(n.Data))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkBodyText(c)
		}
	}
	walkBodyText(bodyNode)

	if bodyTextRunes < spaFrameworkBodyMaxText && len(htmlStr) >= spaFrameworkMinDocBytes {
		return "csr_framework", true
	}

	return "", false
}

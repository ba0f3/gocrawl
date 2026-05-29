package crawler

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	isantibot "github.com/ba0f3/is-antibot-go"
	"gocrawl/internal/utils"
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
func detectCSRFrameworkOrSPAShell(html string) (tag string, ok bool) {
	if html == "" {
		return "", false
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", false
	}
	for _, sel := range []string{
		"#root", "#__next", "#__nuxt", "#app",
		"[data-reactroot]", "[data-ng-app]", "[ng-version]",
	} {
		n := doc.Find(sel).First()
		if n.Length() == 0 {
			continue
		}
		inner, _ := n.Html()
		text := strings.TrimSpace(n.Text())
		tr := utf8.RuneCountInString(text)
		if tr <= spaMountMaxTextRunes && len(inner) >= spaMountMinInnerHTML {
			return "spa_shell", true
		}
	}
	sample := html
	if len(html) > chromedpHTMLScanMax {
		sample = html[:chromedpHTMLScanMax]
	}
	if !hasUICSRFrameworkMarker(sample) {
		return "", false
	}
	body := doc.Find("body").First()
	if body.Length() == 0 {
		if len(html) >= spaFrameworkMinDocBytes {
			return "csr_framework", true
		}
		return "", false
	}
	bt := utf8.RuneCountInString(strings.TrimSpace(body.Text()))
	if bt < spaFrameworkBodyMaxText && len(html) >= spaFrameworkMinDocBytes {
		return "csr_framework", true
	}
	return "", false
}

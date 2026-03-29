package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"gocrawl/internal/config"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

type devtoolsVersion struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

var (
	wsURLMu    sync.RWMutex
	wsURLCache = map[string]string{}

	chromedpLimiterOnce sync.Once
	chromedpAcquireSlot func()
	chromedpReleaseSlot func()
)

func fetchWebSocketDebuggerURL(ctx context.Context, httpBase string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(httpBase), "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/json/version", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var v devtoolsVersion
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	ws := strings.TrimSpace(v.WebSocketDebuggerURL)
	if ws == "" {
		return "", fmt.Errorf("empty webSocketDebuggerUrl in /json/version response")
	}
	return ws, nil
}

func effectiveChromedpWSURL(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("chromedp: no config")
	}
	if u := strings.TrimSpace(cfg.Crawler.ChromedpWSURL); u != "" {
		return u, nil
	}
	base := strings.TrimSpace(cfg.Crawler.LightpandaHTTPURL)
	if base == "" {
		return "", fmt.Errorf("chromedp: set LIGHTPANDA_WS_URL or LIGHTPANDA_HTTP_URL")
	}
	wsURLMu.RLock()
	cached := wsURLCache[base]
	wsURLMu.RUnlock()
	if cached != "" {
		return cached, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ws, err := fetchWebSocketDebuggerURL(ctx, base)
	if err != nil {
		return "", err
	}
	wsURLMu.Lock()
	wsURLCache[base] = ws
	wsURLMu.Unlock()
	return ws, nil
}

func ensureChromedpLimiter(cfg *config.Config) {
	n := 8
	if cfg != nil && cfg.Crawler.ChromedpMaxConcurrent > 0 {
		n = cfg.Crawler.ChromedpMaxConcurrent
	}
	chromedpLimiterOnce.Do(func() {
		tokens := make(chan struct{}, n)
		for i := 0; i < n; i++ {
			tokens <- struct{}{}
		}
		chromedpAcquireSlot = func() { <-tokens }
		chromedpReleaseSlot = func() { tokens <- struct{}{} }
	})
}

// ChromedpConfigured reports whether a WebSocket URL or resolvable HTTP base is set.
func ChromedpConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.Crawler.ChromedpWSURL) != "" || strings.TrimSpace(cfg.Crawler.LightpandaHTTPURL) != ""
}

// ScrapeHTMLViaChromedp loads req.URL using a remote CDP endpoint (e.g. Lightpanda) and returns HTML via JS (EffectiveContentSelectors).
func ScrapeHTMLViaChromedp(cfg *config.Config, req *ScrapeRequest, timeout time.Duration) (html string, err error) {
	if !ChromedpConfigured(cfg) {
		return "", fmt.Errorf("chromedp: LIGHTPANDA_WS_URL or LIGHTPANDA_HTTP_URL not configured")
	}
	if req == nil || req.URL == "" {
		return "", fmt.Errorf("chromedp: request URL required")
	}
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	ws, err := effectiveChromedpWSURL(cfg)
	if err != nil {
		return "", err
	}
	ensureChromedpLimiter(cfg)
	chromedpAcquireSlot()
	defer chromedpReleaseSlot()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, ws)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	ua := strings.TrimSpace(cfg.Crawler.UserAgent)
	if ua == "" {
		ua = "GoCrawl/1.0"
	}
	sels := EffectiveContentSelectors(req)
	lenJS := SelectorsMaxTextLenJS(sels)
	extractJS := SelectorsJS(sels)
	minText := cfg.Crawler.ChromedpHydrationMinTextRunes
	maxPolls := cfg.Crawler.ChromedpHydrationMaxPolls
	pollEvery := cfg.Crawler.ChromedpHydrationPollEvery
	navWait := cfg.Crawler.ChromedpNavWait
	loadWait := cfg.Crawler.ChromedpLoadWaitTimeout
	if loadWait <= 0 {
		loadWait = 30 * time.Second
	}
	if timeout > 8*time.Second && loadWait > timeout-4*time.Second {
		loadWait = timeout - 4*time.Second
		if loadWait < 5*time.Second {
			loadWait = 5 * time.Second
		}
	}

	err = chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return emulation.SetUserAgentOverride(ua).Do(ctx)
		}),
		chromedp.Navigate(req.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if err != nil {
		return "", err
	}
	// Short CDP round-trips only: Lightpanda (and similar) often time out a single long Poll/AwaitPromise call.
	// SPAs often stay on "interactive" forever (XHRs, streaming) and never reach "complete"; waiting only for
	// "complete" can spin until loadWait and hit Lightpanda's ~10s CDP/session limits after fetches finish.
	deadline := time.Now().Add(loadWait)
	var stableInteractive int
	for time.Now().Before(deadline) {
		var rs string
		err = chromedp.Run(browserCtx, chromedp.Evaluate(`document.readyState`, &rs))
		if err != nil {
			return "", err
		}
		if rs == "complete" {
			break
		}
		if rs == "interactive" {
			stableInteractive++
			if stableInteractive >= 2 {
				break
			}
		} else {
			stableInteractive = 0
		}
		if err = chromedp.Run(browserCtx, chromedp.Sleep(80*time.Millisecond)); err != nil {
			return "", err
		}
	}
	if err = cdpPacedWait(browserCtx, navWait); err != nil {
		return "", err
	}
	// Brief settle; paced so the CDP session does not sit idle (Lightpanda "CDP timeout" after fetches).
	if err = cdpPacedWait(browserCtx, 120*time.Millisecond); err != nil {
		return "", err
	}
	for i := 0; i < maxPolls; i++ {
		var textLen float64
		err = chromedp.Run(browserCtx, chromedp.Evaluate(lenJS, &textLen))
		if err != nil {
			return "", err
		}
		if int(textLen) >= minText {
			break
		}
		if i < maxPolls-1 && pollEvery > 0 {
			if err = cdpPacedWait(browserCtx, pollEvery); err != nil {
				return "", err
			}
		}
	}
	var out string
	err = chromedp.Run(browserCtx, chromedp.Evaluate(extractJS, &out))
	if err != nil {
		return "", err
	}
	return out, nil
}

const cdpPacedStride = 180 * time.Millisecond

// cdpPacedWait spreads wall-clock wait across short CDP evaluations so remotes (e.g. Lightpanda) that
// time out idle WebSocket sessions still see traffic during long post-load / hydration delays.
func cdpPacedWait(browserCtx context.Context, total time.Duration) error {
	if total <= 0 {
		return nil
	}
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		var noop int
		if err := chromedp.Run(browserCtx, chromedp.Evaluate(`0`, &noop)); err != nil {
			return err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil
		}
		step := cdpPacedStride
		if remaining < step {
			step = remaining
		}
		time.Sleep(step)
	}
	return nil
}

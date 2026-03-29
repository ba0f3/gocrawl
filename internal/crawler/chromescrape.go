package crawler

import (
	"context"
	"fmt"
	"time"

	"gocrawl/internal/config"

	"github.com/chromedp/chromedp"
)

// ScrapeHTMLViaChromedp loads pageURL using a remote CDP endpoint (e.g. Lightpanda) and returns main-like HTML via JS selectors.
func ScrapeHTMLViaChromedp(cfg *config.Config, pageURL string, timeout time.Duration) (html string, err error) {
	if cfg == nil || cfg.Crawler.ChromedpWSURL == "" {
		return "", fmt.Errorf("chromedp: LIGHTPANDA_WS_URL not configured")
	}
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, cfg.Crawler.ChromedpWSURL)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	js := MainContentSelectorsJS()
	var out string
	err = chromedp.Run(browserCtx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(js, &out),
	)
	if err != nil {
		return "", err
	}
	return out, nil
}

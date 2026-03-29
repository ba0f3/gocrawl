package crawler

import (
	"context"
	"fmt"
	"time"

	"gocrawl/internal/config"

	"github.com/chromedp/chromedp"
)

// ScrapeHTMLViaChromedp loads req.URL using a remote CDP endpoint (e.g. Lightpanda) and returns HTML via JS (EffectiveContentSelectors).
func ScrapeHTMLViaChromedp(cfg *config.Config, req *ScrapeRequest, timeout time.Duration) (html string, err error) {
	if cfg == nil || cfg.Crawler.ChromedpWSURL == "" {
		return "", fmt.Errorf("chromedp: LIGHTPANDA_WS_URL not configured")
	}
	if req == nil || req.URL == "" {
		return "", fmt.Errorf("chromedp: request URL required")
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

	js := SelectorsJS(EffectiveContentSelectors(req))
	var out string
	err = chromedp.Run(browserCtx,
		chromedp.Navigate(req.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(js, &out),
	)
	if err != nil {
		return "", err
	}
	return out, nil
}

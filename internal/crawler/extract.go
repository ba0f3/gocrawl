package crawler

import (
	"bytes"
	"strconv"
	"strings"

	"gocrawl/internal/extractor"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// applyJsExtract appends JS-derived text from inline scripts when extractJsData is set.
func applyJsExtract(req *ScrapeRequest, body []byte, result *ScrapeResult) {
	if req == nil || !req.ExtractJsData || len(body) == 0 || result == nil {
		return
	}
	blobs := extractor.ExtractJsDataFromHTML(string(body))
	extra := extractor.ExtractReadableTextFromBlobs(blobs)
	if extra == "" {
		return
	}
	if result.Markdown != "" {
		result.Markdown += "\n\n" + extra
	} else {
		result.Markdown = extra
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]string)
	}
	result.Metadata["jsExtracted"] = "true"
	result.Metadata["jsBlobCount"] = strconv.Itoa(len(blobs))
}

// extractContentForScrape returns HTML for markdown/links. When webclaw-style extraction applies,
// fullDoc is the parsed page for RecoverMarkdownH1.
func extractContentForScrape(e *colly.HTMLElement, req *ScrapeRequest) (contentHTML string, scope *goquery.Selection, fullDoc *goquery.Document) {
	if e == nil || req == nil {
		return "", nil, nil
	}
	if !EffectiveUseAdvancedExtractor(req) || e.Response == nil || len(e.Response.Body) == 0 {
		ch, sc := pickContentHTML(e, req)
		return ch, sc, nil
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(e.Response.Body))
	if err != nil {
		ch, sc := pickContentHTML(e, req)
		return ch, sc, nil
	}
	opts := &extractor.ExtractionOptions{
		ExcludeSelectors: req.ExcludeSelectors,
		IncludeSelectors: UserContentSelectors(req),
		OnlyMainContent:  EffectiveOnlyMainContent(req),
	}
	fragment, err := extractor.ExtractMainHTML(doc, req.URL, opts)
	if err != nil || strings.TrimSpace(fragment) == "" {
		ch, sc := pickContentHTML(e, req)
		return ch, sc, doc
	}
	fragDoc, err := goquery.NewDocumentFromReader(strings.NewReader(`<div id="gocrawl-extract">` + fragment + `</div>`))
	if err != nil {
		ch, sc := pickContentHTML(e, req)
		return ch, sc, doc
	}
	sel := fragDoc.Find("#gocrawl-extract").First()
	return fragment, sel, doc
}

// refineChromedpHTML re-runs extraction on HTML returned from the browser when advanced extraction is enabled.
func refineChromedpHTML(html string, req *ScrapeRequest) (string, *goquery.Document) {
	if req == nil || !EffectiveUseAdvancedExtractor(req) {
		return html, nil
	}
	wrapped := "<html><head></head><body>" + html + "</body></html>"
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return html, nil
	}
	opts := &extractor.ExtractionOptions{
		ExcludeSelectors: req.ExcludeSelectors,
		IncludeSelectors: UserContentSelectors(req),
		OnlyMainContent:  EffectiveOnlyMainContent(req),
	}
	fragment, err := extractor.ExtractMainHTML(doc, req.URL, opts)
	if err != nil || strings.TrimSpace(fragment) == "" {
		return html, doc
	}
	return fragment, doc
}

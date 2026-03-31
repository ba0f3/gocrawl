package extractor

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var mainOnlySelector = "article, main, [role='main']"

// ExtractMainHTML returns inner HTML of the best main-content subtree using webclaw-style logic.
func ExtractMainHTML(doc *goquery.Document, pageURL string, opts *ExtractionOptions) (html string, err error) {
	if doc == nil {
		return "", nil
	}
	if opts == nil {
		opts = &ExtractionOptions{OnlyMainContent: true}
	}
	var base *url.URL
	if pageURL != "" {
		if u, err := url.Parse(pageURL); err == nil {
			base = u
		}
	}
	_ = base

	ex := buildExcludeSet(doc, opts.ExcludeSelectors)

	if len(opts.IncludeSelectors) > 0 {
		var parts []string
		for _, sel := range opts.IncludeSelectors {
			sel = strings.TrimSpace(sel)
			if sel == "" {
				continue
			}
			doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
				if h, e := s.Html(); e == nil && strings.TrimSpace(h) != "" {
					parts = append(parts, h)
				}
			})
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n"), nil
		}
	}

	if opts.OnlyMainContent {
		if sel := doc.Find(mainOnlySelector).First(); sel.Length() > 0 {
			if h, e := sel.Html(); e == nil {
				return h, nil
			}
		}
	}

	if best := findBestCandidate(doc, ex); best != nil {
		if h, e := best.Html(); e == nil {
			return h, nil
		}
	}

	body := doc.Find("body").First()
	if body.Length() > 0 {
		if h, e := body.Html(); e == nil {
			return h, nil
		}
	}
	if h, e := doc.Html(); e == nil {
		return h, nil
	}
	return "", nil
}

// RecoverMarkdownH1 prepends an H1 from the document when the title text is missing from the markdown.
func RecoverMarkdownH1(doc *goquery.Document, md string) string {
	if doc == nil {
		return md
	}
	h1 := doc.Find("h1").First()
	if h1.Length() == 0 {
		return md
	}
	title := strings.TrimSpace(h1.Text())
	if title == "" {
		return md
	}
	if strings.Contains(md, title) {
		return md
	}
	return "# " + title + "\n\n" + md
}

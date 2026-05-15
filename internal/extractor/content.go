package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// ExtractMainHTML returns inner HTML of the best main-content subtree using webclaw-style logic.
func ExtractMainHTML(doc *goquery.Document, _ string, opts *ExtractionOptions) (htmlRes string, err error) {
	if doc == nil {
		return "", nil
	}
	if opts == nil {
		opts = &ExtractionOptions{OnlyMainContent: true}
	}
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
		// ⚡ Bolt Optimization: Manually traverse x/net/html tree
		// to avoid goquery.Find multi-selector allocation overhead.
		var firstMain *goquery.Selection
		var walkMain func(*html.Node)
		walkMain = func(n *html.Node) {
			if firstMain != nil {
				return
			}
			if n.Type == html.ElementNode {
				tag := n.Data
				isMain := tag == "article" || tag == "main"
				if !isMain {
					for _, a := range n.Attr {
						if strings.EqualFold(a.Key, "role") && strings.EqualFold(a.Val, "main") {
							isMain = true
							break
						}
					}
				}
				if isMain {
					firstMain = &goquery.Selection{Nodes: []*html.Node{n}}
					return
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walkMain(c)
			}
		}
		for _, n := range doc.Nodes {
			walkMain(n)
		}

		if firstMain != nil {
			if h, e := firstMain.Html(); e == nil {
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

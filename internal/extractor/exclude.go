package extractor

import (
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const maxExcludeSelectors = 100

func buildExcludeSet(doc *goquery.Document, selectors []string) map[*html.Node]struct{} {
	ex := make(map[*html.Node]struct{})
	if len(selectors) > maxExcludeSelectors {
		selectors = selectors[:maxExcludeSelectors]
	}
	for _, sel := range selectors {
		sel = trim(sel)
		if sel == "" {
			continue
		}
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			addSubtreeNodes(ex, s)
		})
	}
	return ex
}

func addSubtreeNodes(ex map[*html.Node]struct{}, root *goquery.Selection) {
	if root == nil || root.Length() == 0 {
		return
	}
	root.Union(root.Find("*")).Each(func(_ int, s *goquery.Selection) {
		if n := s.Get(0); n != nil {
			ex[n] = struct{}{}
		}
	})
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func isExcluded(n *html.Node, ex map[*html.Node]struct{}) bool {
	if n == nil {
		return false
	}
	_, ok := ex[n]
	return ok
}

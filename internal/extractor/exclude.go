package extractor

import (
	"strings"

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
		sel = strings.TrimSpace(sel)
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
	// ⚡ Bolt Optimization: Use manual x/net/html tree traversal
	// to bypass massive slice/struct allocations from root.Find("*") and root.Union().
	for _, n := range root.Nodes {
		traverseAndAddExcluded(ex, n)
	}
}

func traverseAndAddExcluded(ex map[*html.Node]struct{}, n *html.Node) {
	if n == nil {
		return
	}
	if n.Type == html.ElementNode {
		ex[n] = struct{}{}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		traverseAndAddExcluded(ex, c)
	}
}

func isExcluded(n *html.Node, ex map[*html.Node]struct{}) bool {
	if n == nil {
		return false
	}
	_, ok := ex[n]
	return ok
}

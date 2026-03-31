package extractor

import (
	"math"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var candidateSelector = "article, main, [role='main'], div, section, td"

func findBestCandidate(doc *goquery.Document, exclude map[*html.Node]struct{}) *goquery.Selection {
	var best *goquery.Selection
	var bestScore float64

	doc.Find(candidateSelector).Each(func(_ int, s *goquery.Selection) {
		n := s.Get(0)
		if n == nil || isExcluded(n, exclude) {
			return
		}
		if IsNoise(s) || hasNoiseAncestor(s) || isUnderExcluded(s, exclude) {
			return
		}
		sc := scoreNode(s)
		if sc > 0 && (best == nil || sc > bestScore) {
			best = s
			bestScore = sc
		}
	})
	return best
}

func hasNoiseAncestor(sel *goquery.Selection) bool {
	for p := sel.Parent(); p.Length() > 0; p = p.Parent() {
		if IsNoise(p) {
			return true
		}
	}
	return false
}

func isUnderExcluded(sel *goquery.Selection, exclude map[*html.Node]struct{}) bool {
	for cur := sel; cur.Length() > 0; cur = cur.Parent() {
		if n := cur.Get(0); n != nil && isExcluded(n, exclude) {
			return true
		}
	}
	return false
}

func scoreNode(sel *goquery.Selection) float64 {
	if sel.Length() == 0 {
		return 0
	}
	text := strings.TrimSpace(sel.Text())
	textLen := float64(len(text))
	if textLen < 50 {
		return 0
	}
	score := math.Log(textLen)
	n := sel.Get(0)
	tag := ""
	if n != nil && n.Type == html.ElementNode {
		tag = n.Data
	}
	switch tag {
	case "article", "main":
		score += 50
	}
	if role, _ := sel.Attr("role"); strings.TrimSpace(role) == "main" {
		score += 50
	}
	if class, ok := sel.Attr("class"); ok {
		cl := strings.ToLower(class)
		if strings.Contains(cl, "content") || strings.Contains(cl, "article") ||
			strings.Contains(cl, "post") || strings.Contains(cl, "entry") {
			score += 25
		}
	}
	if id, ok := sel.Attr("id"); ok {
		idl := strings.ToLower(id)
		if strings.Contains(idl, "content") || strings.Contains(idl, "article") ||
			strings.Contains(idl, "post") || strings.Contains(idl, "main") {
			score += 25
		}
	}
	pCount := float64(sel.Find("p").Length())
	score += pCount * 3

	linkTextLen := 0
	sel.Find("a").Each(func(_ int, a *goquery.Selection) {
		linkTextLen += len(strings.TrimSpace(a.Text()))
	})
	ltf := float64(linkTextLen)
	isSemantic := tag == "article" || tag == "main"
	if r, _ := sel.Attr("role"); strings.TrimSpace(r) == "main" {
		isSemantic = true
	}
	if textLen > 0 {
		ld := ltf / textLen
		if isSemantic {
			if ld > 0.7 {
				score *= 0.3
			} else if ld > 0.5 {
				score *= 0.5
			}
		} else {
			if ld > 0.5 {
				score *= 0.1
			} else if ld > 0.3 {
				score *= 0.5
			}
		}
	}
	return score
}

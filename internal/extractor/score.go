package extractor

import (
	"math"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var candidateSelector = "article, main, [role='main'], div, section, td"

var classScorePatterns = []string{"content", "article", "post", "entry"}
var idScorePatterns = []string{"content", "article", "post", "main"}

// ⚡ Bolt Optimization: Zero-allocation case-insensitive substring matcher.
func hasScorePattern(s string, patterns []string) bool {
	sLen := len(s)
	if sLen == 0 {
		return false
	}
	for _, p := range patterns {
		pLen := len(p)
		if pLen > sLen {
			continue
		}
		for i := 0; i <= sLen-pLen; i++ {
			match := true
			for j := 0; j < pLen; j++ {
				c := s[i+j]
				if c >= 'A' && c <= 'Z' {
					c += 'a' - 'A'
				}
				if c != p[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func findBestCandidate(doc *goquery.Document, exclude map[*html.Node]struct{}) *goquery.Selection {
	var best *goquery.Selection
	var bestScore float64

	doc.Find(candidateSelector).Each(func(_ int, s *goquery.Selection) {
		n := s.Get(0)
		if n == nil || isExcluded(n, exclude) {
			return
		}
		if IsNoise(s) || IsNoiseDescendant(s) || isUnderExcluded(s, exclude) {
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

func isUnderExcluded(sel *goquery.Selection, exclude map[*html.Node]struct{}) bool {
	if sel == nil || sel.Length() == 0 {
		return false
	}
	// ⚡ Bolt Optimization: Manually traverse x/net/html nodes
	// instead of allocating new goquery.Selection objects in a loop.
	for p := sel.Get(0).Parent; p != nil; p = p.Parent {
		if isExcluded(p, exclude) {
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
	role, _ := sel.Attr("role")
	// ⚡ Bolt Optimization: Use strings.EqualFold for zero-allocation case-insensitive comparison
	isMainRole := strings.EqualFold(strings.TrimSpace(role), "main")
	if isMainRole {
		score += 50
	}
	if class, ok := sel.Attr("class"); ok {
		// ⚡ Bolt Optimization: Use zero-allocation matching
		if hasScorePattern(class, classScorePatterns) {
			score += 25
		}
	}
	if id, ok := sel.Attr("id"); ok {
		if hasScorePattern(id, idScorePatterns) {
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
	if isMainRole {
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

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

// ⚡ Bolt Optimization: Zero-allocation candidate node check
func isCandidateNode(n *html.Node) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	tag := n.Data
	if tag == "article" || tag == "main" || tag == "div" || tag == "section" || tag == "td" {
		return true
	}
	for _, a := range n.Attr {
		if a.Key == "role" && strings.EqualFold(a.Val, "main") {
			return true
		}
	}
	return false
}

func findBestCandidate(doc *goquery.Document, exclude map[*html.Node]struct{}) *goquery.Selection {
	var best *goquery.Selection
	var bestScore float64

	// ⚡ Bolt Optimization: Manually traverse x/net/html tree
	// instead of using goquery.Find with a complex multi-selector which allocates heavily.
	// We use a reusable goquery.Selection wrapper to avoid allocating for every candidate node.
	selWrapper := &goquery.Selection{Nodes: make([]*html.Node, 1)}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isCandidateNode(n) {
			if !isExcluded(n, exclude) {
				selWrapper.Nodes[0] = n
				if !IsNoise(selWrapper) && !IsNoiseDescendant(selWrapper) && !isUnderExcluded(selWrapper, exclude) {
					sc := scoreNode(selWrapper)
					if sc > 0 && (best == nil || sc > bestScore) {
						// Only allocate a new selection when we find a new best
						best = &goquery.Selection{Nodes: []*html.Node{n}}
						bestScore = sc
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	for _, n := range doc.Nodes {
		walk(n)
	}

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

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == '\v'
}

// ⚡ Bolt Optimization: Calculate trimmed text length via zero-allocation manual tree traversal
func calculateTrimmedTextLength(n *html.Node) int {
	if n == nil {
		return 0
	}

	var firstNonSpaceFound bool
	var totalBytes int
	var trailingSpaceBytes int

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			data := node.Data

			if !firstNonSpaceFound {
				start := 0
				for start < len(data) && isSpace(data[start]) {
					start++
				}

				if start < len(data) {
					firstNonSpaceFound = true
					totalBytes += len(data) - start

					end := len(data)
					for end > start && isSpace(data[end-1]) {
						end--
					}
					trailingSpaceBytes = len(data) - end
				}
			} else {
				totalBytes += len(data)

				end := len(data)
				for end > 0 && isSpace(data[end-1]) {
					end--
				}
				if end == 0 {
					trailingSpaceBytes += len(data)
				} else {
					trailingSpaceBytes = len(data) - end
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)

	if !firstNonSpaceFound {
		return 0
	}

	return totalBytes - trailingSpaceBytes
}

func scoreNode(sel *goquery.Selection) float64 {
	if sel.Length() == 0 {
		return 0
	}
	// ⚡ Bolt Optimization: Use zero-allocation calculateTrimmedTextLength
	// instead of strings.TrimSpace(sel.Text()) which creates heavy string allocations
	textLen := float64(calculateTrimmedTextLength(sel.Get(0)))
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
	// ⚡ Bolt Optimization: Manually traverse x/net/html nodes
	// instead of using goquery's Find() which allocates many new objects.
	pCount := 0
	linkTextLen := 0

	if rootNode := sel.Get(0); rootNode != nil {
		for c := rootNode.FirstChild; c != nil; c = c.NextSibling {
			countPAndA(c, &pCount, &linkTextLen)
		}
	}

	score += float64(pCount) * 3
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

func countPAndA(n *html.Node, pCount *int, linkTextLen *int) {
	if n.Type == html.ElementNode {
		if n.Data == "p" {
			*pCount++
		} else if n.Data == "a" {
			// ⚡ Bolt Optimization: Use zero-allocation trimmed text calculation
			// instead of strings.Builder + strings.TrimSpace
			*linkTextLen += calculateTrimmedTextLength(n)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		countPAndA(c, pCount, linkTextLen)
	}
}

package crawler

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MainContentSelectors lists CSS selectors tried in order for article/main body extraction.
func MainContentSelectors() []string {
	return []string{
		"main",
		"article",
		"[role='main']",
		"[itemprop='articleBody']",
		".article-body",
		".markdown-body",
		".post-content",
		".entry-content",
		"#main-content",
		"#content",
		".content",
		".post",
		".entry",
		"article .content",
	}
}

// MainContentSelectorsJS returns an IIFE script that returns innerHTML from the first matching selector or document.body.
func MainContentSelectorsJS() string {
	var parts []string
	for _, s := range MainContentSelectors() {
		j, err := json.Marshal(s)
		if err != nil {
			continue
		}
		parts = append(parts, string(j))
	}
	return fmt.Sprintf(
		`(function(){var sels=[%s];for(var i=0;i<sels.length;i++){var el=document.querySelector(sels[i]);if(el&&el.innerHTML&&el.innerHTML.length>80)return el.innerHTML;}return document.body?document.body.innerHTML:'';})()`,
		strings.Join(parts, ","),
	)
}

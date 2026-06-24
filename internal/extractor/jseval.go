package extractor

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gocrawl/internal/utils"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"golang.org/x/net/html"
)

const jsevalSetup = `
globalThis.window = globalThis;
globalThis.self = globalThis;
globalThis.document = {
  createElement: function() { return { style: {}, setAttribute: function(){}, appendChild: function(){} }; },
  getElementById: function() { return null; },
  querySelector: function() { return null; },
  querySelectorAll: function() { return []; },
  addEventListener: function() {},
  createEvent: function() { return { initEvent: function(){} }; },
  createTextNode: function() { return {}; },
  head: { appendChild: function(){}, removeChild: function(){} },
  body: { appendChild: function(){}, removeChild: function(){} },
  documentElement: { style: {} },
  cookie: "",
  readyState: "complete",
  location: { href: "", hostname: "", pathname: "/" }
};
globalThis.navigator = {
  userAgent: "Mozilla/5.0",
  language: "en-US",
  languages: ["en-US"],
  platform: "Linux x86_64",
  cookieEnabled: true
};
globalThis.location = { href: "", hostname: "", pathname: "/", search: "", hash: "" };
globalThis.history = { pushState: function(){}, replaceState: function(){} };
globalThis.__gocrawlTimerId = 1;
globalThis.__gocrawlTimerQueue = [];
globalThis.__gocrawlTimerFns = {};
globalThis.setTimeout = function(fn) {
  var id = globalThis.__gocrawlTimerId++;
  if (typeof fn === "function") {
    globalThis.__gocrawlTimerFns[id] = fn;
    globalThis.__gocrawlTimerQueue.push(id);
  }
  return id;
};
globalThis.clearTimeout = function(id) {
  delete globalThis.__gocrawlTimerFns[id];
};
globalThis.setInterval = function() { return 0; };
globalThis.clearInterval = function() {};
globalThis.requestAnimationFrame = function() { return 0; };
globalThis.cancelAnimationFrame = function() {};
globalThis.console = { log: function(){}, warn: function(){}, error: function(){}, info: function(){}, debug: function(){} };
globalThis.fetch = function() { return Promise.resolve({ json: function(){ return Promise.resolve({}); }, text: function(){ return Promise.resolve(""); } }); };
globalThis.XMLHttpRequest = function() { this.open = function(){}; this.send = function(){}; this.setRequestHeader = function(){}; };
globalThis.localStorage = { getItem: function(){ return null; }, setItem: function(){}, removeItem: function(){}, clear: function(){} };
globalThis.sessionStorage = { getItem: function(){ return null; }, setItem: function(){}, removeItem: function(){}, clear: function(){} };
globalThis.addEventListener = function() {};
globalThis.removeEventListener = function() {};
globalThis.dispatchEvent = function() {};
globalThis.getComputedStyle = function() { return {}; };
globalThis.matchMedia = function() { return { matches: false, addListener: function(){}, removeListener: function(){} }; };
globalThis.Image = function() {};
globalThis.Event = function() {};
globalThis.CustomEvent = function() {};
globalThis.MutationObserver = function() { this.observe = function(){}; this.disconnect = function(){}; };
globalThis.IntersectionObserver = function() { this.observe = function(){}; this.disconnect = function(){}; };
globalThis.ResizeObserver = function() { this.observe = function(){}; this.disconnect = function(){}; };
globalThis.performance = { now: function(){ return 0; }, mark: function(){}, measure: function(){} };
globalThis.crypto = { getRandomValues: function(arr) { return arr; } };
globalThis.URL = function(u) { this.href = u || ""; this.searchParams = { get: function(){ return null; } }; };
globalThis.Promise = Promise;
self.__next_f = self.__next_f || [];
`

const jsevalScan = `
(function() {
  var results = [];
  var keys = Object.keys(globalThis);
  for (var i = 0; i < keys.length; i++) {
    var key = keys[i];
    if (key.indexOf("__") !== 0) continue;
    var val = globalThis[key];
    if (val === null || val === undefined) continue;
    if (key === "__next_f") {
      if (Array.isArray(val) && val.length > 0) {
        var json = JSON.stringify(val);
        if (json.length > 100) {
          results.push({ name: key, data: json, size: json.length });
        }
      }
      continue;
    }
    if (typeof val === "object") {
      try {
        var json = JSON.stringify(val);
        if (json && json.length > 100) {
          results.push({ name: key, data: json, size: json.length });
        }
      } catch(e) {}
    }
  }
  return JSON.stringify(results);
})()
`

const jsevalDrainTimers = `
(function() {
  var safety = 1000;
  while (globalThis.__gocrawlTimerQueue && globalThis.__gocrawlTimerQueue.length && safety-- > 0) {
    var id = globalThis.__gocrawlTimerQueue.shift();
    var fn = globalThis.__gocrawlTimerFns[id];
    delete globalThis.__gocrawlTimerFns[id];
    if (typeof fn === "function") {
      try { fn(); } catch (e) {}
    }
  }
})()
`

var errExecutionTimeout = errors.New("execution timeout")

const maxExtractedTextBytes = 256 * 1024

// JsDataBlob holds data extracted from inline script execution (webclaw-style).
type JsDataBlob struct {
	Name string `json:"name"`
	Data string `json:"data"`
	Size int    `json:"size"`
}

// ExtractJsDataFromHTML runs inline scripts in a goja sandbox and collects window.__* JSON blobs.
func ExtractJsDataFromHTML(htmlStr string) []JsDataBlob {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return nil
	}
	var scripts []string

	// ⚡ Bolt Optimization: Manually traverse x/net/html tree
	// to avoid goquery.Find("script") selector and struct allocation overhead.
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			var src, t string
			for _, attr := range n.Attr {
				if attr.Key == "src" {
					src = attr.Val
				} else if attr.Key == "type" {
					t = attr.Val
				}
			}
			if src != "" {
				goto next
			}

			{
				// ⚡ Bolt Optimization: Use zero-allocation EqualFold instead of strings.ToLower
				tTrim := strings.TrimSpace(t)
				isClassic := tTrim == "" ||
					strings.EqualFold(tTrim, "text/javascript") ||
					strings.EqualFold(tTrim, "application/javascript") ||
					strings.EqualFold(tTrim, "text/ecmascript") ||
					strings.EqualFold(tTrim, "application/ecmascript")

				if !isClassic {
					goto next
				}

				// Extract script text from children
				var sb strings.Builder
				var extractText func(*html.Node)
				extractText = func(node *html.Node) {
					if node.Type == html.TextNode {
						sb.WriteString(node.Data)
					}
					for c := node.FirstChild; c != nil; c = c.NextSibling {
						extractText(c)
					}
				}
				extractText(n)

				txt := strings.TrimSpace(sb.String())
				if txt == "" {
					goto next
				}
				if len(txt) > 2<<20 {
					goto next
				}
				scripts = append(scripts, txt)
			}
		}
	next:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	for _, n := range doc.Nodes {
		walk(n)
	}

	if len(scripts) == 0 {
		return nil
	}

	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	if err := vm.Set("globalThis", vm.GlobalObject()); err != nil {
		return nil
	}
	if _, err := vm.RunString(jsevalSetup); err != nil {
		return nil
	}
	deadline := time.Now().Add(8 * time.Second)
	for _, script := range scripts {
		if time.Now().After(deadline) {
			break
		}
		if err := runWithDeadline(vm, script, deadline); err != nil {
			if isExecutionTimeoutErr(err) {
				break
			}
			log.Printf("jseval: script runtime error: %v", err)
			continue
		}
		if err := runWithDeadline(vm, jsevalDrainTimers, deadline); err != nil {
			if isExecutionTimeoutErr(err) {
				break
			}
			log.Printf("jseval: timer drain runtime error: %v", err)
		}
	}
	v, err := runValueWithDeadline(vm, jsevalScan, deadline)
	if err != nil {
		return nil
	}
	raw, ok := v.Export().(string)
	if !ok {
		return nil
	}
	var blobs []JsDataBlob
	if err := json.Unmarshal([]byte(raw), &blobs); err != nil {
		return nil
	}
	return blobs
}

func runWithDeadline(vm *goja.Runtime, script string, deadline time.Time) error {
	_, err := runValueWithDeadline(vm, script, deadline)
	return err
}

func runValueWithDeadline(vm *goja.Runtime, script string, deadline time.Time) (goja.Value, error) {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return nil, errExecutionTimeout
	}
	done := make(chan struct{})
	timer := time.NewTimer(remaining)
	defer func() {
		close(done)
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	go func() {
		select {
		case <-timer.C:
			vm.Interrupt(errExecutionTimeout)
		case <-done:
		}
	}()
	// Clear any stale interrupt before executing the next script.
	vm.ClearInterrupt()
	v, err := vm.RunString(script)
	return v, err
}

func isExecutionTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errExecutionTimeout) {
		return true
	}
	// ⚡ Bolt Optimization: Use zero-allocation HasAnyLowercasePattern instead of strings.ToLower
	return utils.HasAnyLowercasePattern(err.Error(), []string{"execution timeout"})
}

// ExtractReadableTextFromBlobs walks JSON blobs and builds a markdown section (webclaw-style).
func ExtractReadableTextFromBlobs(blobs []JsDataBlob) string {
	if len(blobs) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	var texts []string
	accumulatedBytes := 0
	appendWithinCap := func(t string) bool {
		if t == "" {
			return false
		}
		if _, ok := seen[t]; ok {
			return false
		}
		remaining := maxExtractedTextBytes - accumulatedBytes
		if remaining <= 0 {
			return true
		}
		if len(t) > remaining {
			// Stop further growth once the budget is reached.
			tr := strings.TrimSpace(t[:remaining])
			if tr != "" {
				seen[tr] = struct{}{}
				texts = append(texts, tr)
				accumulatedBytes += len(tr)
			}
			return true
		}
		seen[t] = struct{}{}
		texts = append(texts, t)
		accumulatedBytes += len(t)
		return false
	}

outer:
	for _, b := range blobs {
		if b.Name == "__next_f" {
			for _, t := range extractNextFText(b.Data) {
				if stop := appendWithinCap(t); stop {
					break outer
				}
			}
			continue
		}
		var v interface{}
		if err := json.Unmarshal([]byte(b.Data), &v); err != nil {
			continue
		}
		var found []string
		walkJSONForText(v, &found, 0)
		for _, t := range found {
			if stop := appendWithinCap(t); stop {
				break outer
			}
		}
	}
	if len(texts) == 0 {
		return ""
	}
	return "## Additional Content\n\n" + strings.Join(texts, "\n\n")
}

func walkJSONForText(v interface{}, out *[]string, depth int) {
	if depth > 15 {
		return
	}
	switch x := v.(type) {
	case string:
		if clean := filterReadable(x); clean != "" {
			*out = append(*out, clean)
		}
	case map[string]interface{}:
		for _, vv := range x {
			walkJSONForText(vv, out, depth+1)
		}
	case []interface{}:
		for _, vv := range x {
			walkJSONForText(vv, out, depth+1)
		}
	}
}

// ⚡ Bolt Optimization: Zero-allocation string builder-based approach to strip HTML tags
// without using regexp.MustCompile(`<[^>]+>`).ReplaceAllString which allocates heavily.
func stripHTMLTagsStrict(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '<' {
			j := i + 1
			for j < len(s) && s[j] != '>' {
				j++
			}
			if j < len(s) && j > i+1 { // <[^>]+>
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func filterReadable(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 15 {
		return ""
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "//") {
		return ""
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return ""
	}
	// ⚡ Bolt Optimization: Use IndexByte instead of Contains for faster rejection.
	if strings.IndexByte(s, '{') >= 0 && strings.IndexByte(s, '}') >= 0 && (strings.IndexByte(s, ':') >= 0 || strings.IndexByte(s, ';') >= 0) {
		return ""
	}

	hasLeftAngle := strings.IndexByte(s, '<') >= 0
	hasRightAngle := strings.IndexByte(s, '>') >= 0
	hasHTMLTags := hasLeftAngle && hasRightAngle

	if hasHTMLTags && strings.Count(s, "<") > 3 && strings.Count(s, ">") > 3 {
		stripped := stripHTMLTagsStrict(s)
		if len(strings.TrimSpace(stripped)) < 15 {
			return ""
		}
	}
	alphaSpace := 0
	hasSeparator := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			alphaSpace++
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			hasSeparator = true
		}
	}
	totalRunes := utf8.RuneCountInString(s)
	if totalRunes == 0 {
		return ""
	}
	if float64(alphaSpace)/float64(totalRunes) < 0.6 {
		return ""
	}
	if !hasSeparator {
		return ""
	}

	// ⚡ Bolt Optimization: Skip regex replace if there are no HTML tags, avoiding unnecessary allocation.
	if hasHTMLTags {
		clean := strings.TrimSpace(stripHTMLTagsStrict(s))
		if len(clean) <= 15 {
			return ""
		}
		return clean
	}

	// s is already trimmed at the start of the function and length checked.
	return s
}

func extractNextFText(rawJSON string) []string {
	var entries []interface{}
	if err := json.Unmarshal([]byte(rawJSON), &entries); err != nil {
		return nil
	}

	// ⚡ Bolt Optimization: Pre-calculate total string length and call wire.Grow()
	// to allocate memory exactly once, eliminating repeated heap allocations
	// and dynamic array resizing when concatenating many Next.js JSON payloads.
	var totalLen int
	for _, e := range entries {
		if arr, ok := e.([]interface{}); ok && len(arr) >= 2 {
			if t, ok := arr[0].(float64); ok && int(t) == 1 {
				if payload, ok := arr[1].(string); ok {
					totalLen += len(payload)
				}
			}
		}
	}

	var wire strings.Builder
	wire.Grow(totalLen)

	for _, e := range entries {
		if arr, ok := e.([]interface{}); ok && len(arr) >= 2 {
			if t, ok := arr[0].(float64); ok && int(t) == 1 {
				if payload, ok := arr[1].(string); ok {
					wire.WriteString(payload)
				}
			}
		}
	}

	var texts []string
	// ⚡ Bolt Optimization: Use manual zero-allocation line scanning
	// instead of strings.Split which allocates a huge slice for large Next.js payloads.
	str := wire.String()
	for {
		idx := strings.IndexByte(str, '\n')
		var line string
		if idx < 0 {
			line = str
			str = ""
		} else {
			line = str[:idx]
			str = str[idx+1:]
		}

		if len(line) == 0 && idx < 0 {
			break
		}

		pipeIdx := strings.IndexByte(line, '|')
		if pipeIdx < 0 {
			continue
		}
		payload := line[pipeIdx+1:]
		var v interface{}
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			continue
		}
		walkRSC(v, &texts, 0)
	}
	return texts
}

func walkRSC(v interface{}, out *[]string, depth int) {
	if depth > 20 {
		return
	}
	switch x := v.(type) {
	case string:
		if clean := filterReadable(x); clean != "" {
			*out = append(*out, clean)
		}
	case []interface{}:
		for _, item := range x {
			walkRSC(item, out, depth+1)
		}
	case map[string]interface{}:
		if ch, ok := x["children"]; ok {
			walkRSC(ch, out, depth+1)
		}
		for k, vv := range x {
			if k == "children" {
				continue
			}
			walkRSC(vv, out, depth+1)
		}
	}
}

package extractor

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
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
globalThis.setTimeout = function(fn) { if (typeof fn === "function") { try { fn(); } catch(e) {} } return 0; };
globalThis.clearTimeout = function() {};
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

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// JsDataBlob holds data extracted from inline script execution (webclaw-style).
type JsDataBlob struct {
	Name string `json:"name"`
	Data string `json:"data"`
	Size int    `json:"size"`
}

// ExtractJsDataFromHTML runs inline scripts in a goja sandbox and collects window.__* JSON blobs.
func ExtractJsDataFromHTML(html string) []JsDataBlob {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}
	var scripts []string
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		if src, _ := s.Attr("src"); src != "" {
			return
		}
		if t, _ := s.Attr("type"); t == "module" {
			return
		}
		txt := strings.TrimSpace(s.Text())
		if txt == "" {
			return
		}
		if len(txt) > 2<<20 {
			return
		}
		scripts = append(scripts, txt)
	})
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
		_, _ = vm.RunString(script)
	}
	v, err := vm.RunString(jsevalScan)
	if err != nil {
		return nil
	}
	raw := v.String()
	var blobs []JsDataBlob
	if err := json.Unmarshal([]byte(raw), &blobs); err != nil {
		return nil
	}
	return blobs
}

// ExtractReadableTextFromBlobs walks JSON blobs and builds a markdown section (webclaw-style).
func ExtractReadableTextFromBlobs(blobs []JsDataBlob) string {
	if len(blobs) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	var texts []string
	for _, b := range blobs {
		if b.Name == "__next_f" {
			for _, t := range extractNextFText(b.Data) {
				if _, ok := seen[t]; !ok {
					seen[t] = struct{}{}
					texts = append(texts, t)
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
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				texts = append(texts, t)
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
	if strings.Contains(s, "{") && strings.Contains(s, "}") && (strings.Contains(s, ":") || strings.Contains(s, ";")) {
		return ""
	}
	if strings.Count(s, "<") > 3 && strings.Count(s, ">") > 3 {
		stripped := htmlTagRe.ReplaceAllString(s, "")
		if len(strings.TrimSpace(stripped)) < 15 {
			return ""
		}
	}
	alphaSpace := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == ' ' || unicodeIsSpace(r) {
			alphaSpace++
		}
	}
	if float64(alphaSpace)/float64(len(s)) < 0.6 {
		return ""
	}
	if !strings.Contains(s, " ") {
		return ""
	}
	clean := strings.TrimSpace(htmlTagRe.ReplaceAllString(s, ""))
	if len(clean) <= 15 {
		return ""
	}
	return clean
}

func unicodeIsSpace(r rune) bool {
	return r == '\t' || r == '\n' || r == '\r' || r == '\u00a0'
}

func extractNextFText(rawJSON string) []string {
	var entries []interface{}
	if err := json.Unmarshal([]byte(rawJSON), &entries); err != nil {
		return nil
	}
	var wire strings.Builder
	for _, e := range entries {
		arr, ok := e.([]interface{})
		if !ok || len(arr) < 2 {
			continue
		}
		t, _ := arr[0].(float64)
		if int(t) != 1 {
			continue
		}
		if payload, ok := arr[1].(string); ok {
			wire.WriteString(payload)
		}
	}
	var texts []string
	for _, line := range strings.Split(wire.String(), "\n") {
		idx := strings.Index(line, "|")
		if idx < 0 {
			continue
		}
		payload := line[idx+1:]
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

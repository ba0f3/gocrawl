package crawler

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestShouldChromedpFallback_visitError(t *testing.T) {
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		VisitErr: errors.New("fail"),
		Result:   &ScrapeResult{Metadata: map[string]string{}},
		OnlyMain: true,
	})
	if !yes || tag != "visit_error" {
		t.Fatalf("got %v %q", yes, tag)
	}
}

func TestShouldChromedpFallback_nilResult(t *testing.T) {
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		OnlyMain: true,
	})
	if !yes || tag != "nil_result" {
		t.Fatalf("got %v %q", yes, tag)
	}
}

func TestShouldChromedpFallback_collyError(t *testing.T) {
	res := &ScrapeResult{Metadata: map[string]string{"error": "timeout"}}
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		Result:   res,
		OnlyMain: true,
	})
	if !yes || tag != "colly_error" {
		t.Fatalf("got %v %q", yes, tag)
	}
}

func TestShouldChromedpFallback_statusCodes(t *testing.T) {
	res := &ScrapeResult{Metadata: map[string]string{"statusCode": "403"}}
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		Result: res,
	})
	if !yes || tag != "status_403" {
		t.Fatalf("got %v %q", yes, tag)
	}
	res2 := &ScrapeResult{Metadata: map[string]string{"statusCode": "200"}}
	yes2, _ := ShouldChromedpFallback(&FallbackCriteria{
		Result: res2,
	})
	if yes2 {
		t.Fatal("expected no fallback for 200")
	}
	custom := []int{500}
	res3 := &ScrapeResult{Metadata: map[string]string{"statusCode": "500"}}
	yes3, tag3 := ShouldChromedpFallback(&FallbackCriteria{
		Result:      res3,
		StatusCodes: custom,
	})
	if !yes3 || tag3 != "status_500" {
		t.Fatalf("custom codes: got %v %q", yes3, tag3)
	}
}

func TestShouldChromedpFallback_antibotHeader(t *testing.T) {
	h := http.Header{}
	h.Set("cf-mitigated", "challenge")
	res := &ScrapeResult{Metadata: map[string]string{"statusCode": "200"}}
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		Result:  res,
		Headers: h,
		URL:     "https://example.com/page",
	})
	if !yes || tag != "antibot_cloudflare" {
		t.Fatalf("got %v %q want antibot_cloudflare", yes, tag)
	}
}

func TestShouldChromedpFallback_antibotHTML(t *testing.T) {
	html := `<html><head></head><body><script src="https://hcaptcha.com/1/api.js"></script></body></html>`
	res := &ScrapeResult{Metadata: map[string]string{"statusCode": "200"}}
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		Result:   res,
		PageBody: []byte(html),
		URL:      "https://example.com/",
	})
	if !yes || tag != "antibot_hcaptcha" {
		t.Fatalf("got %v %q want antibot_hcaptcha", yes, tag)
	}
}

func TestShouldChromedpFallback_challengeHTML(t *testing.T) {
	html := "<html><title>Just a moment...</title><body>cloudflare</body></html>"
	res := &ScrapeResult{Metadata: map[string]string{"statusCode": "200"}}
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		Result:   res,
		PageBody: []byte(html),
	})
	if !yes || tag != "challenge_html" {
		t.Fatalf("got %v %q", yes, tag)
	}
}

func TestShouldChromedpFallback_spaShell(t *testing.T) {
	html := `<!DOCTYPE html><html><body><div id="root"><script></script><!--` + strings.Repeat("x", 450) + `--></div></body></html>`
	res := &ScrapeResult{
		Metadata: map[string]string{"statusCode": "200"},
		Markdown: strings.Repeat("word ", 30),
	}
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		Result:   res,
		PageBody: []byte(html),
	})
	if !yes || tag != "spa_shell" {
		t.Fatalf("got %v %q", yes, tag)
	}
}

func TestShouldChromedpFallback_csrFrameworkNext(t *testing.T) {
	html := `<!DOCTYPE html><html><head><script src="/_next/static/chunks/main.js"></script></head><body></body></html>` +
		strings.Repeat(" ", 2600)
	res := &ScrapeResult{Metadata: map[string]string{"statusCode": "200"}, Markdown: "x"}
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		Result:   res,
		PageBody: []byte(html),
	})
	if !yes || tag != "csr_framework" {
		t.Fatalf("got %v %q want csr_framework", yes, tag)
	}
}

func TestShouldChromedpFallback_csrFrameworkVue(t *testing.T) {
	html := `<!DOCTYPE html><html><body><div data-v-1a2b3c4d></div></body></html>` + strings.Repeat("\n", 2600)
	res := &ScrapeResult{Metadata: map[string]string{"statusCode": "200"}, Markdown: "x"}
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		Result:   res,
		PageBody: []byte(html),
	})
	if !yes || tag != "csr_framework" {
		t.Fatalf("got %v %q want csr_framework", yes, tag)
	}
}

func TestHasUICSRFrameworkMarker(t *testing.T) {
	cases := []struct {
		snippet string
		want    bool
	}{
		{`<script src="/@vite/client"></script>`, true},
		{`window.__NUXT__={}`, true},
		{`<html><body><p>Static article about React</p></body></html>`, false},
	}
	for _, tc := range cases {
		got := hasUICSRFrameworkMarker(tc.snippet)
		if got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.snippet, got, tc.want)
		}
	}
}

func TestShouldChromedpFallback_thinMarkdown(t *testing.T) {
	res := &ScrapeResult{
		Metadata: map[string]string{"statusCode": "200"},
		Markdown: "short",
	}
	yes, tag := ShouldChromedpFallback(&FallbackCriteria{
		Result:   res,
		OnlyMain: true,
		PageBody: []byte("<html><body><p>ok</p></body></html>"),
	})
	if !yes || tag != "thin_markdown" {
		t.Fatalf("got %v %q", yes, tag)
	}
}

func TestShouldChromedpFallback_thinMarkdownSkippedWhenFullPage(t *testing.T) {
	res := &ScrapeResult{
		Metadata: map[string]string{"statusCode": "200"},
		Markdown: "short",
	}
	yes, _ := ShouldChromedpFallback(&FallbackCriteria{
		Result:   res,
		OnlyMain: false,
		PageBody: []byte("<html><body><p>ok</p></body></html>"),
	})
	if yes {
		t.Fatal("onlyMain false should not use thin_markdown alone")
	}
}

func TestDefaultChromedpFallbackStatusCodes(t *testing.T) {
	d := DefaultChromedpFallbackStatusCodes()
	if len(d) < 4 {
		t.Fatal(d)
	}
}

package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gocrawl/internal/config"
)

func TestSummarizeMarkdown_Mock(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "One sentence summary here."}},
			},
		})
	}))
	defer ts.Close()

	// Use a unique timeout to prevent test cache collisions
	testTimeout := 5*time.Second + 1*time.Millisecond
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Enabled: true,
			BaseURL: ts.URL,
			Model:   "test-model",
			Timeout: testTimeout,
		},
	}

	// Inject a client into the cache to bypass SSRF protection for 127.0.0.1 in the test
	testClient := ts.Client()
	testClient.Timeout = testTimeout
	clientCache.Store(testTimeout, testClient)
	defer clientCache.Delete(testTimeout)

	out, err := SummarizeMarkdown(context.Background(), cfg, "test-model", "# Hello\n\nSome markdown content that is long enough.", 2)
	if err != nil {
		t.Fatal(err)
	}
	if out != "One sentence summary here." {
		t.Fatalf("unexpected: %q", out)
	}
}

func TestStripThinkingTags_TrimsSpace(t *testing.T) {
	if stripThinkingTags("  hello  ") != "hello" {
		t.Fatalf("got %q", stripThinkingTags("  hello  "))
	}
	got := stripThinkingTags("prefix <think>internal reasoning</think> summary")
	if got != "prefix summary" {
		t.Fatalf("expected think block removed, got %q", got)
	}
}

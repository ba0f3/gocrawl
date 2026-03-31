package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"strings"
	"time"

	"gocrawl/internal/config"
)

// Matches <think>...</think> blocks emitted by some models before the final answer.
var thinkingTagRE = regexp.MustCompile(`(?is)<think\b[^>]*>[\s\S]*?</think>`)
var wsRE = regexp.MustCompile(`\s+`)
var clientCache sync.Map

// SummarizeMarkdown calls an OpenAI-compatible chat completions API (webclaw-style prompt).
func SummarizeMarkdown(ctx context.Context, cfg *config.Config, model, markdown string, maxSentences int) (string, error) {
	if cfg == nil || !cfg.LLM.Enabled {
		return "", fmt.Errorf("llm: disabled")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.LLM.BaseURL), "/")
	if base == "" {
		return "", fmt.Errorf("llm: LLM_BASE_URL not set")
	}
	if model == "" {
		return "", fmt.Errorf("llm: model not set")
	}
	if maxSentences <= 0 {
		maxSentences = 3
	}
	system := fmt.Sprintf(
		"You are a summarization engine. Summarize the following content in exactly %d sentences. "+
			"Output ONLY the summary, nothing else. No introductions, no questions, no formatting, no preamble.",
		maxSentences,
	)
	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": markdown},
		},
		"temperature": 0.3,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if k := strings.TrimSpace(cfg.LLM.APIKey); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}
	timeout := cfg.LLM.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	client := getHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("llm: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("llm: empty choices")
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	return stripThinkingTags(text), nil
}

func stripThinkingTags(s string) string {
	s = thinkingTagRE.ReplaceAllString(s, " ")
	s = wsRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func getHTTPClient(timeout time.Duration) *http.Client {
	if c, ok := clientCache.Load(timeout); ok {
		return c.(*http.Client)
	}
	c := &http.Client{Timeout: timeout}
	actual, _ := clientCache.LoadOrStore(timeout, c)
	return actual.(*http.Client)
}

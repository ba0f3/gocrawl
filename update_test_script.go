package main

import (
	"os"
	"strings"
)

func main() {
	content, err := os.ReadFile("internal/llm/summarize_test.go")
	if err != nil {
		panic(err)
	}

	old := `	cfg := &config.Config{
		LLM: config.LLMConfig{
			Enabled: true,
			BaseURL: ts.URL,
			Model:   "test-model",
			Timeout: 5 * time.Second,
		},
	}

	// Inject a client into the cache to bypass SSRF protection for 127.0.0.1 in the test
	testClient := ts.Client()
	testClient.Timeout = 5 * time.Second
	clientCache.Store(5*time.Second, testClient)
	defer clientCache.Delete(5 * time.Second)`

	new := `	// Use a unique timeout to prevent test cache collisions
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
	defer clientCache.Delete(testTimeout)`

	newContent := strings.Replace(string(content), old, new, 1)
	err = os.WriteFile("internal/llm/summarize_test.go", []byte(newContent), 0644)
	if err != nil {
		panic(err)
	}
}

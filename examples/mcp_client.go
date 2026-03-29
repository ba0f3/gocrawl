package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Example MCP client demonstrating both regular HTTP and SSE streaming

func main() {
	baseURL := "http://localhost:8080/v1/mcp"

	// Example 1: Regular HTTP request
	fmt.Println("=== Example 1: Regular HTTP Request ===")
	regularHTTPExample(baseURL)

	// Example 2: SSE streaming request
	fmt.Println("\n=== Example 2: SSE Streaming Request ===")
	sseStreamingExample(baseURL)
}

func regularHTTPExample(baseURL string) {
	// Prepare request body
	reqBody := map[string]interface{}{
		"url":             "https://example.com",
		"onlyMainContent": true,
		"formats":         []string{"markdown", "html"},
		"timeout":         30,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatal("Error marshaling request:", err)
	}

	// Make HTTP request
	resp, err := http.Post(baseURL+"/scrape", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal("Error making request:", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response:", err)
	}

	fmt.Printf("Response: %s\n", string(body))
}

func sseStreamingExample(baseURL string) {
	// Prepare request body
	reqBody := map[string]interface{}{
		"url":             "https://example.com",
		"onlyMainContent": true,
		"formats":         []string{"markdown"},
		"timeout":         30,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatal("Error marshaling request:", err)
	}

	// Create request with SSE headers
	req, err := http.NewRequest("POST", baseURL+"/scrape", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal("Error creating request:", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error making request:", err)
	}
	defer resp.Body.Close()

	// Read SSE stream
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal("Error reading stream:", err)
		}

		// Process SSE data
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			data = strings.TrimSpace(data)

			// Parse JSON data
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				fmt.Printf("Event Type: %v\n", event["type"])
				fmt.Printf("Event Data: %v\n", event["data"])
				fmt.Println("---")
			}
		}
	}
}

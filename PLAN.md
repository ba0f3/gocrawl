### **Instruction for Code Agent: Go Web Crawler with Colly and Firecrawl API**

**Objective:**
Generate a complete, production-ready Go application that functions as a web crawler and scraper.
The application will use the `gocolly/colly` library to crawl websites and the Firecrawl API to extract clean, AI-ready content in Markdown format from each crawled page.

**Core Requirements:**
1.  **Crawler/Scraper Engine:** Use `github.com/gocolly/colly/v2`.
2.  **API Service:** Expose an API endpoint that accepts a URL and returns the scraped Markdown content, adopt Firecrawl API.
3.  **Configuration:** Manage API keys and target URLs securely using `github.com/spf13/viper` with environment variables and a `.env` file.
4.  **Project Structure:** Adhere to the specified modular project structure.
5.  **Output:** Save the scraped Markdown content from each page into sqlite database, automatic clear old data.
6.  **Database:** Use `github.com/mattn/go-sqlite3` to interact with the sqlite database.
7.  **Context:** Use context7 to update Firecrawl API document.
8.  **User Management:** Add user management feature, user can register, login, and manage their API keys.

**Project Structure:**
Generate the following directory and file structure:
```
gocrawl/
├── cmd/
│   └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── crawler/
│   │   └── colly.go
│   └── scraper/
│       └── firecrawl.go
├── .env
└── go.mod
```


** API Service **
*** Scrape API ***
POST /api/v1/scrape
Request:
```go
package main

import (
	"fmt"
	"strings"
	"net/http"
	"io"
)

func main() {

	url := "https://api.firecrawl.dev/v1/scrape"

	payload := strings.NewReader("{\n  \"url\": \"<string>\",\n  \"onlyMainContent\": true,\n  \"includeTags\": [\n    \"<string>\"\n  ],\n  \"excludeTags\": [\n    \"<string>\"\n  ],\n  \"maxAge\": 0,\n  \"headers\": {},\n  \"waitFor\": 0,\n  \"mobile\": false,\n  \"skipTlsVerification\": false,\n  \"timeout\": 30000,\n  \"parsePDF\": true,\n  \"jsonOptions\": {\n    \"schema\": {},\n    \"systemPrompt\": \"<string>\",\n    \"prompt\": \"<string>\"\n  },\n  \"actions\": [\n    {\n      \"type\": \"wait\",\n      \"milliseconds\": 2,\n      \"selector\": \"#my-element\"\n    }\n  ],\n  \"location\": {\n    \"country\": \"US\",\n    \"languages\": [\n      \"en-US\"\n    ]\n  },\n  \"removeBase64Images\": true,\n  \"blockAds\": true,\n  \"proxy\": \"basic\",\n  \"storeInCache\": true,\n  \"formats\": [\n    \"markdown\"\n  ],\n  \"changeTrackingOptions\": {\n    \"modes\": [\n      \"git-diff\"\n    ],\n    \"schema\": {},\n    \"prompt\": \"<string>\",\n    \"tag\": null\n  },\n  \"zeroDataRetention\": false\n}")

	req, _ := http.NewRequest("POST", url, payload)

	req.Header.Add("Authorization", "Bearer <token>")
	req.Header.Add("Content-Type", "application/json")

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	fmt.Println(res)
	fmt.Println(string(body))

}
```
Response:
```json
{
  "success": true,
  "data": {
    "markdown": "<string>",
    "html": "<string>",
    "rawHtml": "<string>",
    "screenshot": "<string>",
    "links": [
      "<string>"
    ],
    "actions": {
      "screenshots": [
        "<string>"
      ],
      "scrapes": [
        {
          "url": "<string>",
          "html": "<string>"
        }
      ],
      "javascriptReturns": [
        {
          "type": "<string>",
          "value": "<any>"
        }
      ],
      "pdfs": [
        "<string>"
      ]
    },
    "metadata": {
      "title": "<string>",
      "description": "<string>",
      "language": "<string>",
      "sourceURL": "<string>",
      "<any other metadata> ": "<string>",
      "statusCode": 123,
      "error": "<string>"
    },
    "llm_extraction": {},
    "warning": "<string>",
    "changeTracking": {
      "previousScrapeAt": "2023-11-07T05:31:56Z",
      "changeStatus": "new",
      "visibility": "visible",
      "diff": "<string>",
      "json": {}
    }
  }
}
```
***Crawl API***
POST /api/v1/crawl
Request:
```go
package main

import (
	"fmt"
	"strings"
	"net/http"
	"io"
)

func main() {

	url := "https://api.firecrawl.dev/v1/crawl"

	payload := strings.NewReader("{\n  \"url\": \"<string>\",\n  \"excludePaths\": [\n    \"<string>\"\n  ],\n  \"includePaths\": [\n    \"<string>\"\n  ],\n  \"maxDepth\": 10,\n  \"maxDiscoveryDepth\": 123,\n  \"ignoreSitemap\": false,\n  \"ignoreQueryParameters\": false,\n  \"limit\": 10000,\n  \"allowBackwardLinks\": false,\n  \"crawlEntireDomain\": false,\n  \"allowExternalLinks\": false,\n  \"allowSubdomains\": false,\n  \"delay\": 123,\n  \"maxConcurrency\": 123,\n  \"webhook\": {\n    \"url\": \"<string>\",\n    \"headers\": {},\n    \"metadata\": {},\n    \"events\": [\n      \"completed\"\n    ]\n  },\n  \"scrapeOptions\": {\n    \"onlyMainContent\": true,\n    \"includeTags\": [\n      \"<string>\"\n    ],\n    \"excludeTags\": [\n      \"<string>\"\n    ],\n    \"maxAge\": 0,\n    \"headers\": {},\n    \"waitFor\": 0,\n    \"mobile\": false,\n    \"skipTlsVerification\": false,\n    \"timeout\": 30000,\n    \"parsePDF\": true,\n    \"jsonOptions\": {\n      \"schema\": {},\n      \"systemPrompt\": \"<string>\",\n      \"prompt\": \"<string>\"\n    },\n    \"actions\": [\n      {\n        \"type\": \"wait\",\n        \"milliseconds\": 2,\n        \"selector\": \"#my-element\"\n      }\n    ],\n    \"location\": {\n      \"country\": \"US\",\n      \"languages\": [\n        \"en-US\"\n      ]\n    },\n    \"removeBase64Images\": true,\n    \"blockAds\": true,\n    \"proxy\": \"basic\",\n    \"storeInCache\": true,\n    \"formats\": [\n      \"markdown\"\n    ],\n    \"changeTrackingOptions\": {\n      \"modes\": [\n        \"git-diff\"\n      ],\n      \"schema\": {},\n      \"prompt\": \"<string>\",\n      \"tag\": null\n    }\n  },\n  \"zeroDataRetention\": false\n}")

	req, _ := http.NewRequest("POST", url, payload)

	req.Header.Add("Authorization", "Bearer <token>")
	req.Header.Add("Content-Type", "application/json")

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	fmt.Println(res)
	fmt.Println(string(body))

}
```
Response:
```json
{
  "success": true,
  "id": "<string>",
  "url": "<string>"
}
```
***Get Crawl Status API***
GET /api/v1/crawl/{id}
Request:
```go
package main

import (
	"fmt"
	"net/http"
	"io"
)

func main() {

	url := "https://api.firecrawl.dev/v1/crawl/<id>"

	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("Authorization", "Bearer <token>")

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	fmt.Println(res)
	fmt.Println(string(body))

}
```
Response:
```json
{
  "status": "<string>",
  "total": 123,
  "completed": 123,
  "creditsUsed": 123,
  "expiresAt": "2023-11-07T05:31:56Z",
  "next": "<string>",
  "data": [
    {
      "markdown": "<string>",
      "html": "<string>",
      "rawHtml": "<string>",
      "links": [
        "<string>"
      ],
      "screenshot": "<string>",
      "metadata": {
        "title": "<string>",
        "description": "<string>",
        "language": "<string>",
        "sourceURL": "<string>",
        "<any other metadata> ": "<string>",
        "statusCode": 123,
        "error": "<string>"
      }
    }
  ]
}
```
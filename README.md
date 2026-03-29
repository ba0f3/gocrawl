# GoCrawl - Firecrawl-like Web Crawler

A production-grade Go web crawler that provides a Firecrawl-compatible API for web scraping and content extraction.

## Features

- Web crawling using Colly (queued jobs, configurable workers)
- HTML to Markdown conversion
- Multiple backends: MongoDB or SQL (PostgreSQL / SQLite via GORM)
- Optional user authentication with API keys
- REST API under `/v1` (Firecrawl-style)
- Optional HTTP rate limiting, crawl retries, and chromedp/Lightpanda fallback for JS-heavy pages
- Automatic data cleanup and crawl job progress tracking

## Prerequisites

- Go 1.26 or higher (see `go.mod`)
- When `ENABLE_AUTH=true`: a database (MongoDB, PostgreSQL, or SQLite) per configuration

## Installation

1. Clone the repository:

```bash
git clone <repository-url>
cd gocrawl
```

2. Install dependencies:

```bash
go mod tidy
```

3. Set up environment variables by copying `.env` and modifying as needed:

```bash
cp .env .env.local
```

4. Run the application:

```bash
go run ./cmd
```

Or use the Makefile:

```bash
make run      # development
make build    # produces ./bin/gocrawl
```

## Configuration

Environment variables can be set in the `.env` file or as system environment variables.

### Server

| Variable | Description |
|----------|-------------|
| `PORT` | Server port (default: `8080`) |
| `HOST` | Bind address (default: empty; use `0.0.0.0` to listen on all interfaces) |

### Database

| Variable | Description |
|----------|-------------|
| `DATABASE_DRIVER` | `mongo` (default), `postgres`, or `sqlite` |
| `MONGO_URI` | MongoDB URI (default: `mongodb://localhost:27017`) |
| `DB_NAME` | Database name (default: `gocrawl`) |
| `DATABASE_DSN` | Postgres connection string or SQLite file path |
| `SQLITE_PATH` | Alternative SQLite path if `DATABASE_DSN` is empty |

### Security

| Variable | Description |
|----------|-------------|
| `JWT_SECRET` | Secret for JWT-related use |
| `ENABLE_AUTH` | `true` to require API keys on protected routes and to use a real database; `false` runs without `Authorization` and uses an **in-memory** crawl job store (jobs are lost on restart; register/login stay unavailable) |

### Crawler (selected)

| Variable | Description |
|----------|-------------|
| `CRAWL_WORKERS` | Goroutines that drain the crawl job queue (default: same as `MAX_CONCURRENT_CRAWLS`) |
| `MAX_CONCURRENT_CRAWLS` | Default per-job Colly parallelism (default: `10`) |
| `CRAWL_MAX_RETRIES` | HTTP retries for 429/5xx (default: `3`) |
| `LIGHTPANDA_WS_URL` | Optional CDP WebSocket URL for JS fallback scraping |
| `RATE_LIMIT_REQUESTS` / `RATE_LIMIT_WINDOW` | If both set (e.g. `100` and `1m`), enables per-client rate limiting on `/v1` |

### Data retention

- `DATA_RETENTION_DAYS`, `CLEANUP_INTERVAL` — see `internal/config/config.go` for defaults.

## API base URL

All HTTP APIs are mounted at **`/v1`** (not `/api/v1`).

Example base:

```bash
export BASE_URL="http://localhost:8080"
```

## API overview

| Method | Path | Auth (if enabled) | Description |
|--------|------|---------------------|-------------|
| POST | `/v1/auth/register` | No | Create user (requires DB) |
| POST | `/v1/auth/login` | No | Login; response includes `apiKey` |
| POST | `/v1/scrape` | Bearer API key | Single-page scrape |
| POST | `/v1/crawl` | Bearer API key | Enqueue multi-page crawl |
| GET | `/v1/crawl/{id}` | Bearer API key | Job status and results |

## Optional CSS selectors

### Scrape body (`POST /v1/scrape`)

| Field | JSON | Description |
|-------|------|-------------|
| Content root | `contentSelector` | One CSS selector for the node whose HTML is converted to markdown / stored as `html`. |
| Content root | `contentSelectors` | More selectors, tried in order after `contentSelector`. |
| Links list | `linkSelector` | Which elements provide outbound links (default `a[href]`). Use e.g. `main a[href]` to ignore footer/nav. |

If `contentSelector` or `contentSelectors` are set, they **replace** the built-in main-content list (`main`, `article`, …). If no selector matches, the extractor falls back to `body`. `onlyMainContent` applies only when you do **not** set custom content selectors.

### Crawl body (`POST /v1/crawl`)

| Field | JSON | Description |
|-------|------|-------------|
| Link discovery | `linkSelectors` | Only enqueue links that match these selectors (e.g. `article a[href]`, `.post-list a`). When empty, the crawler uses the default article/main heuristics plus all `a[href]`. |
| Per page | `scrapeOptions` | Same fields as scrape: `contentSelector`, `contentSelectors`, `linkSelector`, `onlyMainContent`, `formats`, etc., applied to each fetched page. |

## Testing with curl

The examples below use `$BASE_URL` (default `http://localhost:8080`). With authentication enabled, set `API_KEY` after login.

### 1. Scrape a single page (auth disabled)

When `ENABLE_AUTH=false`, omit `Authorization`:

```bash
curl -sS -X POST "${BASE_URL:-http://localhost:8080}/v1/scrape" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com",
    "onlyMainContent": true,
    "formats": ["markdown", "html"]
  }' | jq .
```

### 2. Register and log in (auth enabled)

Requires MongoDB or SQL configured and `ENABLE_AUTH=true`.

```bash
curl -sS -X POST "${BASE_URL:-http://localhost:8080}/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"demo-secret"}' | jq .

curl -sS -X POST "${BASE_URL:-http://localhost:8080}/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"demo-secret"}' | jq .
```

Save the API key from the login response:

```bash
export API_KEY="$(curl -sS -X POST "${BASE_URL:-http://localhost:8080}/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"demo-secret"}' | jq -r .data.apiKey)"
echo "$API_KEY"
```

### 3. Scrape with API key

```bash
curl -sS -X POST "${BASE_URL:-http://localhost:8080}/v1/scrape" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${API_KEY}" \
  -d '{
    "url": "https://hehemetal.com",
    "onlyMainContent": true,
    "formats": ["markdown"]
  }' | jq .
```

Successful responses use the wrapper: `{"success":true,"data":{...}}` (or `success:false` with `error`).

### 4. Start a crawl job

Returns `id` immediately; workers process the queue in the background.

```bash
curl -sS -X POST "${BASE_URL:-http://localhost:8080}/v1/crawl" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${API_KEY}" \
  -d '{
    "url": "https://hehemetal.com",
    "limit": 5,
    "maxDepth": 1,
    "maxConcurrency": 2,
    "delay": 500
  }' | jq .
```

Example response shape:

```json
{"success":true,"id":"<uuid>","url":"https://hehemetal.com"}
```

### 5. Poll crawl status and results

Use the **`id` from the crawl POST response** (not another UUID). A wrong or expired id returns HTTP 404.

```bash
JOB_ID="<paste-id-from-crawl-response>"

curl -sS "${BASE_URL:-http://localhost:8080}/v1/crawl/${JOB_ID}" \
  -H "Authorization: Bearer ${API_KEY}" | jq .
```

Response includes `status` (`queued`, `crawling`, `completed`, …), `total`, `completed`, and `data` (array of page results).

### 6. Quick one-liner (auth off)

```bash
curl -sS -X POST "http://localhost:8080/v1/scrape" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com","onlyMainContent":true,"formats":["markdown"]}'
```

### 7. Scrape with custom selectors

```bash
curl -sS -X POST "http://localhost:8080/v1/scrape" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/article",
    "contentSelector": "article",
    "linkSelector": "article a[href]",
    "formats": ["markdown","html"]
  }' | jq .
```

### 8. Crawl with restricted link discovery

```bash
curl -sS -X POST "http://localhost:8080/v1/crawl" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/blog",
    "limit": 20,
    "maxDepth": 2,
    "linkSelectors": ["article a[href]", ".pagination a[href]"],
    "scrapeOptions": {
      "onlyMainContent": true,
      "contentSelector": "article",
      "formats": ["markdown"]
    }
  }' | jq .
```

## Response shapes (reference)

**Scrape** (`writeResponse`):

```json
{
  "success": true,
  "data": {
    "markdown": "...",
    "html": "...",
    "rawHtml": "...",
    "links": ["https://example.com/..."],
    "metadata": { "title": "...", "sourceURL": "..." }
  }
}
```

**Start crawl** (raw JSON, not wrapped):

```json
{ "success": true, "id": "<job-uuid>", "url": "https://example.com" }
```

**Crawl status** (raw JSON):

```json
{
  "status": "completed",
  "total": 3,
  "completed": 3,
  "creditsUsed": 0,
  "expiresAt": "2026-03-30T12:00:00Z",
  "data": [ { "markdown": "...", "metadata": { } } ]
}
```

## Project structure (abbreviated)

```
gocrawl/
├── cmd/main.go              # Entrypoint, routes, DB wiring
├── internal/
│   ├── api/                 # HTTP handlers, crawl manager, middleware
│   ├── config/
│   ├── crawler/             # Colly scrape, retries, chromedp fallback
│   ├── extractor/           # HTML → Markdown
│   ├── db/                  # Store interface, Mongo + GORM SQL
│   ├── user/
│   └── mcp/
├── Makefile                 # build, test, run, fmt, vet
├── go.mod
└── README.md
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make all` (or `make fmt vet test build`)
5. Submit a pull request

## License

This project is licensed under the MIT License.

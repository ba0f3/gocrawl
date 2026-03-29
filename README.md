# GoCrawl - Firecrawl-like Web Crawler

A production-grade Go web crawler that provides a Firecrawl-compatible API for web scraping and content extraction.

## Features

- Web crawling using Colly (queued jobs, configurable workers)
- HTML to Markdown conversion
- Multiple backends: MongoDB or SQL (PostgreSQL / SQLite via GORM)
- Optional user authentication with API keys
- REST API under `/v1` (Firecrawl-style)
- Optional HTTP rate limiting, crawl retries, and chromedp/Lightpanda (auto fallback for CSR, HTTP 401/403/429/503, challenge pages, or `forceBrowser`)
- Automatic data cleanup and crawl job progress tracking

## Quick start (Docker / GHCR)

Run the prebuilt image (no database needed with `ENABLE_AUTH=false`):

```bash
docker pull ghcr.io/ba0f3/gocrawl:latest

docker run --rm -p 8151:8151 \
  -e PORT=8151 \
  -e HOST=0.0.0.0 \
  -e ENABLE_AUTH=false \
  ghcr.io/ba0f3/gocrawl:latest
```

The API is at `http://localhost:8151/v1/...` (for example `POST /v1/scrape`). For private images, `docker login ghcr.io` first; for auth and databases, see [Docker installation](#docker-installation) and **Configuration**.

## Prerequisites

- **From source:** Go 1.26 or higher (see `go.mod`)
- **Docker:** optional — see [Docker installation](#docker-installation) to run a prebuilt image or build locally
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

## Docker installation

Requires [Docker](https://docs.docker.com/get-docker/). Optional: [Docker Compose](https://docs.docker.com/compose/) for `docker-compose.yml`.

### Pull a prebuilt image (GHCR)

CI publishes images to GitHub Container Registry on pushes to `main` and version tags (`v*`). Replace `<owner>` with your GitHub user or organization (lowercase).

```bash
docker pull ghcr.io/<owner>/gocrawl:latest
```

If the package is private, sign in first:

```bash
echo "$GITHUB_TOKEN" | docker login ghcr.io -u USERNAME --password-stdin
```

Run the container (auth disabled; in-memory crawl jobs — same behavior as a local dev run without a database):

```bash
docker run --rm -p 8151:8151 \
  -e PORT=8151 \
  -e HOST=0.0.0.0 \
  -e ENABLE_AUTH=false \
  ghcr.io/<owner>/gocrawl:latest
```

The API is available at `http://localhost:8151/v1/...`. For `ENABLE_AUTH=true`, pass database settings (`DATABASE_DRIVER`, `MONGO_URI` or `DATABASE_DSN`, etc.) — see **Configuration** below.

### Build the image locally

From the repository root:

```bash
docker build -t gocrawl:local .
docker run --rm -p 8151:8151 -e PORT=8151 -e HOST=0.0.0.0 -e ENABLE_AUTH=false gocrawl:local
```

### Docker Compose

Base stack (`docker-compose.yml`): **gocrawl** on port `8151` and **lightpanda** on `9222` (optional JS/CDP fallback). MongoDB is **not** included.

1. Copy the environment template and edit values (see **Configuration** below):

```bash
cp .env.example .env
```

2. Start:

```bash
docker compose up --build -d
```

Compose loads `.env` when present (`env_file`); variables also support defaults in `docker-compose.yml`. For a typical no-auth scrape API, keep `ENABLE_AUTH=false` in `.env` (no database required). The **gocrawl** service defaults **`LIGHTPANDA_HTTP_URL=http://lightpanda:9222`** so chromedp can resolve the CDP WebSocket from Lightpanda’s `/json/version` without pasting a full `ws://…` URL (override or clear the variable if you do not run Lightpanda).

#### Optional MongoDB (`docker-compose.mongo.yml`)

Use this **only** when you want MongoDB in Compose (for example `ENABLE_AUTH=true` with `DATABASE_DRIVER=mongo`).

1. In `.env`, set at least:

```bash
DATABASE_DRIVER=mongo
MONGO_URI=mongodb://mongo:27017
ENABLE_AUTH=true
JWT_SECRET=<a-long-random-secret>
```

2. Start both files:

```bash
docker compose -f docker-compose.yml -f docker-compose.mongo.yml up --build -d
```

The overlay adds the **mongo** service and makes **gocrawl** wait until Mongo is healthy. For Postgres or SQLite instead, run your database elsewhere and set `DATABASE_DRIVER` / `DATABASE_DSN` (or `SQLITE_PATH`) in `.env` without the Mongo overlay.

## Configuration

Environment variables can be set in the `.env` file or as system environment variables.

### Server

| Variable | Description |
|----------|-------------|
| `PORT` | Server port (default: `8151`) |
| `HOST` | Bind address (default: empty; use `0.0.0.0` to listen on all interfaces) |
| `GIN_MODE` | `debug` for verbose Gin logging; otherwise release mode (default) |
| `SERVER_READ_HEADER_TIMEOUT` | Max time to read request headers (default: `10s`; mitigates slow clients) |
| `SERVER_READ_TIMEOUT` | Max time to read the full request including body (default: `60s`) |
| `SERVER_WRITE_TIMEOUT` | Max time to write the response; `0` or unset means no limit (default), which suits long scrapes and SSE |
| `SERVER_IDLE_TIMEOUT` | Keep-alive idle timeout (default: `120s`) |
| `SERVER_MAX_HEADER_BYTES` | Maximum request header size in bytes (default: `1048576`) |
| `SERVER_SHUTDOWN_TIMEOUT` | Graceful shutdown deadline after SIGINT/SIGTERM (default: `30s`) |

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
| `CRAWL_TIMEOUT` | Default per-request timeout (e.g. `30s`) |
| `USER_AGENT` | User-Agent for Colly and chromedp |
| `CRAWL_MAX_RETRIES` | HTTP retries for 429/5xx (default: `3`) |
| `LIGHTPANDA_WS_URL` | Optional full CDP WebSocket URL (e.g. from `GET http://host:9222/json/version` → `webSocketDebuggerUrl`) |
| `LIGHTPANDA_HTTP_URL` | Optional HTTP base (e.g. `http://lightpanda:9222`); first chromedp use resolves and caches `webSocketDebuggerUrl` |
| `CHROMEDP_AUTO_FALLBACK` | When `true` (default), automatically use chromedp after Colly when heuristics match (thin main text, SPA shell, challenge HTML, configured status codes, errors). Set `false` to only use chromedp with JSON `forceBrowser: true` |
| `CHROMEDP_FALLBACK_STATUS_CODES` | Comma-separated HTTP statuses that trigger auto fallback (default `401,403,429,503` if unset) |
| `CHROMEDP_MAX_CONCURRENT` | Max concurrent chromedp sessions (default `8` if unset or `0`) |
| `CHROMEDP_LOAD_WAIT_TIMEOUT` | Max time to poll `document.readyState` after navigation (default `30s`; capped under the scrape timeout). Stops on **`complete`** or on two consecutive **`interactive`** reads (SPAs often never reach `complete` after XHRs, which used to spin until Lightpanda’s CDP timeout). Short `Evaluate` calls only |
| `CHROMEDP_NAV_WAIT` | Post-load settle before hydration (default `500ms` if unset; set `0s` to disable). Longer waits are **paced** with tiny CDP pings so idle sessions are less likely to hit Lightpanda’s **CDP timeout** |
| `CHROMEDP_HYDRATION_POLL_INTERVAL` | Delay between hydration polls (default `300ms`, also paced with CDP pings) |
| `CHROMEDP_HYDRATION_MAX_POLLS` | Max hydration polls (default `22`) |
| `CHROMEDP_HYDRATION_MIN_TEXT_RUNES` | Stop polling when main-selector text length reaches this (default `80`) |
| `RATE_LIMIT_REQUESTS` / `RATE_LIMIT_WINDOW` | If both set (e.g. `100` and `1m`), enables per-client rate limiting on `/v1` |

**Chromedp limitations:** Running pages in Lightpanda helps with **client-side rendering** and some **simple bot or challenge pages**, but it does **not** guarantee bypass of advanced anti-bot (CAPTCHA vendors, strict TLS/JA3 fingerprinting, etc.). Difficult sites may need proxies, a full browser profile, or manual steps.

### Data retention

- `DATA_RETENTION_DAYS`, `CLEANUP_INTERVAL` — see `internal/config/config.go` for defaults.

## API base URL

All HTTP APIs are mounted at **`/v1`** (not `/api/v1`).

Example base:

```bash
export BASE_URL="http://localhost:8151"
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
| Output shapes | `formats` | Array: `"markdown"`, `"html"`, `"rawHtml"`. Only requested fields are filled; others are omitted from JSON (`omitempty`). **If `formats` is omitted or empty**, all three are returned (backward compatible). Example: `["markdown"]` returns only `markdown` (plus `metadata`, `links`). |
| Main vs full page | `onlyMainContent` | **Omitted** or `true`: try built-in selectors (`main`, `article`, …) then `body` if none match. **`false`**: use the entire `<body>` (full page). Omitting the field now defaults to main-content mode (previously a JSON quirk made omitted behave like `false`). |
| Content root | `contentSelector` | One CSS selector for the node whose HTML is converted to markdown / stored as `html`. |
| Content root | `contentSelectors` | More selectors, tried in order after `contentSelector`. |
| Links list | `linkSelector` | Optional. **Omitted:** collect links only inside the same DOM subtree as the extracted content (e.g. only under `<article>` when that block is used for markdown), with **URLs de-duplicated**. **Set:** run this selector against the **full page** (e.g. `a[href]` for every anchor on the page). |
| Browser-only | `forceBrowser` | When `true` and `LIGHTPANDA_WS_URL` or `LIGHTPANDA_HTTP_URL` is set, **skips the Colly HTTP fetch** and loads the page with chromedp only. Metadata includes `chromedpTrigger: force_browser`. |

If `contentSelector` or `contentSelectors` are set, they **replace** the built-in main-content list (`main`, `article`, …). If no selector matches, the extractor falls back to `body`. `onlyMainContent` applies only when you do **not** set custom content selectors.

When chromedp runs (auto fallback or `forceBrowser`), **`metadata`** may include `extractor: chromedp` and **`chromedpTrigger`** (`thin_markdown`, `spa_shell`, `csr_framework` for Vue/React/Next/Nuxt/Angular/SvelteKit/Remix/Astro/Vite-style shells, `challenge_html`, `status_403`, `visit_error`, etc.).

### Crawl body (`POST /v1/crawl`)

| Field | JSON | Description |
|-------|------|-------------|
| Link discovery | `linkSelector` | Single CSS selector for the same purpose as `linkSelectors` (convenience). |
| Link discovery | `linkSelectors` | Only enqueue links that match these selectors (e.g. `article a[href]`, `.post-list a`). When both `linkSelector` and `linkSelectors` are set, `linkSelector` is tried first. When empty, the crawler uses the default article/main heuristics plus all `a[href]`. |
| Per page | `scrapeOptions` | Same fields as scrape: `contentSelector`, `contentSelectors`, `linkSelector`, `onlyMainContent`, `formats`, etc., applied to each fetched page. If you set `linkSelectors` but omit `scrapeOptions.linkSelector`, the same selectors are reused when collecting `links` on each scraped page (Colly debug logs will show that selector instead of only `a[href]`). |

## Testing with curl

The examples below use `$BASE_URL` (default `http://localhost:8151`). With authentication enabled, set `API_KEY` after login.

### 1. Scrape a single page (auth disabled)

When `ENABLE_AUTH=false`, omit `Authorization`:

```bash
curl -sS -X POST "${BASE_URL:-http://localhost:8151}/v1/scrape" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://hehemetal.com",
    "onlyMainContent": true,
    "formats": ["markdown", "html"]
  }' | jq .
```

### 2. Register and log in (auth enabled)

Requires MongoDB or SQL configured and `ENABLE_AUTH=true`.

```bash
curl -sS -X POST "${BASE_URL:-http://localhost:8151}/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"demo-secret"}' | jq .

curl -sS -X POST "${BASE_URL:-http://localhost:8151}/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"demo-secret"}' | jq .
```

Save the API key from the login response:

```bash
export API_KEY="$(curl -sS -X POST "${BASE_URL:-http://localhost:8151}/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"demo-secret"}' | jq -r .data.apiKey)"
echo "$API_KEY"
```

### 3. Scrape with API key

```bash
curl -sS -X POST "${BASE_URL:-http://localhost:8151}/v1/scrape" \
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
curl -sS -X POST "${BASE_URL:-http://localhost:8151}/v1/crawl" \
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

curl -sS "${BASE_URL:-http://localhost:8151}/v1/crawl/${JOB_ID}" \
  -H "Authorization: Bearer ${API_KEY}" | jq .
```

Response includes `status` (`queued`, `crawling`, `completed`, …), `total`, `completed`, and `data` (array of page results).

### 6. Quick one-liner (auth off)

```bash
curl -sS -X POST "http://localhost:8151/v1/scrape" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://hehemetal.com","onlyMainContent":true,"formats":["markdown"]}'
```

### 7. Scrape with custom selectors

```bash
curl -sS -X POST "http://localhost:8151/v1/scrape" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://hehemetal.com/news/buc-tuong-duoc-duc-bang-bac-cua-helios",
    "contentSelector": "article",
    "linkSelector": "article a[href]",
    "formats": ["markdown","html"]
  }' | jq .
```

### 8. Crawl with restricted link discovery

```bash
curl -sS -X POST "http://localhost:8151/v1/crawl" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://hehemetal.com/category/news",
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
    "links": ["https://hehemetal.com/..."],
    "metadata": { "title": "...", "sourceURL": "..." }
  }
}
```

**Start crawl** (raw JSON, not wrapped):

```json
{ "success": true, "id": "<job-uuid>", "url": "https://hehemetal.com" }
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

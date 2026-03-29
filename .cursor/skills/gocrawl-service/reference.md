# GoCrawl integration guide (coding agents)

This document explains how to run GoCrawl and how **`/v1/scrape`** and **`/v1/crawl`** behave so clients can be implemented correctly.

## 1. Docker Compose setup

### Base stack (no MongoDB)

The default `docker-compose.yml` runs **gocrawl** (port **8080**) and **lightpanda** (port **9222**, optional CDP/JS fallback). MongoDB is **not** included.

1. From the repository root, create a local env file:

```bash
cp .env.example .env
```

2. For a simple scrape API without login, keep **`ENABLE_AUTH=false`** in `.env`. No database is required.

3. Start:

```bash
docker compose up --build -d
```

4. Base URL for HTTP calls: **`http://localhost:8080`** (or your host). API routes live under **`/v1`**.

5. **Lightpanda / chromedp:** With Compose, **`LIGHTPANDA_HTTP_URL`** defaults to **`http://lightpanda:9222`** on the **gocrawl** service so the process can resolve the CDP socket via **`/json/version`** (no manual `ws://…` paste). Alternatively set **`LIGHTPANDA_WS_URL`** to the full `webSocketDebuggerUrl`. Auto fallback uses chromedp after Colly for errors, selected HTTP statuses (401/403/429/503 by default), challenge-style HTML, SPA shells, or very thin main markdown; set **`CHROMEDP_AUTO_FALLBACK=false`** to disable that and use only JSON **`forceBrowser: true`**. After navigation, chromedp polls **`document.readyState`** with short CDP calls until **`complete`** or **stable `interactive`** (many SPAs never fire `complete` after API calls), then optional **`CHROMEDP_NAV_WAIT`** and hydration delays are **paced** with tiny CDP evaluations so Lightpanda is less likely to log **CDP timeout** on an idle socket while the page finishes XHR-driven rendering. See `README.md` (Configuration) for **`CHROMEDP_*`** tuning and anti-bot limitations.

### Optional MongoDB overlay

Use **`docker-compose.mongo.yml`** only when you need MongoDB in Compose (typically **`ENABLE_AUTH=true`** with **`DATABASE_DRIVER=mongo`**).

In `.env`, set at least:

```bash
DATABASE_DRIVER=mongo
MONGO_URI=mongodb://mongo:27017
ENABLE_AUTH=true
JWT_SECRET=<long-random-secret>
```

Start:

```bash
docker compose -f docker-compose.yml -f docker-compose.mongo.yml up --build -d
```

The overlay adds the **mongo** service and makes **gocrawl** wait until Mongo is healthy. For PostgreSQL or SQLite, run the database elsewhere and set **`DATABASE_DRIVER`** / **`DATABASE_DSN`** (or **`SQLITE_PATH`**) without this overlay.

### Prebuilt image (no Compose)

```bash
docker pull ghcr.io/ba0f3/gocrawl:latest
docker run --rm -p 8080:8080 \
  -e PORT=8080 -e HOST=0.0.0.0 -e ENABLE_AUTH=false \
  ghcr.io/ba0f3/gocrawl:latest
```

## 2. Authentication and storage

| `ENABLE_AUTH` | Database | Crawl jobs | Client |
|---------------|----------|------------|--------|
| `false` | Not required | **In-memory** (lost on restart) | Omit `Authorization` on `/v1/scrape`, `/v1/crawl`, etc. |
| `true` | Required (Mongo / Postgres / SQLite) | **Persisted** in DB | Register → login → `Authorization: Bearer <apiKey>` |

When auth is off, the server may synthesize an internal user id for jobs; polling still uses the returned **`id`**.

## 3. Response shapes

### Most endpoints (`writeResponse`)

Successful JSON looks like:

```json
{"success": true, "data": { ... }}
```

Errors:

```json
{"success": false, "error": "message"}
```

### Crawl enqueue (`POST /v1/crawl`)

This handler returns **raw JSON** (not the `data` wrapper):

```json
{"success": true, "id": "<job-uuid>", "url": "https://..."}
```

Use **`id`** for **`GET /v1/crawl/{id}`**.

### Crawl status (`GET /v1/crawl/{id}`)

Returns **`CrawlStatusResponse`** directly (no `success` wrapper): `status`, `total`, `completed`, `creditsUsed`, `expiresAt`, optional `data` (array of per-page results).

## 4. How `/v1/scrape` works

- **Purpose:** Fetch **one URL**, extract content (Markdown/HTML), metadata, and links — **synchronously** in the HTTP request.
- **Method:** `POST`
- **Body:** `ScrapeRequest` JSON (see below).
- **Flow:** unless **`forceBrowser: true`**, HTTP fetch (Colly) → HTML parse → optional chromedp fallback if Lightpanda is configured and auto-heuristics match (or always when **`forceBrowser`**) → JSON response in **`data`**. Check **`metadata.chromedpTrigger`** when **`extractor`** is **`chromedp`** (e.g. **`spa_shell`** for empty mount nodes, **`csr_framework`** when HTML matches Vue/React/Next/Nuxt/Angular/SvelteKit/Remix/Astro/Vite-style signatures).

### `ScrapeRequest` (main fields)

| Field | Meaning |
|-------|--------|
| `url` | **Required.** Page to scrape. |
| `formats` | Array: `"markdown"`, `"html"`, `"rawHtml"`. **Empty or omitted** = all three (legacy). Otherwise only listed fields are populated (`omitempty`). |
| `onlyMainContent` | Omitted or `true`: main/article-style extraction; `false`: full `<body>` when not using custom content selectors. |
| `contentSelector` / `contentSelectors` | CSS root(s) for extracted content; when set, replace default main-content heuristics. |
| `linkSelector` | Optional; controls which anchors are collected into `links` (see README). |
| `includeTags` / `excludeTags` | Tag filters where supported. |
| `timeout` | Per-request timeout hint where used. |
| `removeBase64Images` | Strip base64 images from output when true. |
| `forceBrowser` | If `true` and Lightpanda env is set, load the page with chromedp only (skip Colly). |

Use **`POST /v1/scrape`** when you need a **single page** result immediately.

## 5. How `/v1/crawl` works

- **Purpose:** **Multi-page** crawl: discover links from a seed URL, visit up to **`limit`** pages, respect **depth** and filters, run per-page extraction using **`scrapeOptions`** (same idea as scrape).
- **Method:** `POST` enqueues a **job**; workers process asynchronously.
- **Polling:** `GET /v1/crawl/{id}` with **`id` from the POST response** until `status` is terminal (e.g. `completed` or `failed`).

### Defaults (server-side if omitted in JSON)

From `internal/api/routes.go`:

- **`maxDepth`:** default **10** if `0`
- **`limit`:** default **10000** if `0`
- **`maxConcurrency`:** default **10** if `0`

### `CrawlRequestBody` (main fields)

| Field | Meaning |
|-------|--------|
| `url` | Seed URL. |
| `limit` | Max pages to fetch. |
| `maxDepth` | Link depth from seed. |
| `maxConcurrency` | Colly parallelism for this job. |
| `delay` | Delay between requests (**milliseconds**; see `crawl_manager.go`). |
| `linkSelector` / `linkSelectors` | Restrict which anchors enqueue new URLs. |
| `scrapeOptions` | Same shape as **`ScrapeRequest`** — applied to **each** crawled page. |
| `excludePaths` / `includePaths` | Path filters. |
| `allowExternalLinks` / `allowSubdomains` / `crawlEntireDomain` / etc. | Crawl boundary rules (see `CrawlRequestBody` in `internal/api/routes.go`). |

### Job lifecycle

1. **`POST /v1/crawl`** creates a job (`queued`), returns **`id`**.
2. Workers **`ClaimNextQueuedJob`**, set status to **`crawling`**, run Colly + scrape pipeline, append **`CrawlResult`** rows, then **`completed`** (or **`failed`** on error).

## 6. MCP routes (JSON over HTTP)

Same auth rules as REST. Paths under **`/v1/mcp`**:

- `POST /v1/mcp/scrape`
- `POST /v1/mcp/crawl`
- `GET /v1/mcp/stats`

Use when integrating MCP-style clients; semantics align with scrape/crawl.

## 7. Minimal client checklist

1. Set **`BASE_URL`** (e.g. `http://localhost:8080`).
2. If auth is on: register, login, store **`apiKey`**, send **`Authorization: Bearer <apiKey>`**.
3. Single page: **`POST /v1/scrape`** with `url` (+ optional `formats`, selectors).
4. Multi page: **`POST /v1/crawl`**, read **`id`**, loop **`GET /v1/crawl/{id}`** until done, read **`data`** array.
5. Do not use `/api/v1` — use **`/v1`** only.

## 8. Further reading in the repo

- **`README.md`** — curl examples, selector tables, configuration.
- **`AGENTS.md`** — layout, CI, conventions.
- **`internal/api/routes.go`** — exact JSON types for crawl and responses.
- **`internal/crawler/colly.go`** — `ScrapeRequest` / scrape pipeline.

---
name: gocrawl-service
description: >-
  Calls and integrates with the GoCrawl HTTP API (Firecrawl-style scrape/crawl under /v1).
  Use when the user runs or points to gocrawl, scrapes URLs via an API, needs curl or
  client examples for /v1/scrape or /v1/crawl, Docker Compose setup, authentication, or
  understanding how scrape vs crawl jobs behave.
---

# GoCrawl service (agent reference)

GoCrawl is a Go HTTP service: synchronous **scrape** (one URL) and asynchronous **crawl** (many pages). **All REST routes are under `/v1`** (not `/api/v1`).

## Quick facts

```bash
export BASE_URL="http://localhost:8151"
```

| Topic | Summary |
|--------|---------|
| Auth off | No `Authorization`; crawl jobs in-memory only. |
| Auth on | `POST /v1/auth/register` → `POST /v1/auth/login` → `Authorization: Bearer <apiKey>`; DB required. |
| Scrape | `POST /v1/scrape` — one page, synchronous, wrapped as `{"success":true,"data":{...}}`. |
| Crawl | `POST /v1/crawl` — returns `{"success":true,"id":"...","url":"..."}`; poll `GET /v1/crawl/{id}`. |

## Endpoints (REST)

| Method | Path |
|--------|------|
| POST | `/v1/scrape` |
| POST | `/v1/crawl` |
| GET | `/v1/crawl/{id}` |
| POST | `/v1/auth/register` / `/v1/auth/login` (if auth on) |

MCP JSON routes: `POST /v1/mcp/scrape`, `POST /v1/mcp/crawl`, `GET /v1/mcp/stats` (same auth rules).

## Docker (short)

**Compose (no Mongo):** `cp .env.example .env`, set `ENABLE_AUTH=false` for simplest setup, then `docker compose up --build -d`. Compose defaults **`LIGHTPANDA_HTTP_URL=http://lightpanda:9222`** for chromedp auto-discovery; see **reference.md** for `forceBrowser` and fallback behavior.

**Compose + Mongo:** use `-f docker-compose.yml -f docker-compose.mongo.yml` and set `DATABASE_DRIVER`, `MONGO_URI`, `ENABLE_AUTH`, `JWT_SECRET` in `.env`.

**Image only:** `docker pull ghcr.io/ba0f3/gocrawl:latest` and `docker run` with `-e ENABLE_AUTH=false` (see README).

## Full integration guide

For step-by-step Compose, env vars, response envelope differences, **`ScrapeRequest`** / **`CrawlRequestBody`** fields, defaults, and job lifecycle, read **[reference.md](reference.md)** in this folder.

Repository docs: **`README.md`**, **`AGENTS.md`**, **`.env.example`**.

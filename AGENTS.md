# Agent guide — gocrawl

This file orients automated coding agents and contributors to the **gocrawl** repository: a Go HTTP service that crawls/scrapes pages (Colly), extracts Markdown, and exposes Firecrawl-style REST and MCP-oriented endpoints.

## Quick facts

- **Language / module:** Go, module path `gocrawl` (see `go.mod`).
- **Entrypoint:** `cmd/main.go` — loads config (Viper + `.env`), opens a `db.Store` (or in-memory jobs when auth is off), registers Gorilla mux routes, starts `http.ListenAndServe`.
- **Persistence:** `internal/db.Store` — Mongo (`DATABASE_DRIVER=mongo` + `MONGO_URI`) or Postgres/SQLite via GORM (`DATABASE_DRIVER=postgres|sqlite` with `DATABASE_DSN` / `SQLITE_PATH`). When `ENABLE_AUTH=false`, crawl jobs use `MemoryStore` (no users or API keys; see `cmd/main.go` and `internal/db`).
- **Product spec / roadmap:** `PLAN.md` describes the original Firecrawl-like goals; the tree under `internal/` is the source of truth for what is implemented today.

## Layout

| Path | Role |
|------|------|
| `cmd/main.go` | Wiring: config, DB init, middleware, route registration. |
| `internal/api/` | HTTP handlers, crawl manager, middleware (auth, CORS, logging). |
| `internal/config/` | `config.Load()` and env-backed structs (server, database, security, crawler, retention, rate limits, SSE). |
| `internal/crawler/` | Colly-based crawl/scrape execution; chromedp remote (Lightpanda) with auto fallback and `forceBrowser` (see `README.md` / `LIGHTPANDA_*`, `CHROMEDP_*`). |
| `internal/extractor/` | HTML → Markdown and related extraction. |
| `internal/db/` | `Store` interface; Mongo and GORM (Postgres/SQLite) implementations, indexes, cleanup routine. |
| `internal/user/` | Registration, login, API keys. |
| `internal/mcp/` | MCP-related server/SSE integration used by the API layer. |
| `examples/mcp_client.go` | Example MCP client usage. |

HTTP routes are mounted under **`/v1`** (not `/api/v1` in code): e.g. `POST /v1/scrape`, `POST /v1/crawl`, `GET /v1/crawl/{id}`, auth under `/v1/auth/*`, optional `GET /v1/sse`, and MCP-style paths under `/v1/mcp/*`.

## Run and verify

- **Dependencies:** `go mod tidy`
- **Run:** `go run ./cmd/main.go` (ensure `.env` or env vars match `README.md`; a database is required when auth is enabled).
- **Tests:** `go test ./...`

**CI (GitHub Actions):**

- `.github/workflows/go.yml` — build and test on push/PR to `main`.
- `.github/workflows/docker-ghcr.yml` — build the `Dockerfile` image and push to `ghcr.io/<owner>/gocrawl` on pushes to `main`, tags `v*`, and manual runs (`latest`, SHA, semver tags as configured in the workflow).

**Docker:** local builds use `Dockerfile`, `docker-compose.yml` (gocrawl + lightpanda), and optional `docker-compose.mongo.yml` for MongoDB. Published images follow the GHCR workflow above.

## MCP / IDE integration

- `mcp-server.json` describes how to run or attach an MCP server for this project; use it when wiring editors or external MCP clients.

## Conventions for changes

- Keep new code and comments in **English**.
- Prefer **small, focused changes** that match existing patterns in `internal/` (error handling, logging, handler shape).
- Do not commit secrets; configuration belongs in env / `.env` (see `.gitignore` if present).
- **Keep docs in sync with code changes.** When you add or change behavior, HTTP/MCP routes, request/response shapes, env vars, Docker/Compose, or CI: update the same change set with the relevant docs — typically **`README.md`**, **`AGENTS.md`** (quick facts, layout, CI, or conventions if affected), **`.env.example`** (new or renamed variables), **`.cursor/skills/gocrawl-service/`** (`SKILL.md` / `reference.md` when integration or API usage for agents changes), and **`.github/workflows/`** when pipelines or published images change. Do not leave user-facing or agent-facing docs stale after a feature or fix that alters how the service is run or called.
- When behavior or storage diverges from `PLAN.md` (e.g. Mongo vs. older plan text), **trust the code**; update `PLAN.md` only if you are aligning it with reality or the user asks.

## Further reading

- `README.md` — setup, env vars, API overview.
- `PLAN.md` — original requirements and API examples (may lag implementation in places).
- `.cursor/skills/gocrawl-service/` — agent-oriented API and Docker Compose integration (`SKILL.md`, `reference.md`).

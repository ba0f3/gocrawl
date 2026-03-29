# Agent guide — gocrawl

This file orients automated coding agents and contributors to the **gocrawl** repository: a Go HTTP service that crawls/scrapes pages (Colly), extracts Markdown, and exposes Firecrawl-style REST and MCP-oriented endpoints.

## Quick facts

- **Language / module:** Go, module path `gocrawl` (see `go.mod`).
- **Entrypoint:** `cmd/main.go` — loads config (Viper + `.env`), optionally connects to MongoDB, registers Gorilla mux routes, starts `http.ListenAndServe`.
- **Persistence:** MongoDB for users, API keys, and crawl jobs when auth is enabled. With `ENABLE_AUTH=true`, the server can run without a database (see `cmd/main.go` and `internal/db`).
- **Product spec / roadmap:** `PLAN.md` describes the original Firecrawl-like goals; the tree under `internal/` is the source of truth for what is implemented today.

## Layout

| Path | Role |
|------|------|
| `cmd/main.go` | Wiring: config, DB init, middleware, route registration. |
| `internal/api/` | HTTP handlers, crawl manager, middleware (auth, CORS, logging). |
| `internal/config/` | `config.Load()` and env-backed structs (server, Mongo, security, crawler, retention, rate limits, SSE). |
| `internal/crawler/` | Colly-based crawl/scrape execution. |
| `internal/extractor/` | HTML → Markdown and related extraction. |
| `internal/db/` | Mongo access, indexes, cleanup routine. |
| `internal/user/` | Registration, login, API keys. |
| `internal/mcp/` | MCP-related server/SSE integration used by the API layer. |
| `examples/mcp_client.go` | Example MCP client usage. |

HTTP routes are mounted under **`/v1`** (not `/api/v1` in code): e.g. `POST /v1/scrape`, `POST /v1/crawl`, `GET /v1/crawl/{id}`, auth under `/v1/auth/*`, optional `GET /v1/sse`, and MCP-style paths under `/v1/mcp/*`.

## Run and verify

- **Dependencies:** `go mod tidy`
- **Run:** `go run ./cmd/main.go` (ensure `.env` or env vars match `README.md`; Mongo required unless auth is disabled).
- **Tests:** `go test ./...`

Docker: `Dockerfile` and `docker-compose.yml` support containerized runs; see comments in those files for ports and services.

## MCP / IDE integration

- `mcp-server.json` describes how to run or attach an MCP server for this project; use it when wiring editors or external MCP clients.

## Conventions for changes

- Keep new code and comments in **English**.
- Prefer **small, focused changes** that match existing patterns in `internal/` (error handling, logging, handler shape).
- Do not commit secrets; configuration belongs in env / `.env` (see `.gitignore` if present).
- When behavior or storage diverges from `PLAN.md` (e.g. Mongo vs. older plan text), **trust the code** and update docs only when the user asks or when fixing clear inaccuracies.

## Further reading

- `README.md` — setup, env vars, API overview.
- `PLAN.md` — original requirements and API examples (may lag implementation in places).

# Product Overview

GoCrawl is a production-grade web crawler and scraper that provides a Firecrawl-compatible API. It's designed as a self-hosted alternative to Firecrawl cloud service, implementing all crawling, extraction, and API functionality in-house.

## Core Features

- **Web Scraping**: Single page scraping with content extraction and markdown conversion
- **Web Crawling**: Multi-page crawling with configurable depth and concurrency
- **Content Processing**: HTML to Markdown conversion with metadata extraction
- **User Management**: Registration, authentication, and API key management
- **Data Persistence**: MongoDB storage with automatic cleanup
- **Real-time Updates**: Server-Sent Events (SSE) for crawl progress
- **MCP Integration**: Model Context Protocol server capabilities

## API Compatibility

The service implements Firecrawl-compatible endpoints:
- `POST /v1/scrape` - Single page scraping
- `POST /v1/crawl` - Start crawl jobs
- `GET /v1/crawl/{id}` - Get crawl status and results

## Deployment Options

- Standalone binary
- Docker container with MongoDB
- Development mode with authentication disabled
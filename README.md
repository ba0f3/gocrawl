# GoCrawl - Firecrawl-like Web Crawler

A production-grade Go web crawler that provides a Firecrawl-compatible API for web scraping and content extraction.

## Features

- 🔄 Web crawling using Colly
- 📝 HTML to Markdown conversion
- 🗄️ MongoDB for data persistence
- 🔐 User authentication with API keys (optional)
- 🚀 RESTful API compatible with Firecrawl
- 🧹 Automatic data cleanup
- 📊 Progress tracking for crawl jobs

## Prerequisites

- Go 1.19 or higher
- MongoDB (local or remote)

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

4. Start MongoDB (if running locally):
```bash
mongod
```

5. Run the application:
```bash
go run ./cmd/main.go
```

## Configuration

Environment variables can be set in the `.env` file or as system environment variables:

### Server Configuration
- `PORT`: Server port (default: 8080)
- `HOST`: Server host (default: localhost)

### Database Configuration
- `MONGO_URI`: MongoDB connection URI (default: mongodb://localhost:27017)
- `DB_NAME`: Database name (default: gocrawl)

### Security
- `JWT_SECRET`: Secret for JWT tokens
- `DISABLE_AUTH`: Set to `true` to disable authentication (default: false)

### Crawler Configuration
- `MAX_CONCURRENT_CRAWLS`: Maximum concurrent crawl operations (default: 10)
- `CRAWL_TIMEOUT`: Timeout for crawl operations (default: 30s)
- `USER_AGENT`: User agent string (default: GoCrawl/1.0)

### Data Retention
- `DATA_RETENTION_DAYS`: Days to keep crawl data (default: 30)
- `CLEANUP_INTERVAL`: Cleanup interval (default: 24h)

## API Endpoints

### Authentication (if enabled)

#### Register User
```bash
POST /api/v1/auth/register
Content-Type: application/json

{
  "username": "your_username",
  "password": "your_password"
}
```

#### Login
```bash
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "your_username",
  "password": "your_password"
}
```

### Scraping

#### Scrape Single Page
```bash
POST /api/v1/scrape
Content-Type: application/json
Authorization: Bearer <api_key>  # Only if auth is enabled

{
  "url": "https://example.com",
  "onlyMainContent": true,
  "formats": ["markdown"],
  "timeout": 30
}
```

Response:
```json
{
  "success": true,
  "data": {
    "markdown": "# Title\nContent here...",
    "html": "<h1>Title</h1><p>Content here...</p>",
    "rawHtml": "<!DOCTYPE html>...",
    "links": ["https://example.com/link1"],
    "metadata": {
      "title": "Page Title",
      "description": "Page description",
      "sourceURL": "https://example.com"
    }
  }
}
```

#### Start Crawl Job
```bash
POST /api/v1/crawl
Content-Type: application/json
Authorization: Bearer <api_key>  # Only if auth is enabled

# (Not yet implemented)
```

#### Get Crawl Status
```bash
GET /api/v1/crawl/{id}
Authorization: Bearer <api_key>  # Only if auth is enabled

# (Not yet implemented)
```

## Usage Examples

### With Authentication Disabled

Set `DISABLE_AUTH=true` in your `.env` file:

```bash
curl -X POST http://localhost:8080/api/v1/scrape \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com",
    "onlyMainContent": true,
    "formats": ["markdown"]
  }'
```

### With Authentication Enabled

1. Register a user:
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "testpass"
  }'
```

2. Login to get API key:
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "testpass"
  }'
```

3. Use the API key for scraping:
```bash
curl -X POST http://localhost:8080/api/v1/scrape \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-api-key>" \
  -d '{
    "url": "https://example.com",
    "onlyMainContent": true,
    "formats": ["markdown"]
  }'
```

## Project Structure

```
gocrawl/
├── cmd/
│   └── main.go                   # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go             # Configuration management
│   ├── crawler/
│   │   └── colly.go              # Web crawling logic
│   ├── extractor/
│   │   └── markdown.go           # HTML to Markdown conversion
│   ├── api/
│   │   ├── routes.go             # HTTP API handlers
│   │   └── middleware.go         # Authentication & CORS middleware
│   ├── user/
│   │   └── user.go               # User management
│   └── db/
│       └── db.go                 # MongoDB operations
├── .env                          # Environment configuration
├── go.mod                        # Go module file
└── README.md                     # This file
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License.

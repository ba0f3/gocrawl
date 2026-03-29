# Technology Stack

## Core Technologies

- **Language**: Go 1.24+
- **Web Framework**: Gorilla Mux for HTTP routing
- **Database**: MongoDB with official Go driver
- **Configuration**: Viper for environment and config management
- **Web Crawling**: Colly v2 for web scraping
- **Content Processing**: html-to-markdown/v2 for content extraction
- **Authentication**: JWT tokens with bcrypt password hashing
- **Containerization**: Docker with multi-stage builds

## Key Dependencies

```go
github.com/gocolly/colly/v2          // Web crawling engine
github.com/gorilla/mux               // HTTP router
github.com/spf13/viper               // Configuration management
go.mongodb.org/mongo-driver          // MongoDB driver
github.com/JohannesKaufmann/html-to-markdown/v2  // HTML to Markdown
github.com/google/uuid               // UUID generation
golang.org/x/crypto                  // Password hashing
github.com/modelcontextprotocol/go-sdk  // MCP server support
```

## Build & Development Commands

### Local Development
```bash
# Install dependencies
go mod tidy

# Run application
go run ./cmd/main.go

# Build binary
go build -o gocrawl ./cmd/main.go

# Run tests
go test ./...

# Run specific package tests
go test ./internal/extractor -v
```

### Docker Development
```bash
# Build Docker image
docker build -t gocrawl .

# Run with Docker Compose (includes MongoDB)
docker-compose up -d

# View logs
docker-compose logs -f gocrawl

# Stop services
docker-compose down
```

### Configuration
- Environment variables via `.env` file
- Viper automatically loads from `.env` and system environment
- Authentication can be disabled with `ENABLE_AUTH=true` for development

## Architecture Patterns

- **Clean Architecture**: Clear separation between API, business logic, and data layers
- **Dependency Injection**: Dependencies passed through constructors
- **Middleware Pattern**: HTTP middleware for auth, logging, CORS
- **Repository Pattern**: Database operations abstracted behind interfaces
- **Configuration Pattern**: Centralized config management with defaults
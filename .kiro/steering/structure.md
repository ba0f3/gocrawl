# Project Structure

## Directory Layout

```
gocrawl/
├── cmd/
│   └── main.go                   # Application entry point
├── internal/                     # Private application code
│   ├── api/                      # HTTP API layer
│   │   ├── routes.go             # API handlers and routing logic
│   │   ├── middleware.go         # HTTP middleware (auth, CORS, logging)
│   │   └── crawl_manager.go      # Crawl job management
│   ├── config/
│   │   └── config.go             # Configuration structs and loading
│   ├── crawler/
│   │   └── colly.go              # Web crawling implementation using Colly
│   ├── extractor/
│   │   ├── markdown.go           # HTML to Markdown conversion
│   │   └── markdown_test.go      # Content extraction tests
│   ├── db/
│   │   └── db.go                 # MongoDB operations and models
│   ├── user/
│   │   └── user.go               # User management and authentication
│   └── mcp/                      # Model Context Protocol integration
│       ├── server.go             # MCP server implementation
│       └── sse.go                # Server-Sent Events for real-time updates
├── examples/
│   └── mcp_client.go             # MCP client usage examples
├── docs/
│   └── *.md                      # Documentation files
├── .env                          # Environment configuration template
├── .dockerignore                 # Docker ignore patterns
├── Dockerfile                    # Multi-stage Docker build
├── docker-compose.yml            # Development environment with MongoDB
├── mcp-server.json               # MCP server configuration
├── go.mod                        # Go module definition
└── README.md                     # Project documentation
```

## Code Organization Principles

### Package Structure
- **`cmd/`**: Application entry points (main packages)
- **`internal/`**: Private application code, not importable by external packages
- **`examples/`**: Usage examples and sample code
- **`docs/`**: Documentation and migration guides

### Internal Package Guidelines
- **`api/`**: HTTP layer - handlers, middleware, routing
- **`config/`**: Configuration management and validation
- **`crawler/`**: Core crawling logic using Colly
- **`extractor/`**: Content processing and format conversion
- **`db/`**: Database operations and data models
- **`user/`**: Authentication and user management
- **`mcp/`**: Model Context Protocol server functionality

### File Naming Conventions
- Use snake_case for file names: `crawl_manager.go`
- Test files: `*_test.go`
- Main packages in `cmd/` directory
- One main concept per file, related functionality grouped together

### Import Organization
1. Standard library imports
2. Third-party imports
3. Internal package imports (prefixed with module name)

Example:
```go
import (
    "context"
    "fmt"
    "net/http"

    "github.com/gorilla/mux"
    "github.com/spf13/viper"

    "gocrawl/internal/config"
    "gocrawl/internal/db"
)
```

### Configuration Files
- **`.env`**: Environment variables template (not committed with secrets)
- **`docker-compose.yml`**: Development environment setup
- **`mcp-server.json`**: MCP server endpoint definitions
- **`.dockerignore`**: Docker build context exclusions
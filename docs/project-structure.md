# Project Structure

```text
.
├── cmd/
│   └── server/              # Application entry point (package main)
│       ├── main.go          # Entry point, run loop, graceful shutdown
│       ├── server.go        # HTTP server setup, routing, CORS
│       ├── storage.go       # Storage initialization
│       ├── vips.go          # libvips configuration
│       └── logging.go       # Logging setup, server info output
├── internal/
│   ├── cache/               # In-memory and disk cache with TTL & LRU eviction
│   │   ├── disk/            # Disk cache implementation: file I/O, index, cleanup goroutine
│   │   └── memory/          # In-memory cache implementation
│   ├── config/              # Environment/server config loading
│   ├── format/              # Shared formatting utilities
│   ├── handler/             # HTTP request handling
│   ├── logger/              # Leveled logger wrapper
│   ├── operations/          # Image operation types (resize, crop, fit, format, quality)
│   ├── parser/              # URL & filter string parsing
│   ├── processor/           # Image processing orchestration (libvips)
│   ├── signature/           # URL signature generation & verification
│   │   └── hashers/         # Pluggable hash algorithms (SHA-256, SHA-512)
│   └── storage/             # Storage abstraction + cached storage wrapper
│       └── drivers/         # Local filesystem and S3-compatible drivers
├── docs/                    # Project documentation
├── .dockerignore            # Docker build context exclusions
├── .env.example             # Example environment variables
├── docker-compose.yml       # Local/dev service orchestration
├── README.md                # Main project documentation
├── storage.*.json           # Storage configuration examples
├── Dockerfile               # Container image definition
├── go.mod                   # Go module definition
└── go.sum                   # Go dependency checksums
```

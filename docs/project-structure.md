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
├── examples/                # Example configs for runtime setup
│   ├── storage/
│   │   ├── local.example.json
│   │   └── s3.example.json
│   └── systemd/
│       ├── mage-local.service.example
│       └── mage-s3.service.example
├── .dockerignore            # Docker build context exclusions
├── .env.example             # Example environment variables
├── docker-compose.local.yml # Local filesystem compose file
├── docker-compose.s3.yml    # S3-backed compose file
├── README.md                # Main project documentation
├── Dockerfile               # Container image definition
├── go.mod                   # Go module definition
└── go.sum                   # Go dependency checksums
```

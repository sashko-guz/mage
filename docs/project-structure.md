# Project Structure

```text
.
├── cmd/
│   └── server/              # Application entry point
├── internal/
│   ├── cache/               # In-memory (ristretto) and disk cache with TTL & LRU eviction
│   │   └── disk/            # Disk cache implementation: file I/O, index, cleanup goroutine
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
├── .env.example             # Example environment variables
├── docker-compose.yml       # Local/dev service orchestration
├── README.md                # Main project documentation
├── storage.*.json           # Storage configuration examples
├── Dockerfile               # Container image definition
├── go.mod                   # Go module definition
└── go.sum                   # Go dependency checksums
```

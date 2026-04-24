# Project Structure

```text
.
├── cmd/
│   └── mage/
│       └── main.go              # Thin entry point
├── internal/
│   ├── app/                     # Application lifecycle
│   │   ├── app.go               # Bootstrap, Run(), graceful shutdown
│   │   └── vips.go              # libvips configuration
│   ├── config/                  # Unified configuration
│   │   └── config.go            # Load config from env
│   ├── http/                    # HTTP transport layer
│   │   ├── server.go            # HTTP server setup
│   │   ├── router.go            # Route definitions
│   │   └── middleware/          # HTTP middleware
│   │       └── cors.go
│   ├── routes/                  # Route registry
│   │   └── routes.go            # Add(), Match()
│   ├── imaging/                 # Image domain types
│   │   └── operations/          # Image operations + Request type
│   ├── thumbnail/               # Thumbnail domain
│   │   ├── handler/             # HTTP handler
│   │   ├── parser/              # URL parsing
│   │   └── processor/           # Image processing (libvips)
│   ├── storage/                 # Storage layer
│   │   ├── drivers/             # Local, S3 drivers
│   │   ├── cache/               # Caching layer
│   │   │   ├── disk/            # Disk cache
│   │   │   └── memory/          # Memory cache (Ristretto)
│   │   ├── config.go            # Storage config from env
│   │   ├── factory.go           # Storage factory
│   │   └── cached.go            # Cached storage wrapper
│   ├── auth/                    # Security
│   │   └── signature/           # URL signing
│   │       └── hashers/         # SHA-256, SHA-512
│   └── pkg/                     # Shared utilities
│       ├── logger/              # Leveled logger
│       └── format/              # Formatting helpers
├── docs/                        # Documentation
├── examples/
│   └── systemd/                 # Systemd service examples
├── .env.example
├── docker-compose.local.yml
├── docker-compose.s3.yml
├── Dockerfile
├── go.mod
└── go.sum
```

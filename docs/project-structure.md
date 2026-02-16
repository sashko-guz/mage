# Project Structure

```text
.
├── cmd/
│   └── server/              # Application entry point
├── internal/
│   ├── cache/               # Multi-layer caching (memory + disk)
│   ├── config/              # Configuration management
│   ├── handler/             # HTTP request handlers
│   ├── operations/          # Image operations (filters)
│   ├── parser/              # URL and environment parsing
│   ├── processor/           # Image processing with libvips
│   └── storage/             # Storage layer abstractions and drivers
├── docs/                    # Project documentation
├── storage.*.json           # Storage configuration examples
├── Dockerfile               # Container image definition
└── go.mod                   # Go module dependencies
```

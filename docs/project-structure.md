# Project Structure

```text
.
├── cmd/
│   └── server/              # Application entry point
│       └── main.go
├── internal/
│   ├── cache/               # Memory and disk cache implementations
│   ├── config/              # Environment/server config loading
│   ├── handler/             # HTTP request handling
│   ├── logger/              # Leveled logger wrapper
│   ├── operations/          # Image operations (resize/crop/fit/etc.)
│   ├── parser/              # URL/filter/signature parsing
│   ├── processor/           # Image processing orchestration
│   └── storage/             # Storage abstraction + cache wrapper + drivers
│       └── drivers/
│           ├── local.go
│           └── s3.go
├── docs/                    # Project documentation
├── .env.example             # Example environment variables
├── docker-compose.yml       # Local/dev service orchestration
├── README.md                # Main project documentation
├── storage.*.json           # Storage configuration examples
├── Dockerfile               # Container image definition
├── go.mod                   # Go module definition
└── go.sum                   # Go dependency checksums
```

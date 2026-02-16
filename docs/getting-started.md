# Getting Started

## Prerequisites

- Go 1.26+
- Docker (optional)
- libvips 8.18

## Local Development

1. Clone repository:

```bash
git clone https://github.com/sashko-guz/mage.git
cd mage
```

2. Install dependencies:

```bash
go mod download
```

3. Configure storage:

```bash
# For local storage
cp storage.local.example.json storage.json

# Or for S3 storage
cp storage.s3.example.json storage.json

# Edit storage.json with your configuration
```

4. Run server:

```bash
go run ./cmd/server
```

## Docker

Build and run:

```bash
docker build -t mage .
docker run -p 8080:8080 -e STORAGE_CONFIG_PATH=/app/storage.json -v "$(pwd)/storage.json:/app/storage.json:ro" mage
```

## Docker Compose

Run with compose from project root:

```bash
docker compose up --build
```

Compose uses relative paths from `docker-compose.yml` and mounts:

- `./storage.docker.json` -> `/app/storage.docker.json`
- `./.data` -> `/app/data`
- `./.cache` -> `/app/.cache`

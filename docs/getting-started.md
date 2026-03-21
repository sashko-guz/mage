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

3. Configure environment and storage:

```bash
cp .env.example .env
# Edit .env with your configuration

# For local storage
cp storage.local.example.json storage.json

# Or for S3 storage
cp storage.s3.example.json storage.json
# S3 credentials are read from env vars — set S3_ACCESS_KEY and S3_SECRET_KEY in .env
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

1. Prepare required files:

```bash
cp .env.example .env
# Edit .env — set S3_ACCESS_KEY, S3_SECRET_KEY, and other values as needed

mkdir -p .data .cache
```

2. Build and run:

```bash
docker compose up --build
```

Compose mounts:

- `./storage.docker.json` → `/app/storage.json` (read-only)
- `./.data` → `/app/.data`
- `./.cache` → `/app/.cache`

Environment variables are loaded from `.env` via `env_file`.

### Resource limits

Override memory/CPU limits via `.env`:

```env
DOCKER_MEMORY_LIMIT=4g
DOCKER_CPU_LIMIT=2
```

Defaults: memory `2g`, CPU unlimited (`0`).

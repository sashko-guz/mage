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

3. Configure environment:

```bash
cp .env.example .env
# Edit .env with your configuration

# For local storage, set:
STORAGE_DRIVER=local
STORAGE_ROOT=/path/to/your/images

# Create cache directories if using caching
mkdir -p .cache/sources .cache/thumbs
```

4. Run server:

```bash
go run ./cmd/mage
```

## Docker

Build and run:

```bash
docker build -t mage .
docker run -p 8080:8080 --env-file .env mage
```

## Docker Compose

### Local storage

1. Prepare environment:

```bash
cp .env.example .env
# Edit .env and set STORAGE_DRIVER=local and STORAGE_ROOT

mkdir -p .cache/sources .cache/thumbs
```

2. Build and run:

```bash
docker compose -f docker-compose.local.yml up --build
```

Local compose mounts:

- `${STORAGE_ROOT}` → `${STORAGE_ROOT}` (read-only, same absolute path inside container)
- `./.cache` → `/app/.cache`

### S3 storage

1. Prepare environment:

```bash
cp .env.example .env
# Edit .env and set:
# STORAGE_DRIVER=s3
# S3_BUCKET, S3_REGION, S3_ACCESS_KEY, S3_SECRET_KEY
# Optionally S3_BASE_URL and S3_USE_PATH_STYLE for MinIO

mkdir -p .cache/sources .cache/thumbs
```

2. Build and run:

```bash
docker compose -f docker-compose.s3.yml up --build
```

S3 compose mounts:

- `./.cache` → `/app/.cache`

Environment variables are loaded from `.env` via `env_file`.

## Systemd

For native host service, use the examples in `examples/systemd/` and adjust paths.

Both units expect:

- binary at `/usr/local/bin/mage`
- `.env` file in working directory

### Resource limits

Override memory/CPU limits via `.env`:

```env
DOCKER_MEMORY_LIMIT=4g
DOCKER_CPU_LIMIT=2
```

Defaults: memory `2g`, CPU unlimited (`0`).

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
cp examples/storage/local.example.json storage.json

# Or for S3 storage
cp examples/storage/s3.example.json storage.json
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

### Local storage

1. Prepare required files:

```bash
cp .env.example .env
cp examples/storage/local.example.json storage.json
# Edit .env and set STORAGE_ROOT to the same host directory you use locally.

mkdir -p .cache/sources .cache/thumbs
```

2. Build and run:

```bash
docker compose -f docker-compose.local.yml up --build
```

Local compose mounts:

- `./storage.json` → `/app/storage.json` (read-only)
- `${STORAGE_ROOT}` → `${STORAGE_ROOT}` (read-only, same absolute path inside the container)
- `./.cache` → `/app/.cache`

This lets Docker read the same original-image directory as the local binary without copying files into the repository.

### S3 storage

1. Prepare required files:

```bash
cp .env.example .env
cp examples/storage/s3.example.json storage.json
# Edit .env and set S3_BUCKET, S3_REGION, S3_ACCESS_KEY, S3_SECRET_KEY,
# and optionally S3_BASE_URL / S3_USE_PATH_STYLE.

mkdir -p .cache/sources .cache/thumbs
```

2. Build and run:

```bash
docker compose -f docker-compose.s3.yml up --build
```

S3 compose mounts:

- `./storage.json` → `/app/storage.json` (read-only)
- `./.cache` → `/app/.cache`

Environment variables are loaded from `.env` via `env_file`.

There is no default compose file anymore. Choose the backend explicitly with either `docker-compose.local.yml` or `docker-compose.s3.yml`.

## Systemd

For a native host service, start from the matching example and adjust paths for your machine.

Local service:

- `examples/systemd/mage-local.service.example`
- use `examples/storage/local.example.json` as `storage.json`
- set `STORAGE_ROOT` in `.env`

S3 service:

- `examples/systemd/mage-s3.service.example`
- use `examples/storage/s3.example.json` as `storage.json`
- set `S3_BUCKET`, `S3_REGION`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, and optional S3 endpoint variables in `.env`

Both units expect:

- the binary to be available at `/usr/local/bin/mage`
- the project directory to contain `.env` and `storage.json`

### Resource limits

Override memory/CPU limits via `.env`:

```env
DOCKER_MEMORY_LIMIT=4g
DOCKER_CPU_LIMIT=2
```

Defaults: memory `2g`, CPU unlimited (`0`).

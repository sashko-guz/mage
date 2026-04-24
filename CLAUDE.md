# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Mage is a Go image processing service powered by libvips. It generates thumbnails on-the-fly via URL-based API, supports local/S3 storage backends, and includes multi-layer caching (memory + disk).

## Build and Run

```bash
# Run server (loads .env automatically)
go run ./cmd/mage

# Build binary
go build -o mage ./cmd/mage

# Quick local setup
cp .env.example .env
# Set STORAGE_DRIVER=local and STORAGE_ROOT in .env
mkdir -p .cache/sources .cache/thumbs
```

## Configuration

All configuration via environment variables (see `.env.example`):
- Storage: `STORAGE_DRIVER`, `STORAGE_ROOT`, `S3_*`
- Cache: `SOURCE_*_CACHE_*`, `THUMB_*_CACHE_*`
- Server: `PORT`, `LOG_LEVEL`, `HTTP_*`, `CORS_*`
- Signature: `SIGNATURE_SECRET`, `SIGNATURE_ALGO`
- Limits: `MAX_RESIZE_*`, `MAX_INPUT_IMAGE_SIZE_MB`
- Observability: `METRICS_ENABLED`, `METRICS_PATH`, `HEALTH_READINESS_TIMEOUT_SECONDS`

## Architecture

```
internal/
  app/              # Application lifecycle (bootstrap, shutdown)
  config/           # Unified config loading
  http/             # HTTP server, router, middleware
  routes/           # Route registry
  observability/    # Metrics and health checks
    metrics/        # Prometheus metrics
    health/         # Liveness/readiness probes
  imaging/          # Image domain types
    operations/     # Image operations, Request type
  thumbnail/        # Thumbnail domain
    handler/        # HTTP handler
    parser/         # URL parsing
    processor/      # libvips orchestration
  storage/          # Storage + cache layer
    drivers/        # Local, S3
    cache/          # Memory, disk cache
  auth/             # Security
    signature/      # URL signing
  pkg/              # Shared utilities (logger, format)
```

**Request flow:**
1. `http/router.go` - matches route, applies CORS
2. `thumbnail/handler/` - signature validation, cache lookup
3. `thumbnail/parser/` - parses URL into `operations.Request`
4. `imaging/operations/pipeline.go` - applies operations via libvips
5. `storage/cached.go` - multi-layer cache (memory → disk → storage)

## URL API

```
/thumbs/[{signature}/]{width}x{height}/[f:{filters}/]{path}[/as/{alias.ext}]
/t/[{signature}/]{width}x{height}/[f:{filters}/]{path}[/as/{alias.ext}]
```

`/t/` is short alias for `/thumbs/`. Operations: crop/pcrop → fit → resize → format/quality.

Filter aliases: `format`→`fmt`, `quality`→`q`, `crop`→`c`, `pcrop`→`pc`

## Observability

Endpoints:
- `/health` - Liveness probe (always 200 if running)
- `/ready` - Readiness probe (checks storage connectivity)
- `/metrics` - Prometheus metrics

Run with monitoring stack:
```bash
docker-compose -f docker-compose.local.yml --profile monitoring up -d
# Prometheus: localhost:9090, Grafana: localhost:3000
```

## Dependencies

- `github.com/cshum/vipsgen` - libvips bindings (requires libvips)
- `github.com/dgraph-io/ristretto` - memory cache
- `github.com/prometheus/client_golang` - Prometheus metrics
- `aws-sdk-go-v2` - S3 storage driver
- `golang.org/x/sync/singleflight` - request deduplication

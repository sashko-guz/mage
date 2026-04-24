# Configuration

All configuration is done via environment variables. Copy `.env.example` to `.env` and adjust as needed.

## Quick Reference

| Category | Key Variables | Details |
|----------|--------------|---------|
| Storage | `STORAGE_DRIVER`, `STORAGE_ROOT`, `S3_*` | [Storage](#storage) |
| Caching | `SOURCE_*_CACHE_*`, `THUMB_*_CACHE_*` | [Caching](caching.md) |
| S3 HTTP | `S3_MAX_IDLE_CONNS`, `S3_*_TIMEOUT_*` | [S3 HTTP Client](s3-http-client.md) |
| Signature | `SIGNATURE_SECRET`, `SIGNATURE_ALGO` | [Signature](signature.md) |
| Server | `PORT`, `LOG_LEVEL`, `HTTP_*` | [Server](#server) |
| Observability | `METRICS_*`, `HEALTH_*` | [Monitoring](monitoring.md) |
| Processing | `MAX_RESIZE_*`, `MAX_INPUT_IMAGE_SIZE_MB` | [Processing](#image-processing) |

---

## Storage

| Variable | Description | Default |
|----------|-------------|---------|
| `STORAGE_DRIVER` | Storage driver: `local` or `s3` | `local` |
| `STORAGE_ROOT` | Root directory for local driver | (required for local) |

### S3 Driver

| Variable | Description | Default |
|----------|-------------|---------|
| `S3_BUCKET` | S3 bucket name | (required) |
| `S3_REGION` | AWS region | `us-east-1` |
| `S3_ACCESS_KEY` | Access key (optional for IAM roles) | |
| `S3_SECRET_KEY` | Secret key (optional for IAM roles) | |
| `S3_BASE_URL` | Custom endpoint for S3-compatible storage | |
| `S3_USE_PATH_STYLE` | Path-style addressing (required for MinIO) | `false` |

See [S3 HTTP Client](s3-http-client.md) for connection tuning options.

---

## Server

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `LOG_LEVEL` | Log level: `debug`, `info`, `warn`, `error` | `error` |
| `VIPS_CONCURRENCY` | libvips concurrency level | (auto) |

### HTTP Timeouts

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_READ_TIMEOUT_SECONDS` | Time to read full request | `5` |
| `HTTP_READ_HEADER_TIMEOUT_SECONDS` | Time to read headers | `2` |
| `HTTP_WRITE_TIMEOUT_SECONDS` | Time to write response | `30` |
| `HTTP_IDLE_TIMEOUT_SECONDS` | Keep-alive idle timeout | `120` |
| `HTTP_MAX_HEADER_BYTES` | Max header size in bytes | `1048576` |

### CORS

| Variable | Description | Default |
|----------|-------------|---------|
| `CORS_ALLOW_ORIGIN` | Allowed origin | `*` |
| `CORS_ALLOW_METHODS` | Allowed methods | `GET, HEAD, OPTIONS` |
| `CORS_ALLOW_HEADERS` | Allowed headers | `Origin, Content-Type, Accept, Authorization` |
| `CORS_EXPOSE_HEADERS` | Exposed headers | `Content-Type, Content-Length, Cache-Control, X-Mage-Cache` |
| `CORS_MAX_AGE` | Preflight cache duration (seconds) | `86400` |

---

## Observability

| Variable | Description | Default |
|----------|-------------|---------|
| `METRICS_ENABLED` | Enable Prometheus metrics endpoint | `true` |
| `METRICS_PATH` | Metrics endpoint path | `/metrics` |
| `HEALTH_READINESS_TIMEOUT_SECONDS` | Readiness check timeout | `5` |

See [Monitoring](monitoring.md) for full metrics reference, Grafana setup, and alerting examples.

---

## Image Processing

| Variable | Description | Default |
|----------|-------------|---------|
| `MAX_INPUT_IMAGE_SIZE_MB` | Max source image size in MB | `64` |
| `MAX_RESIZE_WIDTH` | Max resize width in pixels | `5120` |
| `MAX_RESIZE_HEIGHT` | Max resize height in pixels | `5120` |
| `MAX_RESIZE_RESOLUTION` | Max total pixel area | `26214400` |
| `CACHE_CONTROL_RESPONSE_HEADER` | Cache-Control header value | `public, max-age=31536000, immutable` |

---

## Docker

| Variable | Description | Default |
|----------|-------------|---------|
| `DOCKER_MEMORY_LIMIT` | Container memory limit | `2g` |
| `DOCKER_CPU_LIMIT` | Container CPU limit (0 = unlimited) | `0` |

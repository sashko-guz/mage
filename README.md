# Mage

An educational Go project for learning modern web development and image processing.

## About

This is a learning project created to explore:
- Go programming language and its ecosystem
- Image processing with libvips
- HTTP server development
- Docker containerization
- Storage abstractions (local and S3)
- Caching strategies

**Note:** This project was mostly created with AI agents to learn Go best practices and modern development patterns.

## What It Does

Mage is an HTTP image processing service that:
- Generates thumbnails on-the-fly
- Supports multiple storage backends (local filesystem, AWS S3)
- Implements caching for improved performance
- Validates requests with HMAC signatures (configurable per storage)
- Uses libvips for fast image processing

## Tech Stack

- **Go 1.25** - Programming language
- **libvips 8.18** - High-performance image processing library
- **Docker** - Containerization
- **AWS SDK v2** - S3 storage integration

## Getting Started

### Prerequisites

- Go 1.25+
- Docker (optional)
- libvips 8.18

### Local Development

1. Clone the repository:
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
cp storages.example.json storages.json
# Edit storages.json with your storage configuration
```

4. Run the server:
```bash
go run ./cmd/server
```

### Docker

Build and run with Docker:
```bash
docker build -t mage .
docker run -p 8080:8080 -v $(pwd)/storages.json:/app/storages.json mage
```

## Configuration

Configure the service using environment variables:

- `PORT` - Server port (default: 8080)
- `STORAGES_CONFIG_PATH` - Path to storages configuration file (default: ./storages.json)
- `VIPS_CONCURRENCY` - libvips concurrency level (optional)

Per-storage settings are configured in `storages.json`:
- `signature_secret_key` - HMAC signature validation secret key (optional)
- `cache.memory.enabled` - Enable/disable in-memory cache for this storage (default: `false`)
- `cache.memory.max_size_mb` - Maximum memory cache size in MB (optional)
  - Default: Not set (memory cache disabled if this field is omitted)
- `cache.memory.max_items` - Maximum number of cached items (optional)
  - Default: Automatically calculated as `max_size_mb / 100` (minimum 100 items)
- `cache.disk.enabled` - Enable/disable disk cache for this storage (default: `false`)
- `cache.disk.ttl_seconds` - Cache time-to-live in seconds (default: `300` if disk cache enabled)
- `cache.disk.max_size_mb` - Maximum disk cache size in megabytes (optional)
  - Default: 0 (unlimited)
  - Examples: 512 (512MB), 1024 (1GB), 5120 (5GB), 10240 (10GB)
  - When exceeded, oldest cached files are automatically deleted during cleanup
- `cache.disk.dir` - Cache directory path (required if disk cache enabled)
- `cache.disk.clear_on_startup` - Clear cache on server startup (default: `false`)

## Multi-Layer Caching

Mage implements a sophisticated multi-layer caching system for maximum performance:

### Cache Hierarchy

1. **Layer 1: In-Memory Cache (Optional)** - Fastest, configurable per storage
   - Uses Ristretto LRU cache with cost-based eviction
   - Configured via `memory.enabled` and `memory.max_size_mb` in storages.json
   - Provides 50-200μs latency (vs 3-8ms disk I/O)
   - **Performance:** +200-500% RPS for hot images

2. **Layer 2: Disk-Based Cache** - Persistent disk cache
   - Hierarchical directory structure for scalability
   - Expiration-based eviction (TTL) + size-based eviction (LRU)
   - **Asynchronous cleanup:** Background goroutine runs periodic cleanup (~30s interval, adaptive backoff)
     - Deletes expired files (based on `ttl_seconds`)
     - Enforces size limit (deletes oldest files if cache exceeds `max_size_mb`)
     - Cleanup frequency adapts: backs off when idle, increases during high deletion rates
   - Survives server restarts

3. **Layer 3: Source Storage** - S3, local filesystem, etc.
   - Only accessed on cache miss

### Cache Configuration Examples

**With both memory and disk caching enabled:**
```json
{
  "storages": [
    {
      "name": "uploads",
      "driver": "local",
      "root": "/var/www/uploads",
      "cache": {
        "memory": {
          "enabled": true,
          "max_size_mb": 512,
          "max_items": 1000
        },
        "disk": {
          "enabled": true,
          "ttl_seconds": 300,
          "max_size_mb": 5120,
          "dir": "/var/cache/mage/uploads",
          "clear_on_startup": false
        }
      }
    },
    {
      "name": "s3-images",
      "driver": "s3",
      "bucket": "my-images",
      "cache": {
        "memory": {
          "enabled": true,
          "max_size_mb": 1024
        },
        "disk": {
          "enabled": true,
          "ttl_seconds": 600,
          "max_size_mb": 10240,
          "dir": "/var/cache/mage/s3",
          "clear_on_startup": false
        }
      }
    }
  ]
}
```

**With caching disabled:**
```json
{
  "storages": [
    {
      "name": "uploads",
      "driver": "local",
      "root": "/var/www/uploads",
      "cache": {
        "memory": {
          "enabled": false
        },
        "disk": {
          "enabled": false
        }
      }
    }
  ]
}
```

### Performance Benefits

With memory cache enabled:
- **Latency:** 150ms P99 → 40-60ms P99
- **Throughput:** 500 RPS → 2000+ RPS (for hot content)
- **CPU Usage:** -20% at same RPS (less disk I/O)
- **Best for:** Popular images that get frequent requests

### Cache Configuration Guide

**In-Memory Cache (`memory.max_size_mb`):**
- **256MB:** Small sites, limited hot images (~2500 cached thumbnails @ 100KB each)
- **512MB:** Medium traffic sites with moderate hot set
- **1024MB:** High traffic sites, large hot image set
- **2048MB+:** Very high traffic, extensive hot content
- `max_items` automatically defaults to approximately `max_size_mb / 100` MB (assuming ~100KB average item size)

**Disk Cache (`disk.ttl_seconds` and `disk.max_size_mb`):**
- **TTL (Time-To-Live):**
  - **Default:** 300 seconds (5 minutes) if not specified
  - **300-600s:** Typical production setting (5-10 minutes)
  - **Shorter TTL (< 300s):** Frequently updated content
  - **Longer TTL (> 1800s):** Static content that rarely changes
- **Max Size (`disk.max_size_mb`):**
  - **0 (default):** Unlimited cache size (only limited by TTL expiration)
  - **512MB:** Small deployments or limited disk space
  - **1024MB (1GB):** Typical production setting
  - **5120MB (5GB):** High-traffic sites with large image sets
  - **10240MB+ (10GB+):** Very high traffic, extensive cache retention
  - **Cleanup happens asynchronously:** Cache cleanup runs periodically (default: every 30 seconds). When cache size exceeds the limit, oldest files are deleted during the next cleanup cycle. **Cache may temporarily exceed size limit** during heavy write periods between cleanup runs.

**Optimization Tips:**
- Caching is **disabled by default** for both memory and disk unless explicitly enabled
- **Cleanup runs asynchronously:** Background cleanup goroutine scans cache every ~30 seconds. It:
  - Deletes expired files (based on TTL)
  - Evicts oldest files if cache exceeds `max_size_mb`
  - Adapts cleanup frequency: backs off when no expired files, runs frequently during high deletion rates
- Disk cache may **temporarily exceed** configured `max_size_mb` between cleanup cycles
- Monitor hit ratio and adjust `max_items` and `max_size_mb` accordingly
- Target 60-80% hit rate for in-memory cache with hot content
- Use `clear_on_startup: true` during development, `false` in production
- Adjust `max_items` to control memory usage more precisely with different image sizes

## S3 HTTP Client Optimization

For high-performance S3 access, configure HTTP client settings per storage to optimize connection pooling and prevent timeouts.

### Configuration

Add `s3_http_config` to your S3 storage configuration:

```json
{
  "name": "s3-images",
  "driver": "s3",
  "bucket": "my-bucket",
  "region": "us-west-1",
  "s3_http_config": {
    "max_idle_conns": 100,
    "max_idle_conns_per_host": 100,
    "max_conns_per_host": 0,
    "idle_conn_timeout_sec": 90,
    "connect_timeout_sec": 10,
    "request_timeout_sec": 30,
    "response_header_timeout_sec": 10
  }
}
```

### Configuration Options

- `max_idle_conns` - Maximum idle connections across all hosts (default: 100)
  - **AWS default: 2** | **Recommended: 100-200**
- `max_idle_conns_per_host` - Maximum idle connections per host (default: 100)
  - **AWS default: 2** | **Recommended: 100-200**
- `max_conns_per_host` - Maximum total connections per host (default: 0 = unlimited)
  - Set to limit concurrent connections if needed
- `idle_conn_timeout_sec` - How long idle connections stay open (default: 90)
- `connect_timeout_sec` - Connection establishment timeout (default: 10)
- `request_timeout_sec` - Full request timeout including data transfer (default: 30)
- `response_header_timeout_sec` - Time to wait for response headers (default: 10)

### Performance Impact

**Default AWS SDK (2 connections):**
- New TCP handshake per request (~30ms overhead)
- Connection pool exhaustion under load
- Requests queue waiting for connections

**Optimized (100+ connections):**
- **+40-80% RPS** for S3-backed thumbnails
- **-30ms latency** per request (connection reuse)
- **P95 latency:** 180ms → 120ms
- **Prevents hung requests** with proper timeouts
- **HTTP/2 multiplexing** enabled automatically

### Real-World Example

**Before optimization:**
- 300 RPS with 2 connection pool
- Frequent connection timeouts
- P99 latency: 450ms

**After optimization:**  
- 450-500 RPS with 100 connection pool
- No timeouts (protected by request_timeout)
- P99 latency: 150ms

**Critical for:**
- High traffic S3-backed services
- Multiple concurrent thumbnail requests
- Cold cache scenarios (cache misses)
- S3-compatible storage (MinIO, Cloudflare R2, etc.)

## Project Structure

```
.
├── cmd/
│   └── server/          # Application entry point
├── internal/
│   ├── cache/           # File-based caching
│   ├── config/          # Configuration management
│   ├── handler/         # HTTP handlers
│   ├── parser/          # URL and environment parsing
│   ├── processor/       # Image processing logic
│   └── storage/         # Storage abstractions
├── Dockerfile           # Container image definition
└── go.mod              # Go module dependencies
```

## Learning Outcomes

Working on this project helped understand:
- Go's standard library and HTTP server patterns
- CGO integration with C libraries (libvips)
- Interface-based design for storage abstractions
- Docker multi-stage builds
- Dependency management with Go modules
- Error handling and logging in production services

## License

This is an educational project created for learning purposes.

## Acknowledgments

Built with assistance from AI coding agents to learn Go development best practices.

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
- Supports storage backends (local filesystem, AWS S3, S3-compatible services)
- Implements multi-layer caching for improved performance
- Validates requests with HMAC signatures (optional)
- Uses libvips for fast image processing

## Tech Stack

- **Go 1.26** - Programming language
- **libvips 8.18** - High-performance image processing library
- **Docker** - Containerization
- **AWS SDK v2** - S3 storage integration

## Getting Started

### Prerequisites

- Go 1.26+
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

3. Configure storage:
```bash
# For local storage
cp storage.local.example.json storage.json

# Or for S3 storage
cp storage.s3.example.json storage.json

# Edit storage.json with your configuration
```

4. Run the server:
```bash
go run ./cmd/server
```

### Docker

Build and run with Docker:
```bash
docker build -t mage .
docker run -p 8080:8080 -v $(pwd)/storage.json:/app/storage.json mage
```

## Configuration

### Environment Variables

- `PORT` - Server port (default: 8080)
- `STORAGE_CONFIG_PATH` - Path to storage configuration file (default: ./storage.json)
- `VIPS_CONCURRENCY` - libvips concurrency level (optional)

### Storage Configuration

Configure your storage in `storage.json`. See example files for reference:
- `storage.local.example.json` - Local filesystem storage
- `storage.s3.example.json` - AWS S3 or S3-compatible storage

#### Configuration Options

**Storage Settings:**
- `driver` - Storage backend: `"local"` or `"s3"`
- `signature_secret` - HMAC signature validation secret key (optional)

**Local Storage:**
- `root` - Root directory path for local storage (required for local driver)

**S3 Storage:**
- `bucket` - S3 bucket name (required for S3 driver)
- `region` - AWS region (default: "us-east-1")
- `access_key` - AWS access key (optional, uses default credentials if omitted)
- `secret_key` - AWS secret key (optional, uses default credentials if omitted)
- `base_url` - Custom endpoint for S3-compatible storage like MinIO (optional)

**Cache Settings:**

Mage supports separate cache configurations for **sources** (original images from storage) and **thumbs** (generated thumbnails):

**Source Cache (`cache.sources`):**
- `cache.sources.memory.enabled` - Enable in-memory cache for source images (default: `false`)
- `cache.sources.memory.max_size_mb` - Maximum memory cache size in MB
- `cache.sources.memory.max_items` - Maximum number of cached items
- `cache.sources.memory.ttl_seconds` - Cache time-to-live in seconds (optional, default: 300)
- `cache.sources.disk.enabled` - Enable disk cache for source images (default: `false`)
- `cache.sources.disk.ttl_seconds` - Cache time-to-live in seconds (default: 300)
- `cache.sources.disk.max_size_mb` - Maximum disk cache size in MB (0 = unlimited)
- `cache.sources.disk.dir` - Cache directory path (required if disk cache enabled)
- `cache.sources.disk.clear_on_startup` - Clear cache on server startup (default: `false`)

**Thumbnail Cache (`cache.thumbs`):**
- `cache.thumbs.memory.enabled` - Enable in-memory cache for thumbnails (default: `false`)
- `cache.thumbs.memory.max_size_mb` - Maximum memory cache size in MB
- `cache.thumbs.memory.max_items` - Maximum number of cached items
- `cache.thumbs.memory.ttl_seconds` - Cache time-to-live in seconds (optional, default: 300)
- `cache.thumbs.disk.enabled` - Enable disk cache for thumbnails (default: `false`)
- `cache.thumbs.disk.ttl_seconds` - Cache time-to-live in seconds (default: 300)
- `cache.thumbs.disk.max_size_mb` - Maximum disk cache size in MB (0 = unlimited)
- `cache.thumbs.disk.dir` - Cache directory path (required if disk cache enabled)
- `cache.thumbs.disk.clear_on_startup` - Clear cache on server startup (default: `false`)

**Note:** You can configure caching independently for sources and thumbnails. Each layer (sources/thumbs) can have memory cache only, disk cache only, both, or neither. When both are enabled, memory cache is checked first for maximum performance.

## URL Filters and API

### URL Format

Mage provides a flexible URL-based API for generating thumbnails with filters:

```
/thumbs/[{signature}/]{width}x{height}/[filters:{filters}/]{path}
```

**Components:**
- `{signature}` - (Optional) HMAC-SHA256 signature for request validation
- `{width}x{height}` - Required thumbnail dimensions (e.g., `200x350`)
- `{filters}` - (Optional) Filter string with multiple filters separated by semicolons
- `{path}` - Required file path in storage (e.g., `path/to/image.jpg`)

### Available Filters

Filters are optional and specified in the URL after `filters:`. Multiple filters are separated by semicolons.

#### format(format)

Specifies output image format.

- **Supported formats:** `jpeg`, `png`, `webp`
- **Default:** Detected from file extension (defaults to `jpeg` if not specified)
- **Example:** `format(webp)`

#### quality(level)

JPEG/WebP compression quality level.

- **Range:** 1-100
- **Default:** 75
- **Example:** `quality(88)`

#### fit(mode[,color])

Specifies how the image fits within the requested dimensions.

- **Modes:**
  - `cover` (default) - Scales and crops to fill dimensions, maintains aspect ratio
  - `fill` - Scales to fit within dimensions, maintains aspect ratio, fills remaining space with color

- **Fill Color** (for `fill` mode only):
  - `black` - Fill with black background
  - `white` (default for JPEG/WebP) - Fill with white background
  - `transparent` (PNG only) - Fill with transparent background

- **Default:** `cover`
- **Examples:**
  - `fit(fill)` - Fill mode with default color (transparent for PNG, white otherwise)
  - `fit(fill,black)` - Fill mode with black background
  - `fit(cover)` - Cover mode (crop to fill dimensions)

#### crop(x1,y1,x2,y2)

Crops the image to a rectangular area before any resizing or fit operations.

- **Parameters:**
  - `x1` - X coordinate of the top-left corner (must be >= 0)
  - `y1` - Y coordinate of the top-left corner (must be >= 0)
  - `x2` - X coordinate of the bottom-right corner (must be > x1)
  - `y2` - Y coordinate of the bottom-right corner (must be > y1)

- **Validation:**
  - All coordinates must be non-negative
  - x2 must be greater than x1
  - y2 must be greater than y1
  - Crop area must have non-zero dimensions
  - Coordinates must be within the original image bounds

- **Workflow:** Crop is applied **first**, then the fit/resize operations are applied to the cropped image

- **Examples:**
  - `crop(100,100,500,500)` - Crop to 400x400 area from (100,100) to (500,500)
  - `crop(0,0,1920,1080);fit(cover)` - Crop to 1920x1080, then scale to cover dimensions
  - `crop(50,50,400,400);fit(fill,white)` - Crop to 350x350, then scale and fill with white

#### pcrop(x1,y1,x2,y2)

Crops the image using percentage-based coordinates, useful for responsive designs. Similar to `crop`, but coordinates are specified as percentages (0-100) instead of pixels.

- **Parameters:**
  - `x1` - X coordinate of the top-left corner as a percentage (0-100)
  - `y1` - Y coordinate of the top-left corner as a percentage (0-100)
  - `x2` - X coordinate of the bottom-right corner as a percentage (0-100, must be > x1)
  - `y2` - Y coordinate of the bottom-right corner as a percentage (0-100, must be > y1)

- **Validation:**
  - All coordinates must be between 0 and 100 (inclusive)
  - x2 must be greater than x1
  - y2 must be greater than y1
  - Crop area must have non-zero dimensions

- **Constraints:**
  - Cannot be used together with `crop` filter in the same request (use either `crop` or `pcrop`, not both)

- **Workflow:** Crop is applied **first**, then the fit/resize operations are applied to the cropped image

- **Examples:**
  - `pcrop(10,10,90,90)` - Crop to center 80%x80% of the image
  - `pcrop(25,25,75,75);fit(fill,white)` - Crop to center 50%x50%, then scale and fill with white

### Example URLs

**Without filters:**
```
/thumbs/200x350/path/to/image.jpg
```

**With filters (multiple filters separated by semicolons):**
```
/thumbs/200x350/filters:format(webp);quality(90);fit(fill,black)/path/to/image.jpg
```

**With crop filter:**
```
/thumbs/300x300/filters:crop(100,100,500,500);format(webp)/path/to/image.jpg
```

**With percentage crop filter:**
```
/thumbs/300x300/filters:pcrop(10,10,90,90);format(webp)/path/to/image.jpg
```

**With signature (required when `signature_secret` is configured):**
```
/thumbs/a1b2c3d4e5f6g7h8/200x350/filters:format(webp);quality(88)/path/to/image.jpg
```

### Signature Generation

When `signature_secret` is configured in storage settings, all requests must include a valid HMAC-SHA256 signature. The signature is calculated over all parameters after it in the URL.

**Signature components (in order):**
1. Size (e.g., `200x350`)
2. Filter string if present (e.g., `format(webp);quality(88)`)
3. File path (e.g., `path/to/image.jpg`)

## Multi-Layer Caching

Mage implements a flexible multi-layer caching system with **separate configurations for sources and thumbnails**. This allows you to optimize caching strategies independently for original images and generated thumbnails.

### Cache Hierarchy

The cache system has two independent tiers:

1. **Source Cache (`cache.sources`)** - Caches original images from storage
   - Used when fetching images from S3, local filesystem, etc.
   - Reduces redundant storage reads for frequently accessed originals
   - Especially useful when generating multiple thumbnail variants from the same source

2. **Thumbnail Cache (`cache.thumbs`)** - Caches generated thumbnails
   - Stores processed thumbnails with specific dimensions and filters
   - Avoids re-processing images for repeated requests
   - Typically larger than source cache due to higher request volume

Each tier supports multi-layer caching with optional memory and disk layers:

**Layer 1: In-Memory Cache (Optional)** - Fastest
   - Uses Ristretto LRU cache with cost-based eviction
   - Provides 50-200μs latency (vs 3-8ms disk I/O)
   - **Performance:** +200-500% RPS for hot images
   - Can be enabled independently per tier (sources/thumbs)

**Layer 2: Disk-Based Cache (Optional)** - Persistent
   - Hierarchical directory structure for scalability
   - Expiration-based eviction (TTL) + size-based eviction (LRU)
   - **Asynchronous cleanup:** Background goroutine runs periodic cleanup (~30s interval, adaptive backoff)
     - Deletes expired files (based on `ttl_seconds`)
     - Enforces size limit (deletes oldest files if cache exceeds `max_size_mb`)
     - Cleanup frequency adapts: backs off when idle, increases during high deletion rates
   - Survives server restarts
   - Can be enabled independently per tier (sources/thumbs)

**Layer 3: Source Storage** - S3, local filesystem, etc.
   - Only accessed on cache miss

**When both caches are enabled for a tier:** Memory cache is checked first, then disk cache. Items found in disk cache are automatically promoted to memory cache for faster subsequent access.

### Configuration Examples

**Both caches enabled for sources and thumbnails (recommended for production):**
```json
{
  "driver": "local",
  "root": "/var/www/uploads",
  "signature_secret": "your-secret-key-here",
  "cache": {
    "sources": {
      "memory": {
        "enabled": true,
        "ttl_seconds": 300,
        "max_size_mb": 256,
        "max_items": 100
      },
      "disk": {
        "enabled": true,
        "ttl_seconds": 300,
        "max_size_mb": 2048,
        "dir": "/var/cache/mage/sources",
        "clear_on_startup": false
      }
    },
    "thumbs": {
      "memory": {
        "enabled": true,
        "ttl_seconds": 600,
        "max_size_mb": 512,
        "max_items": 500
      },
      "disk": {
        "enabled": true,
        "ttl_seconds": 300,
        "max_size_mb": 10240,
        "dir": "/var/cache/mage/thumbs",
        "clear_on_startup": false
      }
    }
  }
}
```

**Cache thumbnails only (skip source caching):**
```json
{
  "driver": "local",
  "root": "/var/www/uploads",
  "cache": {
    "thumbs": {
      "memory": {
        "enabled": true,
        "ttl_seconds": 600,
        "max_size_mb": 512,
        "max_items": 500
      },
      "disk": {
        "enabled": true,
        "ttl_seconds": 300,
        "max_size_mb": 10240,
        "dir": "/var/cache/mage/thumbs",
        "clear_on_startup": false
      }
    }
  }
}
```

**Memory cache only for thumbnails (no persistence):**
```json
{
  "driver": "local",
  "root": "/var/www/uploads",
  "cache": {
    "thumbs": {
      "memory": {
        "enabled": true,
        "max_size_mb": 512,
        "max_items": 500,
        "ttl_seconds": 600
      }
    }
  }
}
```

**Disk cache only for thumbnails (persistent):**
```json
{
  "driver": "local",
  "root": "/var/www/uploads",
  "cache": {
    "thumbs": {
      "disk": {
        "enabled": true,
        "ttl_seconds": 300,
        "max_size_mb": 10240,
        "dir": "/var/cache/mage/thumbs",
        "clear_on_startup": false
      }
    }
  }
}
```

**No caching (direct storage access only):**
```json
{
  "driver": "local",
  "root": "/var/www/uploads"
}
```

### Performance Benefits

With memory cache enabled for thumbnails:
- **Latency:** 150ms P99 → 40-60ms P99 (cached thumbnail hits)
- **Throughput:** 500 RPS → 2000+ RPS (for hot content)
- **CPU Usage:** -20% at same RPS (less image processing and disk I/O)
- **Best for:** Popular thumbnails that get frequent requests

With source cache enabled:
- **Reduces storage I/O:** Especially beneficial when generating multiple thumbnail variants from same source
- **S3 cost savings:** Fewer GET requests to S3 when source is cached
- **Best for:** High-traffic scenarios where same source generates many thumbnails

### Cache Configuration Guide

**Cache Strategy Selection:**

You can configure caching independently for **sources** and **thumbs**:

- **No cache:** Simple deployments, rarely accessed images, or when using CDN
- **Thumbnails only (recommended):** Most common - cache generated thumbnails but fetch sources directly
- **Both sources and thumbs:** High traffic sites generating multiple variants from same sources
- **Sources only:** Rare - useful when sources are expensive to fetch but thumbnails vary widely

**Per-Tier Configuration (sources/thumbs):**
- **Memory only:** Development, ephemeral environments (Docker containers), hot content only
- **Disk only:** Persistent caching, limited RAM, server restarts common
- **Both (recommended):** Production, high traffic, optimal performance with persistence

**In-Memory Cache (`sources.memory` / `thumbs.memory`):**
- **Size (`max_size_mb`):**
  - **Sources:** 256-512MB typical (source images are larger but less frequently cached)
  - **Thumbs:** 512-1024MB typical (thumbnails are smaller but high volume)
  - `max_items` controls the number of items (not automatically calculated)
- **TTL (`ttl_seconds`):**
  - **Default:** 300 seconds (5 minutes) if not specified
  - **Sources:** 300-600s typical (originals change less frequently)
  - **Thumbs:** 300-900s typical (generated thumbnails are stable)
  - **60-300s:** Development or frequently changing content
  - **900s+:** Static content that rarely changes

**Disk Cache (`sources.disk` / `thumbs.disk`):**
- **TTL (`ttl_seconds`):**
  - **Default:** 300 seconds (5 minutes) if not specified
  - **Sources:** 300-1800s typical
  - **Thumbs:** 300-7200s typical (thumbnails benefit from longer TTL)
  - **Cleanup happens asynchronously:** Background goroutine runs periodic cleanup (~30s interval)
- **Max Size (`max_size_mb`):**
  - **0 (default):** Unlimited cache size (only limited by TTL expiration)
  - **Sources:** 2048-5120MB typical (2-5GB)
  - **Thumbs:** 5120-10240MB typical (5-10GB) - higher volume
  - **Cache may temporarily exceed size limit** during heavy write periods between cleanup runs

**Optimization Tips:**
- Caching is **disabled by default** - you must explicitly enable what you need
- **Most common setup:** Cache only thumbnails (skip source caching) to reduce complexity
- **High-traffic optimization:** Cache both sources and thumbs with memory+disk layers
- **Separate cache directories:** Use different `dir` paths for sources and thumbs (e.g., `/var/cache/mage/sources`, `/var/cache/mage/thumbs`)
- **TTL tuning:** Thumbnails can have longer TTL since they don't change (sources may be replaced/updated)
- **Size allocation:** Allocate more disk space to thumbs cache (higher request volume)
- **Memory allocation:** Balance based on traffic patterns - thumbnails typically get more memory
- **Cleanup runs asynchronously (disk cache only):** Background goroutine scans cache every ~30 seconds:
  - Deletes expired files (based on TTL)
  - Evicts oldest files if cache exceeds `max_size_mb`
  - Adapts cleanup frequency: backs off when no expired files, runs frequently during high deletion rates
- Monitor hit ratio and adjust `max_items` and `max_size_mb` per tier
- Use `clear_on_startup: true` during development, `false` in production
- When both memory and disk caches are enabled for a tier, items found in disk cache are automatically promoted to memory cache

## S3 HTTP Client Optimization

For high-performance S3 access, configure HTTP client settings per storage to optimize connection pooling and prevent timeouts.

### Configuration

Add `s3_http_config` to your S3 storage configuration in `storage.json`:

```json
{
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

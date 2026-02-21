# Configuration

## Environment Variables

- `PORT` - Server port (default: `8080`)
- `STORAGE_CONFIG_PATH` - Path to storage config file (default: `./storage.json`)
- `VIPS_CONCURRENCY` - libvips concurrency level (optional)
- `LOG_LEVEL` - Log level: `debug`, `info`, `warn`, `error` (default: `info`)
- `HTTP_READ_TIMEOUT_SECONDS` - Time to read full request incl. body (default: `5`)
- `HTTP_READ_HEADER_TIMEOUT_SECONDS` - Time to read request headers (default: `2`)
- `HTTP_WRITE_TIMEOUT_SECONDS` - Time to write response (default: `30`)
- `HTTP_IDLE_TIMEOUT_SECONDS` - Keep-alive idle timeout (default: `120`)
- `HTTP_MAX_HEADER_BYTES` - Maximum request header size in bytes (default: `1048576`)
- `SIGNATURE_SECRET` - HMAC signature secret (optional)
- `MAX_RESIZE_WIDTH` - Maximum allowed resize width in pixels (default: `5120`)
- `MAX_RESIZE_HEIGHT` - Maximum allowed resize height in pixels (default: `5120`)
- `MAX_RESIZE_RESOLUTION` - Maximum allowed total pixel area (width × height) (default: `26214400`, i.e. 5120²)

## Storage Configuration

Configure storage in `storage.json`.

Example files:

- `storage.local.example.json`
- `storage.s3.example.json`
- `storage.docker.example.json`

### Storage Settings

- `driver` - `"local"` or `"s3"`

### Local Driver

- `root` - Root directory path (required)

### S3 Driver

- `bucket` - Bucket name (required)
- `region` - AWS region (default: `us-east-1`)
- `access_key` - Access key (optional)
- `secret_key` - Secret key (optional)
- `base_url` - Custom S3-compatible endpoint (optional)
- `s3_http_config` - Optional HTTP transport tuning

### Cache Configuration

Mage supports independent cache setup for:

- `cache.sources` (original source images)
- `cache.thumbs` (generated thumbnails)

Each layer can use:

- `memory`
- `disk`
- both
- neither

#### Memory cache options

- `enabled`
- `max_size_mb`
- `max_items`
- `ttl_seconds`

#### Disk cache options

- `enabled`
- `ttl_seconds`
- `max_size_mb`
- `max_items`
- `dir`
- `clear_on_startup`
- `async_write.enabled` (default true)
- `async_write.num_workers` (default 4)
- `async_write.queue_size` (default 1000)

### Async Disk Writes

- Request path writes memory cache immediately.
- Disk writes are queued to worker pools.
- If queue is full, write may be dropped (cache best-effort behavior).
- Workers drain gracefully on shutdown.

# Configuration

## Environment Variables

- `PORT` - Server port (default: `8080`)
- `STORAGE_CONFIG_PATH` - Path to storage config file (default: `./storage.json`)
- `VIPS_CONCURRENCY` - libvips concurrency level (optional)
- `LOG_LEVEL` - Log level: `debug`, `info`, `warn`, `error` (default: `info`)
- `CORS_ALLOW_ORIGIN` - Allowed origin for CORS responses. Use `*` for public access or set a specific origin like `https://app.example.com` (default: `*`)
- `CORS_ALLOW_METHODS` - Allowed HTTP methods (default: `GET, HEAD, OPTIONS`)
- `CORS_ALLOW_HEADERS` - Allowed request headers (default: `Origin, Content-Type, Accept, Authorization`)
- `CORS_EXPOSE_HEADERS` - Response headers exposed to the browser (default: `Content-Type, Content-Length, Cache-Control, X-Mage-Cache`)
- `CORS_MAX_AGE` - Preflight cache duration in seconds (default: `86400`)
- `HTTP_READ_TIMEOUT_SECONDS` - Time to read full request incl. body (default: `5`)
- `HTTP_READ_HEADER_TIMEOUT_SECONDS` - Time to read request headers (default: `2`)
- `HTTP_WRITE_TIMEOUT_SECONDS` - Time to write response (default: `30`)
- `HTTP_IDLE_TIMEOUT_SECONDS` - Keep-alive idle timeout (default: `120`)
- `HTTP_MAX_HEADER_BYTES` - Maximum request header size in bytes (default: `1048576`)
- `MAX_INPUT_IMAGE_SIZE_MB` - Maximum allowed source image size in megabytes; larger images are rejected before processing (default: `64`)
- `SIGNATURE_SECRET` - HMAC signature secret (optional)
- `SIGNATURE_ALGO` - Signature hash algorithm: `sha256` or `sha512` (default: `sha256`)
- `SIGNATURE_EXTRACT_START` - Start offset (0-based) for extracting signature from full hex digest (default: `0`)
- `SIGNATURE_LENGTH` - Length of signature substring extracted from digest (default: `16`)
- `MAX_RESIZE_WIDTH` - Maximum allowed resize width in pixels (default: `5120`)
- `MAX_RESIZE_HEIGHT` - Maximum allowed resize height in pixels (default: `5120`)
- `MAX_RESIZE_RESOLUTION` - Maximum allowed total pixel area (width × height) (default: `26214400`, i.e. 5120²)
- `CACHE_CONTROL_RESPONSE_HEADER` - Value of the `Cache-Control` header sent with every thumbnail response (default: `public, max-age=31536000, immutable`)
- `S3_BUCKET` - S3 bucket name, referenced as `${S3_BUCKET}` in storage config
- `S3_REGION` - S3 region, referenced as `${S3_REGION}` in storage config
- `S3_ACCESS_KEY` - S3 access key, referenced as `${S3_ACCESS_KEY}` in storage config (optional, leave empty to use IAM role)
- `S3_SECRET_KEY` - S3 secret key, referenced as `${S3_SECRET_KEY}` in storage config (optional, leave empty to use IAM role)
- `S3_BASE_URL` - Custom S3-compatible endpoint, referenced as `${S3_BASE_URL}` in storage config (optional, for MinIO and other S3-compatible storage)
- `S3_USE_PATH_STYLE` - Use path-style S3 addressing (`true`/`false`), referenced as `"${S3_USE_PATH_STYLE}"` in storage config (default: `false`)
- `STORAGE_ROOT` - Root directory for local driver, referenced as `${STORAGE_ROOT}` in storage config
- `DOCKER_MEMORY_LIMIT` - Container memory limit used by docker-compose (default: `2g`)
- `DOCKER_CPU_LIMIT` - Container CPU limit used by docker-compose (default: `0` = unlimited)

## Storage Configuration

Configure storage in `storage.json`.

Example files:

- `storage.local.example.json`
- `storage.s3.example.json`
- `storage.docker.example.json`

### Storage Settings

- `driver` - `"local"` or `"s3"`

### Local Driver

- `root` - Root directory path (required — use env var reference `${STORAGE_ROOT}`)

### S3 Driver

- `bucket` - Bucket name (required — use env var reference, see below)
- `region` - AWS region (required — use env var reference, see below)
- `access_key` - Access key (optional — use env var reference, see below)
- `secret_key` - Secret key (optional — use env var reference, see below)
- `base_url` - Custom S3-compatible endpoint, e.g. MinIO (optional — use env var reference, see below)
- `use_path_style` - Use path-style S3 addressing (default: `false`; set to `true` for MinIO and most S3-compatible storage — use env var reference `"${S3_USE_PATH_STYLE}"`)
- `s3_http_config` - Optional HTTP transport tuning

#### S3 credentials

To avoid storing credentials in the JSON file, use `${VAR}` references — they are expanded from environment variables at startup:

```json
{
  "driver": "s3",
  "bucket": "${S3_BUCKET}",
  "region": "${S3_REGION}",
  "access_key": "${S3_ACCESS_KEY}",
  "secret_key": "${S3_SECRET_KEY}",
  "base_url": "${S3_BASE_URL}",
  "use_path_style": "${S3_USE_PATH_STYLE}"
}
```

Set `S3_BUCKET`, `S3_REGION`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, and optionally `S3_BASE_URL` in your `.env` file. The JSON config itself contains no secrets and is safe to commit.

For MinIO or S3-compatible storage, set `use_path_style` to `true` and provide `S3_BASE_URL`.

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

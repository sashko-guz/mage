# Caching

Mage supports separate multi-layer caches for source images and generated thumbnails.

## Cache Hierarchy

1. Memory cache (optional) - fastest, volatile
2. Disk cache (optional) - persistent, slower
3. Backing storage (local/S3) - origin

When both memory and disk are enabled, memory is checked first, then disk, then storage.

## Memory Cache

- Powered by Ristretto
- Cost-based bounded cache
- Very low-latency hot reads

## Disk Cache

- Persistent file-based cache with LRU index
- TTL support
- Size-bound enforcement and item-count limit
- Async write path via worker pools
- Background cleanup with adaptive cadence

## Environment Variables

### Source Image Cache

| Variable | Description | Default |
|----------|-------------|---------|
| `SOURCE_MEMORY_CACHE_ENABLED` | Enable memory cache | `false` |
| `SOURCE_MEMORY_CACHE_MAX_SIZE_MB` | Max size in MB | `256` |
| `SOURCE_MEMORY_CACHE_MAX_ITEMS` | Max items | `1000` |
| `SOURCE_MEMORY_CACHE_TTL_SEC` | TTL in seconds | `300` |
| `SOURCE_DISK_CACHE_ENABLED` | Enable disk cache | `false` |
| `SOURCE_DISK_CACHE_DIR` | Cache directory | (required if enabled) |
| `SOURCE_DISK_CACHE_MAX_SIZE_MB` | Max size in MB | `2048` |
| `SOURCE_DISK_CACHE_MAX_ITEMS` | Max items | `131072` |
| `SOURCE_DISK_CACHE_TTL_SEC` | TTL in seconds | `600` |
| `SOURCE_DISK_CACHE_CLEAR_ON_STARTUP` | Clear on startup | `false` |
| `SOURCE_DISK_CACHE_ASYNC_ENABLED` | Enable async writes | `true` |
| `SOURCE_DISK_CACHE_ASYNC_WORKERS` | Async worker count | `4` |
| `SOURCE_DISK_CACHE_ASYNC_QUEUE_SIZE` | Async queue size | `1000` |

### Thumbnail Cache

| Variable | Description | Default |
|----------|-------------|---------|
| `THUMB_MEMORY_CACHE_ENABLED` | Enable memory cache | `false` |
| `THUMB_MEMORY_CACHE_MAX_SIZE_MB` | Max size in MB | `512` |
| `THUMB_MEMORY_CACHE_MAX_ITEMS` | Max items | `1000` |
| `THUMB_MEMORY_CACHE_TTL_SEC` | TTL in seconds | `120` |
| `THUMB_DISK_CACHE_ENABLED` | Enable disk cache | `false` |
| `THUMB_DISK_CACHE_DIR` | Cache directory | (required if enabled) |
| `THUMB_DISK_CACHE_MAX_SIZE_MB` | Max size in MB | `10240` |
| `THUMB_DISK_CACHE_MAX_ITEMS` | Max items | `655360` |
| `THUMB_DISK_CACHE_TTL_SEC` | TTL in seconds | `600` |
| `THUMB_DISK_CACHE_CLEAR_ON_STARTUP` | Clear on startup | `false` |
| `THUMB_DISK_CACHE_ASYNC_ENABLED` | Enable async writes | `true` |
| `THUMB_DISK_CACHE_ASYNC_WORKERS` | Async worker count | `4` |
| `THUMB_DISK_CACHE_ASYNC_QUEUE_SIZE` | Async queue size | `1000` |

## Async Write Behavior

- Memory cache writes are synchronous (fast)
- Disk writes are queued to worker pools
- Queue overflow drops writes (best-effort cache semantics)
- Workers drain gracefully on shutdown

## Typical Strategies

- **Thumbnails-only cache** - most common, caches generated output
- **Sources + thumbnails** - for heavy multi-variant workloads
- **Memory-only** - ephemeral/dev environments
- **Disk-only** - persistence with low memory budget

## Tuning Tips

- Increase `MAX_SIZE_MB` for high miss rates and available resources
- Increase `MAX_ITEMS` if many small objects are expected
- Use separate directories for sources and thumbnails
- Keep longer TTL for thumbnails when source updates are frequent
- Tune `ASYNC_WORKERS` and `ASYNC_QUEUE_SIZE` for burst traffic

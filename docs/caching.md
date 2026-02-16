# Caching

Mage supports separate multi-layer caches for:

- source images (`cache.sources`)
- generated thumbnails (`cache.thumbs`)

## Cache Hierarchy

1. Memory cache (optional)
2. Disk cache (optional)
3. Backing storage (local/S3)

When both memory and disk are enabled, memory is checked first, then disk, then storage.

## Memory Cache

- Powered by Ristretto
- Cost-based bounded cache
- Very low-latency hot reads

## Disk Cache

- Persistent file-based cache with LRU index
- TTL support
- Size-bound enforcement and item-count bound (`max_items`)
- Async write path via worker pools
- Background cleanup with adaptive cadence and activity wake-up

## Async Write Behavior

- Source and thumbnail disk writes are non-blocking by default
- Workers consume queued writes
- Queue overflow can drop writes (best-effort cache semantics)

## Typical Strategies

- Thumbnails-only cache (most common)
- Sources + thumbnails for heavy multi-variant workloads
- Memory-only for ephemeral/dev
- Disk-only for persistence with low memory budget

## Tuning Tips

- Increase `max_size_mb` for high miss rates and available disk
- Increase `max_items` if many small objects are expected
- Use separate `dir` for sources and thumbnails
- Keep longer TTL for thumbnails than sources when source updates are frequent
- Tune `async_write.num_workers` and `queue_size` for burst traffic

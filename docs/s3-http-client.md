# S3 HTTP Client Optimization

For high-throughput S3 access, tune the HTTP client via environment variables.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `S3_MAX_IDLE_CONNS` | Max idle connections across all hosts | `100` |
| `S3_MAX_IDLE_CONNS_PER_HOST` | Max idle connections per host | `100` |
| `S3_MAX_CONNS_PER_HOST` | Max total connections per host (0 = unlimited) | `0` |
| `S3_IDLE_CONN_TIMEOUT_SEC` | Idle connection timeout in seconds | `90` |
| `S3_CONNECT_TIMEOUT_SEC` | TCP connect timeout in seconds | `10` |
| `S3_REQUEST_TIMEOUT_SEC` | Full request timeout in seconds | `30` |
| `S3_RESPONSE_HEADER_TIMEOUT_SEC` | Response header wait timeout in seconds | `10` |

## Example Configuration

```env
# High-throughput tuning
S3_MAX_IDLE_CONNS=150
S3_MAX_IDLE_CONNS_PER_HOST=150
S3_IDLE_CONN_TIMEOUT_SEC=90
S3_CONNECT_TIMEOUT_SEC=10
S3_REQUEST_TIMEOUT_SEC=30
```

## Tuning Tips

- Increase `S3_MAX_IDLE_CONNS_PER_HOST` for high concurrency to single S3 endpoint
- Set `S3_MAX_CONNS_PER_HOST` to limit resource usage under burst traffic
- Adjust timeouts based on network latency to your S3 endpoint

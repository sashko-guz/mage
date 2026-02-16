# S3 HTTP Client Optimization

For high-throughput S3 access, configure `s3_http_config` in storage config.

## Example

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

## Options

- `max_idle_conns` - Max idle connections overall (default: 100)
- `max_idle_conns_per_host` - Max idle connections per host (default: 100)
- `max_conns_per_host` - Max total connections per host (`0` = unlimited)
- `idle_conn_timeout_sec` - Idle connection keepalive timeout
- `connect_timeout_sec` - TCP connect timeout
- `request_timeout_sec` - Full request timeout
- `response_header_timeout_sec` - Response header wait timeout

# Monitoring

Mage includes built-in Prometheus metrics and health check endpoints for production observability.

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/health` | Liveness probe - returns 200 if process is running |
| `/ready` | Readiness probe - checks storage connectivity, returns 503 if unhealthy |
| `/metrics` | Prometheus metrics endpoint |

## Quick Start

### With Docker Compose

```bash
# Start mage with monitoring stack (Prometheus + Grafana)
docker-compose -f docker-compose.local.yml --profile monitoring up -d
```

Access:
- **Mage**: http://localhost:8080
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)

### Without Docker

```bash
# Start mage
go run ./cmd/mage

# Check metrics
curl localhost:8080/metrics | grep mage_
```

## Metrics Reference

### HTTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mage_http_requests_total` | Counter | path, method, status | Total HTTP requests |
| `mage_http_request_duration_seconds` | Histogram | path, method | Request latency |
| `mage_http_active_connections` | Gauge | - | Current active connections |

### Image Processing

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mage_image_processing_duration_seconds` | Histogram | format | Thumbnail generation time |

### Cache Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mage_cache_hits_total` | Counter | type, layer | Cache hits (type: source/thumb, layer: memory/disk) |
| `mage_cache_misses_total` | Counter | type, layer | Cache misses |

### Storage Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mage_storage_operation_duration_seconds` | Histogram | operation, driver | Storage operation latency |

## Grafana Setup

1. Open Grafana: http://localhost:3000
2. Go to **Connections** → **Data sources** → **Add data source**
3. Select **Prometheus**
4. URL: `http://prometheus:9090`
5. Click **Save & test**

### Example Queries

```promql
# Request rate (requests/second)
rate(mage_http_requests_total[5m])

# 95th percentile latency
histogram_quantile(0.95, rate(mage_http_request_duration_seconds_bucket[5m]))

# Cache hit ratio
sum(rate(mage_cache_hits_total[5m])) / (sum(rate(mage_cache_hits_total[5m])) + sum(rate(mage_cache_misses_total[5m])))

# Error rate
sum(rate(mage_http_requests_total{status=~"5.."}[5m])) / sum(rate(mage_http_requests_total[5m]))
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `METRICS_ENABLED` | Enable /metrics endpoint | `true` |
| `METRICS_PATH` | Metrics endpoint path | `/metrics` |
| `HEALTH_READINESS_TIMEOUT_SECONDS` | Readiness check timeout | `5` |
| `GRAFANA_PASSWORD` | Grafana admin password (docker) | `admin` |

## Kubernetes

```yaml
# Pod annotations for Prometheus Operator
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"

# Probes
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

## Alerting Examples

```yaml
# prometheus/alerts.yml
groups:
  - name: mage
    rules:
      - alert: MageHighErrorRate
        expr: sum(rate(mage_http_requests_total{status=~"5.."}[5m])) / sum(rate(mage_http_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate (> 5%)"

      - alert: MageHighLatency
        expr: histogram_quantile(0.95, rate(mage_http_request_duration_seconds_bucket[5m])) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High p95 latency (> 2s)"

      - alert: MageLowCacheHitRate
        expr: sum(rate(mage_cache_hits_total[5m])) / (sum(rate(mage_cache_hits_total[5m])) + sum(rate(mage_cache_misses_total[5m]))) < 0.5
        for: 10m
        labels:
          severity: info
        annotations:
          summary: "Low cache hit rate (< 50%)"
```

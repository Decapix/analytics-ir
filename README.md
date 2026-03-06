# analytics-ir

Self-hosted analytics tracking stack with private ingestion path:

`Frontend/Mobile -> FastAPI (gateway) -> Go event-collector -> ClickHouse`

## What's included

- Kubernetes manifests for dedicated analytics workloads (`k8s/analytics`)
- ClickHouse schema (`clickhouse/schema.sql`)
- Go event collector with in-memory buffering and batched inserts (`collector/`)
- FastAPI gateway integration for non-blocking analytics forwarding and private percentage API (`fastapi_app/main.py`)
- Frontend/mobile event contract and naming (`frontend_tracking.md`)

## Key behaviors

- Collector endpoint: `POST /events`
- Flush policy: every 500 buffered events or every 2 seconds
- Retry policy: short retries on ClickHouse insert errors, log failures, do not crash
- Privacy: raw IP is never stored; collector stores `ip_hash`
- ClickHouse table:
  - DB: `analytics`
  - Table: `analytics_events`
  - Engine: `MergeTree`
  - Partition: monthly by `timestamp`
  - Order key: `(event_name, entity_type, entity_id, timestamp)`

## Deployment order

1. Apply namespace and analytics infra:
   ```bash
   kubectl apply -f k8s/analytics/namespace.yaml
   kubectl apply -f k8s/analytics/clickhouse.yaml
   kubectl apply -f k8s/analytics/event-collector.yaml
   ```
2. Build and push the collector image and replace image name in `event-collector.yaml`.
3. Deploy FastAPI service with `COLLECTOR_URL` pointing to the internal collector service.

## Local collector run

```bash
cd collector
go mod tidy
go run ./cmd/event-collector
```

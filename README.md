# analytics-ir

Internal analytics stack for the Inside Runway platform.

```
Frontend / Mobile
       │
       ▼
  FastAPI (default ns)          ← only entry point from outside
       │  POST /events (fire & forget, non-blocking)
       ▼
  event-collector  (analytics ns, ClusterIP only)
       │  batched inserts, LZ4 compressed
       ▼
   ClickHouse  (analytics ns, StatefulSet)
```

> **Network policy:** the collector is completely inaccessible from the public internet and from every service except the `fastapi` pod. No Ingress, no LoadBalancer.

---

## Table of contents

1. [Architecture](#architecture)
2. [Event payload](#event-payload)
3. [Sending events](#sending-events)
   - [Python (FastAPI / httpx)](#python-fastapi--httpx)
   - [Python (requests, fire & forget)](#python-requests-fire--forget)
   - [JavaScript / TypeScript (fetch)](#javascript--typescript-fetch)
   - [curl](#curl)
4. [Query API](#query-api)
   - [GET /internal/event-names](#get-internalevent-names)
   - [GET /internal/last-events](#get-internallast-events)
   - [GET /internal/percentages](#get-internalpercentages)
   - [GET /healthz](#get-healthz)
5. [How the collector works internally](#how-the-collector-works-internally)
6. [Environment variables](#environment-variables)
7. [Local development](#local-development)
8. [Deployment](#deployment)
9. [Project structure](#project-structure)

---

## Architecture

| Component | Technology | Location |
|---|---|---|
| Event collector | Go 1.22 + Gin | `collector/` |
| Storage | ClickHouse 24.8 | StatefulSet in `analytics` namespace |
| Image | `rg.fr-par.scw.cloud/imageir/go-collector:latest` | Scaleway Container Registry |
| Node | `analitycs-ir` pool (DEV1-M, fr-par-2) | Dedicated, isolated from production workloads |

The collector is a thin, stateless HTTP server. It receives events, enriches them (session tracking, IP hashing, dedup ID), buffers them in memory, and flushes to ClickHouse in batches.

---

## Event payload

`POST http://event-collector.analytics.svc.cluster.local:8080/events`

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `event_name` | string | **yes** | What happened. Use `snake_case`. e.g. `page_view`, `button_click`, `purchase` |
| `entity_type` | string | **yes** | The type of object the event is about. e.g. `product`, `user`, `page`, `session` |
| `entity_id` | string | **yes** | The ID of that object. e.g. a product slug, user ID |
| `session_id` | string | **yes** | Client-generated session identifier |
| `source` | string | **yes** | Origin of the event. e.g. `web`, `ios`, `android`, `api` |
| `platform` | string | **yes** | Runtime context. e.g. `desktop`, `mobile`, `tablet`, `server` |
| `timestamp` | ISO 8601 string | no | Defaults to server time if omitted |
| `actor_user_id` | string | no | ID of the user who triggered the event |
| `actor_entity_id` | string | no | ID of a secondary acting entity (e.g. a stylist acting on a brand's profile) |
| `actor_entity_type` | string | no | Type of that secondary entity |
| `app_version` | string | no | App or API version. e.g. `1.4.2` |
| `device_type` | string | no | `desktop`, `phone`, `tablet` |
| `user_agent` | string | no | Auto-filled from the `User-Agent` request header if not provided |
| `ip_hash` | string | no | SHA-256 of the client IP. Auto-computed if not provided. Raw IP is never stored |
| `properties` | object | no | Any extra key/value data relevant to the event |

### Automatic enrichment (done by the collector, you don't need to set these)

| Field | What the collector does |
|---|---|
| `timestamp` | Set to `now()` UTC if not provided |
| `ip_hash` | SHA-256 of the client IP, stripped from port before hashing |
| `user_agent` | Taken from the HTTP `User-Agent` header if not in the body |
| `properties.collector_event_id` | A UUID added to every event for deduplication |
| `properties.session_continued` | Set to `true` when the `session_id` was seen within the last 30 minutes |

### Minimal valid payload

```json
{
  "event_name": "page_view",
  "entity_type": "page",
  "entity_id": "home",
  "session_id": "sess_abc123",
  "source": "web",
  "platform": "desktop"
}
```

### Full payload example

```json
{
  "timestamp": "2026-03-06T14:00:00Z",
  "event_name": "purchase",
  "entity_type": "product",
  "entity_id": "prod_789",
  "actor_user_id": "user_042",
  "actor_entity_id": "stylist_011",
  "actor_entity_type": "stylist",
  "session_id": "sess_abc123",
  "source": "web",
  "platform": "desktop",
  "app_version": "1.5.0",
  "device_type": "desktop",
  "properties": {
    "price": 129.90,
    "currency": "EUR",
    "category": "outerwear"
  }
}
```

### Response

```
HTTP 202 Accepted
{"status": "queued"}
```

The event is in memory. It will be flushed to ClickHouse within 2 seconds or when 500 events have accumulated, whichever comes first. The response is intentionally immediate — do not wait for a confirmation of persistence.

---

## Sending events

The collector is reachable inside the cluster at:

```
http://event-collector.analytics.svc.cluster.local:8080
```

Only the `fastapi` pod (namespace `default`, label `io.kompose.service: fastapi`) is allowed to connect. All other pods are blocked by NetworkPolicy.

### Python (FastAPI / httpx)

This is the recommended pattern: **fire and forget using a background task**. The FastAPI response is never blocked by the analytics call.

```python
import httpx
from fastapi import BackgroundTasks

COLLECTOR_URL = "http://event-collector.analytics.svc.cluster.local:8080/events"

def _send_event(payload: dict) -> None:
    try:
        with httpx.Client(timeout=2.0) as client:
            client.post(COLLECTOR_URL, json=payload)
    except Exception:
        pass  # analytics must never crash the main app

def track(background_tasks: BackgroundTasks, payload: dict) -> None:
    background_tasks.add_task(_send_event, payload)
```

Usage inside a route:

```python
@router.post("/products/{product_id}/buy")
async def buy_product(
    product_id: str,
    background_tasks: BackgroundTasks,
    current_user: User = Depends(get_current_user),
):
    # ... business logic ...

    track(background_tasks, {
        "event_name": "purchase",
        "entity_type": "product",
        "entity_id": product_id,
        "actor_user_id": str(current_user.id),
        "session_id": current_user.session_id,
        "source": "api",
        "platform": "server",
        "properties": {"price": 129.90, "currency": "EUR"},
    })

    return {"ok": True}
```

### Python (requests, fire & forget)

If you are not in a FastAPI context, use a daemon thread so the call never blocks:

```python
import requests
import threading

COLLECTOR_URL = "http://event-collector.analytics.svc.cluster.local:8080/events"

def track(payload: dict) -> None:
    def _send():
        try:
            requests.post(COLLECTOR_URL, json=payload, timeout=2)
        except Exception:
            pass

    t = threading.Thread(target=_send, daemon=True)
    t.start()
```

### JavaScript / TypeScript (fetch)

From a Next.js / Node.js backend (never from the browser — the collector is not publicly exposed):

```typescript
const COLLECTOR_URL =
  "http://event-collector.analytics.svc.cluster.local:8080/events";

export async function track(payload: Record<string, unknown>): Promise<void> {
  // fire and forget — do not await this in your business logic
  fetch(COLLECTOR_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    signal: AbortSignal.timeout(2000),
  }).catch(() => {
    // analytics must never crash the main app
  });
}
```

Usage:

```typescript
track({
  event_name: "page_view",
  entity_type: "page",
  entity_id: "/brand/acme",
  session_id: req.cookies.session_id,
  source: "web",
  platform: "desktop",
  actor_user_id: session.userId,
  properties: { referrer: req.headers.referer },
});
```

### curl

For local testing or one-off calls:

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{
    "event_name": "page_view",
    "entity_type": "page",
    "entity_id": "home",
    "session_id": "sess_test_001",
    "source": "web",
    "platform": "desktop",
    "properties": {"test": true}
  }'
```

---

## Query API

These endpoints are read-only and intended for internal dashboards and admin tools. They are only accessible from inside the cluster (same NetworkPolicy applies).

### GET /internal/event-names

Returns all distinct event names currently stored in ClickHouse, sorted alphabetically.

```
GET http://event-collector.analytics.svc.cluster.local:8080/internal/event-names
```

**Response**

```json
{
  "count": 10,
  "event_names": [
    "add_to_cart",
    "button_click",
    "checkout",
    "form_submit",
    "login",
    "logout",
    "page_view",
    "purchase",
    "search",
    "signup"
  ]
}
```

---

### GET /internal/last-events

Returns the most recent events stored in ClickHouse, ordered by `timestamp DESC`.

```
GET /internal/last-events?limit=20&event_name=page_view
```

**Query parameters**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | integer | `20` | Number of events to return. Max `1000` |
| `event_name` | string | *(none)* | Optional filter. If provided, only events with this name are returned |

**Response (without filter)**

```json
{
  "limit": 5,
  "count": 5,
  "events": [
    {
      "timestamp": "2026-03-06T14:05:13Z",
      "event_name": "purchase",
      "entity_type": "product",
      "entity_id": "prod_789",
      "actor_user_id": "user_042",
      "actor_entity_id": "",
      "actor_entity_type": "",
      "session_id": "sess_abc123",
      "source": "web",
      "platform": "desktop",
      "app_version": "1.5.0",
      "device_type": "desktop",
      "user_agent": "Mozilla/5.0 ...",
      "ip_hash": "4be83b31...",
      "properties": "{\"price\":129.9,\"currency\":\"EUR\",\"collector_event_id\":\"uuid...\"}"
    }
  ]
}
```

**Response (with `event_name` filter)**

The `event_name` filter used is echoed back in the response:

```json
{
  "limit": 3,
  "count": 3,
  "event_name": "page_view",
  "events": [ ... ]
}
```

> Note: `properties` is returned as a raw JSON string. Parse it with `JSON.parse()` / `json.loads()` if needed.

---

### GET /internal/percentages

Returns event distribution as percentages over a time window.

```
GET /internal/percentages?hours=24
```

**Query parameters**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `hours` | integer | `24` | Look-back window in hours |

**Response**

```json
{
  "interval_hours": 24,
  "results": [
    { "event_name": "page_view",    "total": 312, "percentage": 28.5 },
    { "event_name": "button_click", "total": 201, "percentage": 18.4 },
    { "event_name": "search",       "total": 189, "percentage": 17.3 }
  ]
}
```

---

### GET /healthz

Liveness check. Returns `200 OK` if the process is up.

```
GET /healthz
→ {"ok": true}
```

---

## How the collector works internally

```
POST /events
     │
     ├── Validate required fields (event_name, entity_type, entity_id, session_id, source, platform)
     ├── Enrich: timestamp, ip_hash, user_agent, collector_event_id, session_continued
     ├── Add to BatchBuffer (mutex-protected slice)
     │
     ├── If buffer size >= FLUSH_SIZE  ──► trigger flushAsync()
     └── Return 202 immediately

flushAsync() (non-blocking goroutine, one at a time via mutex guard)
     │
     ├── Drain buffer atomically
     ├── INSERT batch into ClickHouse with LZ4 compression
     │     └── Retry up to 3 times with 300ms delay on failure
     └── Log error if all retries exhausted (events are lost — no persistent queue)

Periodic ticker (every FLUSH_INTERVAL)
     └── trigger flushAsync() regardless of buffer size
```

### Session tracking

The session manager holds an in-memory `map[session_id] → last_seen_time`. On each event:

- If the `session_id` was **never seen before** → new session, `session_continued` is not set.
- If the `session_id` was last seen **within `SESSION_WINDOW`** (default 30 min) → continuation, `properties.session_continued = true`.
- If the `session_id` was last seen **after `SESSION_WINDOW`** → treated as a new session start.

This map is in-memory only. It resets on pod restart.

### Important limitations

- **No persistence queue.** If ClickHouse is down and all 3 retries fail, the in-memory batch is lost. This is acceptable for analytics but means no hard guarantee on delivery.
- **Single replica only.** Running multiple replicas would give each pod its own independent buffer and session map, causing split batches and inconsistent session tracking. Do not scale horizontally.
- **Pod restart loses buffered events.** The flush interval is 2 seconds, so the maximum data loss window on a clean restart is ~2 seconds of events.

---

## Environment variables

All variables are provided via the `analytics-secret` Kubernetes Secret in the `analytics` namespace.

| Variable | Default | Description |
|---|---|---|
| `CLICKHOUSE_ADDR` | `clickhouse:9000` | ClickHouse native protocol address (`host:port`) |
| `CLICKHOUSE_USER` | `default` | ClickHouse username |
| `CLICKHOUSE_PASSWORD` | *(empty)* | ClickHouse password |
| `CLICKHOUSE_DB` | `analytics` | ClickHouse database name |
| `HTTP_ADDR` | `:8080` | Address the HTTP server listens on |
| `FLUSH_SIZE` | `500` | Number of buffered events that triggers an immediate flush |
| `FLUSH_INTERVAL` | `2s` | Maximum time between flushes regardless of buffer size |
| `SESSION_WINDOW` | `30m` | Inactivity duration after which a session is considered new |
| `INSERT_TIMEOUT` | `5s` | Timeout for a single ClickHouse insert attempt (also used for query endpoints) |

---

## Local development

**Requirements:** Docker, Docker Compose

```bash
cd analytics
docker compose up -d
```

This starts:
- `clickhouse` on ports `9000` (native) and `8123` (HTTP)
- `event-collector` on port `8080`

**Send a single test event:**

```bash
./test_requests.sh
```

**Send 100 varied events (load test):**

```bash
./tests/load_test.sh
# or against a specific host:
./tests/load_test.sh http://localhost:8080
```

**Query the API locally:**

```bash
# All known event names
curl http://localhost:8080/internal/event-names

# Last 10 events
curl "http://localhost:8080/internal/last-events?limit=10"

# Last 5 page_view events
curl "http://localhost:8080/internal/last-events?limit=5&event_name=page_view"

# Event distribution over the last 48 hours
curl "http://localhost:8080/internal/percentages?hours=48"
```

**Run the collector directly (without Docker):**

```bash
cd analytics/collector
CLICKHOUSE_ADDR=localhost:9000 go run ./cmd/event-collector
```

---

## Deployment

### Prerequisites

- The `analitycs-ir` node pool exists in the Scaleway Kapsule cluster
- You are authenticated to the Scaleway container registry: `docker login rg.fr-par.scw.cloud`

### Build and push the image

```bash
cd analytics
docker build --no-cache -t rg.fr-par.scw.cloud/imageir/go-collector:latest ./collector/
docker push rg.fr-par.scw.cloud/imageir/go-collector:latest
```

### First-time deploy

Apply manifests in order — namespace and secret must exist before workloads:

```bash
kubectl apply -f k8s/analytics/namespace.yaml
kubectl apply -f k8s/analytics/analytics-secret.yaml
kubectl apply -f k8s/analytics/clickhouse.yaml
kubectl apply -f k8s/analytics/event-collector.yaml
```

Watch pods come up:

```bash
kubectl get pods -n analytics -w
```

Expected final state:

```
NAME                               READY   STATUS    RESTARTS
clickhouse-0                       1/1     Running   0
event-collector-<hash>-<hash>      1/1     Running   0
```

### Update the collector (code change)

```bash
docker build --no-cache -t rg.fr-par.scw.cloud/imageir/go-collector:latest ./collector/
docker push rg.fr-par.scw.cloud/imageir/go-collector:latest
kubectl rollout restart deployment/event-collector -n analytics
kubectl rollout status deployment/event-collector -n analytics
```

### Verify end-to-end from the fastapi pod

```bash
kubectl exec -n default deploy/fastapi -- python3 -c "
import urllib.request, json, datetime
payload = json.dumps({
  'event_name': 'deploy_smoke_test',
  'entity_type': 'system',
  'entity_id': 'prod',
  'session_id': 'smoke-001',
  'source': 'api',
  'platform': 'server',
}).encode()
req = urllib.request.Request(
  'http://event-collector.analytics.svc.cluster.local:8080/events',
  data=payload,
  headers={'Content-Type': 'application/json'},
  method='POST'
)
print(urllib.request.urlopen(req, timeout=5).read().decode())
"
```

Then confirm the event landed in ClickHouse:

```bash
kubectl exec -n analytics statefulset/clickhouse -- \
  clickhouse-client --query \
  "SELECT event_name, entity_id, timestamp FROM analytics.analytics_events ORDER BY timestamp DESC LIMIT 3"
```

---

## Project structure

```
analytics/
├── collector/                          # Go service
│   ├── cmd/event-collector/main.go     # Entrypoint — reads env, wires dependencies
│   ├── internal/
│   │   ├── api/server.go               # HTTP handlers (ingest, query endpoints)
│   │   ├── buffer/buffer.go            # Thread-safe in-memory batch buffer
│   │   ├── clickhouse/client.go        # ClickHouse connection, insert, queries, retry
│   │   ├── model/event.go              # AnalyticsEvent struct
│   │   └── session/manager.go          # In-memory session window tracker
│   ├── Dockerfile                      # Multi-stage: golang:1.22 → distroless
│   └── go.mod
├── clickhouse/
│   ├── schema.sql                      # Table DDL (also loaded as a ConfigMap in k8s)
│   └── users.xml                       # Allows non-localhost connections (empty password)
├── k8s/analytics/
│   ├── namespace.yaml                  # analytics namespace
│   ├── analytics-secret.yaml           # All env vars for the collector
│   ├── clickhouse.yaml                 # StatefulSet + Service + ConfigMaps (schema, users)
│   └── event-collector.yaml           # Deployment + Service + NetworkPolicy
├── tests/
│   └── load_test.sh                    # Sends 100 randomised events, reports pass/fail
├── test_requests.sh                    # Sends one event + health check
├── docker-compose.yaml                 # Local dev stack (clickhouse + event-collector)
└── README.md                           # This file
```

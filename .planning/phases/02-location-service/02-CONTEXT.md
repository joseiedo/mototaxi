# Phase 2: Location Service - Context

**Gathered:** 2026-03-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Go HTTP ingest service that validates driver GPS payloads, persists location to Redis, publishes to Kafka, and exposes Prometheus metrics. No simulator, no push server, no frontend — this phase delivers the Location Service binary and its Docker image only.

</domain>

<decisions>
## Implementation Decisions

### HTTP router

- Use `chi` as the HTTP router (lightweight, idiomatic, stdlib-compatible)
- Include chi's request logging middleware — logs method, path, status, latency per request
- Listen on `:8080` inside Docker
- No graceful shutdown — hard exit on SIGTERM/SIGINT

### Kafka publish strategy

- Sync publish — wait for broker acknowledgment before returning HTTP 200
- `kafka_publish_duration_ms` measures real round-trip to broker (honest Grafana signal)
- If Kafka publish fails: return HTTP 503
- Kafka client: `franz-go` (pure Go, low-allocation, no CGO — required for FROM scratch static binary)
- Message key: `driver_id` (guarantees per-driver ordering across partitions)
- Message value: JSON (re-encoded from validated struct — same shape as HTTP payload)

### Payload validation

- Required fields: `driver_id`, `lat`, `lng`, `bearing`, `speed_kmh`, `emitted_at`
- Strict range checks: lat -90–90, lng -180–180, bearing 0–360, speed >= 0
- Return HTTP 400 with JSON body `{"error": "..."}` on missing fields or range violations
- No Content-Type header validation — trust simulator to send JSON

### Dependency failure behavior

- Startup: ping both Redis and Kafka before accepting traffic — fail fast if either is not ready
- Kafka failure during request: return HTTP 503
- Redis failure during request: return HTTP 503 (fail entire request — no partial success)
- Expose `GET /health` returning HTTP 200 — used by Docker Compose healthcheck

### Claude's Discretion

- Go module name and internal package structure (`cmd/`, `internal/` layout vs flat)
- Redis client library selection (go-redis is standard)
- Prometheus library (prometheus/client_golang is standard)
- Exact docker-compose.yml service block for location-service (port, depends_on, replicas config)
- Kafka topic name and producer config (acks, timeouts)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- None — `location-service/` only has a `.gitkeep` (Phase 1 created the directory stub)

### Established Patterns

- Each Go service has its own `go.mod` / `go.sum` (decided in Phase 1 — separate modules, no dependency bleed)
- Static binary `FROM scratch` final stage — required by LSVC-05, pattern established in Phase 1 context
- `franz-go` must be used instead of CGO-based clients to enable static binary build

### Integration Points

- `docker-compose.yml` — Phase 2 adds the `location-service` service block (already has nginx, redis, redpanda stubs from Phase 1)
- Redis at `redis:6379` on the `mototaxi` Docker network
- Redpanda at `redpanda:9092` on the `mototaxi` Docker network
- `driver.location` topic already created by `redpanda-init` container (Phase 1)
- `location-service:8080` is internal only — no host port mapping (Nginx will front it in Phase 5)

</code_context>

<specifics>
## Specific Ideas

- `GEOADD drivers:geo` + `SET driver:{id}:latest` with 30s TTL — both writes on every POST (per LSVC-02)
- `GET /location/{driver_id}` reads from `driver:{id}:latest` key (per LSVC-03)
- Prometheus metrics endpoint at `GET /metrics` using standard Go runtime metrics plus custom counters (per LSVC-04)
- Custom metrics: `location_updates_received_total` (counter), `kafka_publish_duration_ms` (histogram), `redis_write_duration_ms` (histogram)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 02-location-service*
*Context gathered: 2026-03-05*

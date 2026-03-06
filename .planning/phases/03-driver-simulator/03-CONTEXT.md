# Phase 3: Driver Simulator - Context

**Gathered:** 2026-03-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Go service that seeds customer-driver assignments into Redis at startup, then runs N goroutines emitting realistic GPS updates to the Location Service continuously. No Push Server, no Frontend, no Nginx — this phase delivers the simulator binary and its Docker image only.

</domain>

<decisions>
## Implementation Decisions

### ID scheme
- Driver IDs: `driver-1`, `driver-2`, ... `driver-N` (prefixed strings, not plain integers or UUIDs)
- Customer IDs: `customer-1`, `customer-2`, ... `customer-N`
- Sequential 1:1 assignment: `driver-1 <-> customer-1`, `driver-2 <-> customer-2`, etc.
- `DRIVER_COUNT` controls both driver count and customer count — no separate `CUSTOMER_COUNT` env var
- Customer IDs appear as-is in URLs: `/track/customer-1` (no transformation needed)
- Redis assignment keys: `customer:customer-{N}:driver` → `"driver-{N}"` and `driver:driver-{N}:customer` → `"customer-{N}"`

### HTTP target
- `LOCATION_SERVICE_URL` env var, defaulting to `http://location-service:8080` — configurable so Phase 5 can switch to `http://nginx:80` without code changes
- One shared `http.Client` across all driver goroutines — goroutine-safe, pools TCP connections efficiently
- Pure fire-and-forget service: no HTTP server, no `/health`, no `/metrics` endpoint

### Startup behavior
- Ping Redis in a retry loop at startup (consistent with location-service pattern) — fatal exit if Redis unreachable after retries
- Seed assignments before starting movement loop — overwrite existing keys (idempotent, no DEL step needed)
- Assignment keys have no TTL — persist for the lifetime of the stack; Redis flushes on `docker compose down`
- `depends_on: condition: service_healthy` in docker-compose for Redis (belt-and-suspenders alongside the ping loop)

### Error tolerance
- POST /location failure (503, network error): log error with driver ID + HTTP status, skip that tick, resume on next `EMIT_INTERVAL_MS`
- Redis seeding failure at startup: `log.Fatalf` and exit — no assignments means Push Server has nothing to look up
- Log errors only — no per-emit success logging (10 lines/sec at default DRIVER_COUNT is noise)

### Claude's Discretion
- Bearing calculation formula (haversine/atan2 — standard geographic math)
- Speed variation within the 20–60 km/h range per segment (constant or randomized per leg)
- Go HTTP client timeout values
- Internal package structure (`cmd/`, `internal/` layout)
- Exact retry count/interval for the Redis ping loop
- Docker Compose `depends_on` ordering (beyond Redis)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `location-service/internal/handler/locationPayload`: defines the exact JSON shape the simulator must emit — `driver_id`, `lat`, `lng`, `bearing`, `speed_kmh`, `emitted_at` (RFC3339). Simulator must match this schema.
- `location-service/cmd/location-service/main.go`: startup ping-loop pattern (`store.Ping` + producer.Ping` before accepting traffic) — replicate for Redis ping in simulator.
- `envOr()` helper in location-service `main.go`: simple pattern for env var with default — reuse in simulator.

### Established Patterns
- Separate `go.mod` per service (Phase 1 decision) — simulator gets its own `module mototaxi/simulator`
- `FROM scratch` multi-stage Dockerfile — same pattern as location-service (`Dockerfile` in `simulator/`)
- `franz-go` is NOT needed — simulator is an HTTP client only, no Kafka dependency
- `go-redis/v9` for Redis writes (same version as location-service to keep dependency versions consistent)

### Integration Points
- Redis at `redis:6379` on the `mototaxi` Docker network — same as location-service
- Location Service at `http://location-service:8080` (default) on the `mototaxi` Docker network
- `docker-compose.yml` — Phase 3 adds the `simulator` service block (alongside existing `location-service`, `redis`, `redpanda` services)
- `.env.example` — Phase 3 adds `LOCATION_SERVICE_URL=http://location-service:8080`
- São Paulo bounding box: lat -23.65 to -23.45, lng -46.75 to -46.55 (from PROJECT.md)

</code_context>

<specifics>
## Specific Ideas

- Assignment Redis key pattern matches SIM-01 exactly: `customer:{customer_id}:driver` and `driver:{driver_id}:customer`
- Emit payload must include `emitted_at` as RFC3339 timestamp (used by Push Server to calculate end-to-end latency in Phase 4: `now - emitted_at`)
- Movement: point-to-point within bbox, pick random destination on arrival, never leave bbox (clamp if needed)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 03-driver-simulator*
*Context gathered: 2026-03-06*

# Phase 2: Location Service - Research

**Researched:** 2026-03-05
**Domain:** Go HTTP ingest service — chi router, franz-go Kafka producer, go-redis v9, prometheus/client_golang, FROM scratch Docker image
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**HTTP router**
- Use `chi` as the HTTP router (lightweight, idiomatic, stdlib-compatible)
- Include chi's request logging middleware — logs method, path, status, latency per request
- Listen on `:8080` inside Docker
- No graceful shutdown — hard exit on SIGTERM/SIGINT

**Kafka publish strategy**
- Sync publish — wait for broker acknowledgment before returning HTTP 200
- `kafka_publish_duration_ms` measures real round-trip to broker (honest Grafana signal)
- If Kafka publish fails: return HTTP 503
- Kafka client: `franz-go` (pure Go, low-allocation, no CGO — required for FROM scratch static binary)
- Message key: `driver_id` (guarantees per-driver ordering across partitions)
- Message value: JSON (re-encoded from validated struct — same shape as HTTP payload)

**Payload validation**
- Required fields: `driver_id`, `lat`, `lng`, `bearing`, `speed_kmh`, `emitted_at`
- Strict range checks: lat -90–90, lng -180–180, bearing 0–360, speed >= 0
- Return HTTP 400 with JSON body `{"error": "..."}` on missing fields or range violations
- No Content-Type header validation — trust simulator to send JSON

**Dependency failure behavior**
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

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| LSVC-01 | Accept `POST /location` with driver JSON payload, validate, publish to Kafka topic `driver.location` keyed by `driver_id`, return HTTP 200 | chi router + franz-go ProduceSync + validation struct patterns documented |
| LSVC-02 | Write `GEOADD drivers:geo` and `SET driver:{id}:latest` (30s TTL) to Redis on each update | go-redis v9 Pipeline with GeoAdd + Set patterns documented |
| LSVC-03 | Expose `GET /location/{driver_id}` returning current position from Redis | go-redis v9 Get + chi URLParam patterns documented |
| LSVC-04 | Expose `GET /metrics` with Go runtime metrics plus custom counters/histograms | prometheus/client_golang promauto + promhttp.HandlerFor patterns documented |
| LSVC-05 | Docker image uses multi-stage build with `FROM scratch` final stage (~5MB) | CGO_ENABLED=0 static binary Dockerfile pattern documented |
</phase_requirements>

---

## Summary

The Location Service is a Go HTTP ingest service with four responsibilities: validate driver GPS payloads, write to Redis (geo set + latest key with TTL), publish to Kafka synchronously, and expose Prometheus metrics. All four libraries are mature, well-documented, and work correctly inside a FROM scratch container because all are pure Go with zero CGO requirements.

The key constraint is the static binary requirement (LSVC-05): every library choice must be CGO-free. `franz-go` (pure Go Kafka) is the decisive choice here — it replaces `confluent-kafka-go` which requires librdkafka and cannot be statically linked. `go-redis`, `chi`, and `prometheus/client_golang` are all pure Go and have no special static-binary considerations.

The startup sequence is critical: the service must ping Redis (`redis.Ping`) and Kafka (`kadm.ListBrokers`) before the HTTP listener starts, calling `os.Exit(1)` on failure. During requests, both Redis pipeline failures and Kafka ProduceSync failures return HTTP 503 — no partial success. The Redis write uses a non-transactional pipeline (two commands, same key set, no cross-key atomicity needed) for a single network round-trip.

**Primary recommendation:** Use `cmd/location-service/main.go` as the entry point, `internal/handler/`, `internal/kafka/`, `internal/redis/`, and `internal/metrics/` packages for separation, and wire dependencies via constructor injection in `main.go`.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.2.3 | HTTP router with URLParam, middleware chain | Stdlib-compatible, zero external dependencies, wildly adopted for Go APIs |
| `github.com/twmb/franz-go/pkg/kgo` | latest v1 | Pure Go Kafka producer | Only CGO-free Kafka client suitable for FROM scratch builds; comparable throughput to librdkafka |
| `github.com/twmb/franz-go/pkg/kadm` | same module | Kafka admin — ListBrokers health ping | Companion package to kgo; ListBrokers is the idiomatic connectivity probe |
| `github.com/redis/go-redis/v9` | v9.x | Redis client — GEOADD, SET, GET, Ping | Official Redis Go client, pure Go, RESP3 support, pipeline primitives |
| `github.com/prometheus/client_golang` | v1.20+ | Metrics — counter, histogram, /metrics endpoint | The canonical Go Prometheus instrumentation library |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/go-chi/chi/v5/middleware` | same module | Logger, Recoverer built-in middleware | Use Logger for per-request logs; use Recoverer to catch panics in handlers |
| `github.com/prometheus/client_golang/prometheus/promauto` | same module | Auto-register metrics with a registry | Use `promauto.With(reg)` to avoid manual `MustRegister` calls |
| `github.com/prometheus/client_golang/prometheus/promhttp` | same module | HTTP handler for /metrics | `promhttp.HandlerFor(reg, ...)` — use custom registry, not global default |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `franz-go` | `confluent-kafka-go` | Confluent requires CGO (librdkafka) — incompatible with FROM scratch |
| `franz-go` | `segmentio/kafka-go` | kafka-go is pure Go too, but franz-go has better idempotent production and lower allocations |
| `go-redis/v9` | `valkey-go` | go-redis is the official Redis-org client; Valkey is a fork for Valkey server — use go-redis for Redis 7 |
| `chi` | `gorilla/mux` | gorilla/mux is archived; chi is actively maintained and stdlib-compatible |
| `chi` | `gin` | gin uses a custom `Context` type breaking net/http compatibility; chi keeps standard `http.Handler` |
| Global prometheus registry | Custom registry | Global registry collides in tests; custom registry via `prometheus.NewRegistry()` is testable |

**Installation:**
```bash
go get github.com/go-chi/chi/v5
go get github.com/twmb/franz-go/pkg/kgo
go get github.com/twmb/franz-go/pkg/kadm
go get github.com/redis/go-redis/v9
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promauto
go get github.com/prometheus/client_golang/prometheus/promhttp
```

---

## Architecture Patterns

### Recommended Project Structure

```
location-service/
├── cmd/
│   └── location-service/
│       └── main.go          # wire deps, start HTTP server
├── internal/
│   ├── handler/
│   │   └── location.go      # POST /location, GET /location/{id}, GET /health
│   ├── kafka/
│   │   └── producer.go      # franz-go client wrapper, ProduceSync
│   ├── redisstore/
│   │   └── store.go         # go-redis client wrapper, WriteLocation, ReadLocation
│   └── metrics/
│       └── metrics.go       # registry + counter + histograms, NewMetrics constructor
├── go.mod
├── go.sum
└── Dockerfile
```

**Why this layout:** Matches the established pattern from Phase 1 context (each Go service has its own `go.mod`). `internal/` prevents accidental import from sibling services. Each package has one clear responsibility. `main.go` owns dependency wiring only.

### Pattern 1: chi Router Wiring with Middleware

**What:** Register chi middleware globally, then mount individual routes.
**When to use:** All routes in this service.

```go
// Source: https://pkg.go.dev/github.com/go-chi/chi/v5
r := chi.NewRouter()
r.Use(middleware.Logger)    // logs method, path, status, latency per request
r.Use(middleware.Recoverer) // catch panics, return 500

r.Post("/location", h.HandlePostLocation)
r.Get("/location/{driverID}", h.HandleGetLocation)
r.Get("/health", h.HandleHealth)
r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

http.ListenAndServe(":8080", r)
```

### Pattern 2: franz-go Sync Producer

**What:** Create a kgo.Client at startup, use ProduceSync per request.
**When to use:** Every POST /location request.

```go
// Source: https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo
cl, err := kgo.NewClient(
    kgo.SeedBrokers("redpanda:9092"),
    kgo.RequiredAcks(kgo.LeaderAck()),           // leader ack — honest latency signal
    kgo.RecordDeliveryTimeout(5 * time.Second),  // fail fast on broker unavailability
    kgo.DisableIdempotentWrite(),                // not needed for single-partition ordered writes
)

record := &kgo.Record{
    Topic: "driver.location",
    Key:   []byte(payload.DriverID),   // guarantees per-driver partition ordering
    Value: marshaledJSON,
}
if err := cl.ProduceSync(ctx, record).FirstErr(); err != nil {
    http.Error(w, `{"error":"kafka unavailable"}`, http.StatusServiceUnavailable)
    return
}
```

**Note on RequiredAcks:** With default idempotent production enabled, franz-go requires `AllISRAcks`. Since the CONTEXT.md decision is leader ack (honest signal, not max durability), disable idempotent writes with `kgo.DisableIdempotentWrite()` and use `kgo.LeaderAck()`.

### Pattern 3: go-redis Pipeline for Dual Write

**What:** Send GEOADD and SET in a single pipeline round-trip.
**When to use:** Every POST /location, after successful Kafka publish.

```go
// Source: https://pkg.go.dev/github.com/redis/go-redis/v9
pipe := rdb.Pipeline()
pipe.GeoAdd(ctx, "drivers:geo", &redis.GeoLocation{
    Name:      payload.DriverID,
    Longitude: payload.Lng,
    Latitude:  payload.Lat,
})
pipe.Set(ctx, "driver:"+payload.DriverID+":latest", marshaledJSON, 30*time.Second)
if _, err := pipe.Exec(ctx); err != nil {
    http.Error(w, `{"error":"redis unavailable"}`, http.StatusServiceUnavailable)
    return
}
```

**Why Pipeline not TxPipeline:** No cross-key atomicity requirement. Both keys belong to the same logical driver update. TxPipeline (MULTI/EXEC) adds round-trip overhead with no safety benefit here.

### Pattern 4: Prometheus Custom Registry

**What:** Create an isolated registry, register metrics via promauto, expose via promhttp.HandlerFor.
**When to use:** All custom metrics in this service.

```go
// Source: https://prometheus.io/docs/guides/go-application/
reg := prometheus.NewRegistry()
// Include Go runtime metrics explicitly
reg.MustRegister(collectors.NewGoCollector())
reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

m := &Metrics{
    UpdatesReceived: promauto.With(reg).NewCounter(prometheus.CounterOpts{
        Name: "location_updates_received_total",
        Help: "Total number of location updates received via POST /location",
    }),
    KafkaDuration: promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
        Name:    "kafka_publish_duration_ms",
        Help:    "Kafka sync produce round-trip duration in milliseconds",
        Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500},
    }),
    RedisDuration: promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
        Name:    "redis_write_duration_ms",
        Help:    "Redis pipeline write duration in milliseconds",
        Buckets: []float64{0.5, 1, 2.5, 5, 10, 25, 50, 100},
    }),
}
```

**Observing histogram (milliseconds):**
```go
start := time.Now()
// ... operation ...
m.KafkaDuration.Observe(float64(time.Since(start).Milliseconds()))
```

### Pattern 5: Startup Health Ping (Fail Fast)

**What:** Verify Redis and Kafka connectivity before the HTTP listener starts.
**When to use:** In `main.go`, before `http.ListenAndServe`.

```go
// Redis ping
if err := rdb.Ping(context.Background()).Err(); err != nil {
    log.Fatalf("redis ping failed: %v", err)
}

// Kafka connectivity via kadm
adm := kadm.NewClient(kafkaClient)
if _, err := adm.ListBrokers(context.Background()); err != nil {
    log.Fatalf("kafka ping failed: %v", err)
}
```

### Pattern 6: FROM scratch Multi-Stage Dockerfile

**What:** Build static Go binary in golang builder stage, copy to scratch final stage.
**When to use:** Required for LSVC-05.

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o location-service ./cmd/location-service

# Runtime stage — no OS, no shell, binary only
FROM scratch
COPY --from=builder /app/location-service /location-service
EXPOSE 8080
ENTRYPOINT ["/location-service"]
```

**CA certificates:** This service does not make outbound HTTPS calls (internal Docker network only), so copying `/etc/ssl/certs/ca-certificates.crt` is not required.

### Anti-Patterns to Avoid

- **Global prometheus registry:** `prometheus.MustRegister(...)` on the default registry panics in tests if called more than once. Always use a custom registry.
- **TxPipeline for GEOADD+SET:** Adds MULTI/EXEC overhead for no atomicity benefit (same driver, different key types, independent keys).
- **Async Kafka produce:** The CONTEXT.md decision is sync — using `cl.Produce(ctx, record, callback)` defeats the honest latency measurement goal.
- **CGO-enabled Kafka client:** `confluent-kafka-go` links librdkafka and cannot produce a static binary. FROM scratch build will fail at runtime with "no such file or directory".
- **Registering metrics in `init()`:** Makes test isolation impossible. Inject the registry via constructor.
- **JSON decode without field presence check:** `encoding/json` sets missing fields to zero values — validate after decode with explicit checks, not just by checking zero value (0 is a valid bearing and speed).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP routing with path params | Custom ServeMux with string splitting | `chi` + `chi.URLParam` | Path parameter extraction, middleware chain, stdlib-compatible |
| Kafka producer with acks | Raw TCP + Kafka wire protocol | `franz-go` kgo | Batching, retry, idempotency, partition leader resolution |
| Redis pipelining | Two sequential `Exec` calls | `go-redis Pipeline()` | Single network round-trip, atomic batch submission |
| Prometheus /metrics endpoint | Custom text formatter | `promhttp.HandlerFor` | Wire format, content-type negotiation, compression |
| Histogram bucket definition | Manual bucket math | `prometheus.DefBuckets` or explicit slice | Prometheus conventions for latency histograms are well-studied |
| Static binary build flags | Researching linker flags | `-ldflags="-s -w"` + `CGO_ENABLED=0` | Symbol stripping and CGO disabling are the two required flags |

**Key insight:** The biggest trap is rolling a custom request router or JSON validation framework. Both are solved problems with idiomatic Go solutions that handle edge cases (URL encoding, concurrent reads, zero-value ambiguity) that custom code misses on the first attempt.

---

## Common Pitfalls

### Pitfall 1: franz-go RequiredAcks Incompatibility with Idempotent Writes

**What goes wrong:** By default, franz-go enables idempotent production which requires `AllISRAcks`. If you set `kgo.LeaderAck()` without also setting `kgo.DisableIdempotentWrite()`, the client returns an error at construction time.
**Why it happens:** Kafka's idempotent producer protocol requires all-ISR acks by spec.
**How to avoid:** Always pair `kgo.LeaderAck()` with `kgo.DisableIdempotentWrite()` when targeting leader-only acknowledgment.
**Warning signs:** `NewClient` returns an error mentioning "idempotent" or "required acks".

### Pitfall 2: Validating After JSON Decode (Zero Value Ambiguity)

**What goes wrong:** `lat: 0`, `lng: 0`, `bearing: 0`, and `speed_kmh: 0` are all valid values. Using `if payload.Lat == 0` to detect missing fields will reject valid equatorial coordinates.
**Why it happens:** `encoding/json` sets unset fields to zero values — there is no "present/absent" distinction.
**How to avoid:** Use pointer fields (`*float64`) in the decode target so absent fields are `nil`, or use a separate struct with `json.RawMessage` and manual presence detection.
**Warning signs:** Test cases with `lat: 0, lng: 0` return HTTP 400 incorrectly.

### Pitfall 3: Histogram Metric Name Mismatch

**What goes wrong:** Requirements specify `kafka_publish_duration_ms` but Prometheus conventions typically use `_seconds` suffix. Grafana dashboards will query by exact name.
**Why it happens:** Prometheus docs encourage `_seconds` for durations, but the requirements specify `_ms`.
**How to avoid:** Use the exact names from REQUIREMENTS.md: `kafka_publish_duration_ms` and `redis_write_duration_ms`. Observe with `.Milliseconds()` not `.Seconds()`.
**Warning signs:** Grafana panels show "No data" because metric name is `kafka_publish_duration_seconds` instead of `kafka_publish_duration_ms`.

### Pitfall 4: Pipeline Error on Partial Command Failure

**What goes wrong:** `pipe.Exec(ctx)` returns `([]Cmder, error)` where error is non-nil if ANY command failed. The returned `[]Cmder` may contain partial successes.
**Why it happens:** Redis pipeline executes all commands; errors are per-command, not pipeline-level.
**How to avoid:** Check only `err` from `pipe.Exec` — if non-nil, treat the entire write as failed (per CONTEXT.md decision: no partial success). Do not iterate `[]Cmder`.
**Warning signs:** GEOADD succeeds but SET fails silently because code inspects individual command results instead of the pipeline-level error.

### Pitfall 5: FROM scratch Missing /etc/ssl/certs

**What goes wrong:** If a future handler makes an HTTPS call (e.g., external geocoding API), it fails with "x509: certificate signed by unknown authority".
**Why it happens:** Scratch image has no filesystem — no CA bundle.
**How to avoid:** This phase makes no external HTTPS calls (Redis and Redpanda are plain TCP on the internal Docker network). Document this constraint so Phase 3+ are aware.
**Warning signs:** TLS errors from inside the container; `x509` errors in logs.

### Pitfall 6: Kafka ProduceSync Blocks Forever Without Timeout

**What goes wrong:** If Redpanda is unreachable and `RecordDeliveryTimeout` is not set, `ProduceSync` blocks until the request context expires or forever.
**Why it happens:** franz-go default retries forever (`RecordRetries: math.MaxInt`).
**How to avoid:** Always set `kgo.RecordDeliveryTimeout(5 * time.Second)` (or use a context with deadline).
**Warning signs:** POST /location hangs for minutes during Redpanda restarts instead of returning 503.

---

## Code Examples

Verified patterns from official sources:

### GET /location/{driverID} Handler

```go
// Source: https://pkg.go.dev/github.com/go-chi/chi/v5
func (h *Handler) HandleGetLocation(w http.ResponseWriter, r *http.Request) {
    driverID := chi.URLParam(r, "driverID")
    val, err := h.redis.Get(r.Context(), "driver:"+driverID+":latest").Result()
    if err == redis.Nil {
        http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
        return
    }
    if err != nil {
        http.Error(w, `{"error":"redis unavailable"}`, http.StatusServiceUnavailable)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(val))
}
```

### Pointer-Field Validation Struct

```go
// Avoids zero-value ambiguity per Pitfall 2
type locationPayload struct {
    DriverID  string   `json:"driver_id"`
    Lat       *float64 `json:"lat"`
    Lng       *float64 `json:"lng"`
    Bearing   *float64 `json:"bearing"`
    SpeedKmh  *float64 `json:"speed_kmh"`
    EmittedAt string   `json:"emitted_at"`
}

func (p *locationPayload) validate() error {
    if p.DriverID == "" { return errors.New("missing driver_id") }
    if p.Lat == nil { return errors.New("missing lat") }
    if *p.Lat < -90 || *p.Lat > 90 { return errors.New("lat out of range") }
    if p.Lng == nil { return errors.New("missing lng") }
    if *p.Lng < -180 || *p.Lng > 180 { return errors.New("lng out of range") }
    if p.Bearing == nil { return errors.New("missing bearing") }
    if *p.Bearing < 0 || *p.Bearing > 360 { return errors.New("bearing out of range") }
    if p.SpeedKmh == nil { return errors.New("missing speed_kmh") }
    if *p.SpeedKmh < 0 { return errors.New("speed_kmh must be >= 0") }
    if p.EmittedAt == "" { return errors.New("missing emitted_at") }
    return nil
}
```

### Docker Compose Service Block (Claude's Discretion)

```yaml
location-service:
  build:
    context: ./location-service
    dockerfile: Dockerfile
  restart: unless-stopped
  depends_on:
    redis:
      condition: service_healthy
    redpanda-init:
      condition: service_completed_successfully
  environment:
    REDIS_ADDR: redis:6379
    KAFKA_ADDR: redpanda:9092
    KAFKA_TOPIC: driver.location
  networks:
    - mototaxi
  # No host port mapping — Nginx fronts this service in Phase 5
```

**Note:** Prometheus scrape config (Phase 7) will target `location-service:8080` internally. No ports block needed for metrics.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `segmentio/kafka-go` for Go Kafka | `franz-go` as the preferred pure-Go client | ~2022 | franz-go has dramatically lower allocations and better protocol coverage |
| `go-redis/v8` | `redis/go-redis/v9` (org transferred to Redis) | v9 released 2022 | Import path changed to `github.com/redis/go-redis/v9`; v9 adds RESP3 |
| Global `prometheus.DefaultRegisterer` | Custom `prometheus.NewRegistry()` | Client_golang v1.x | Test isolation; avoids "already registered" panics |
| Alpine-based minimal images | `FROM scratch` for pure-Go services | Ongoing | Scratch is smaller and has zero CVE surface; requires CGO_ENABLED=0 |
| `gorilla/mux` | `go-chi/chi` | gorilla/mux archived 2022 | chi is the actively maintained stdlib-compatible router |

**Deprecated/outdated:**
- `go-redis/v8`: Import path `github.com/go-redis/redis/v8` is the old path — use `github.com/redis/go-redis/v9`
- `gorilla/mux`: Archived, no maintenance — do not use
- `confluent-kafka-go`: CGO dependency makes it incompatible with FROM scratch

---

## Open Questions

1. **Go module name**
   - What we know: Each service has its own `go.mod` (established in Phase 1)
   - What's unclear: Exact module path (e.g., `github.com/user/mototaxi/location-service` vs `mototaxi/location-service`)
   - Recommendation: Use `mototaxi/location-service` (local dev only, no publish intent)

2. **Kafka acks=1 vs default idempotent**
   - What we know: CONTEXT.md says leader ack; franz-go requires DisableIdempotentWrite to use LeaderAck
   - What's unclear: Whether Redpanda v25 supports idempotent production without special config
   - Recommendation: Use `DisableIdempotentWrite() + LeaderAck()` as documented — correct and explicit

3. **`emitted_at` format validation**
   - What we know: Field must be present; range checks don't apply
   - What's unclear: Should the service validate it as a valid RFC3339 timestamp or just check non-empty?
   - Recommendation: Parse as `time.Time` with `time.RFC3339` to catch malformed timestamps early — return 400 on parse failure

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `go test` (stdlib) |
| Config file | none — standard Go test conventions |
| Quick run command | `cd location-service && go test ./...` |
| Full suite command | `cd location-service && go test ./... -race -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LSVC-01 | POST /location valid payload returns 200, Kafka message published | integration | `go test ./internal/handler/ -run TestPostLocation` | Wave 0 |
| LSVC-01 | POST /location missing field returns 400 with JSON error | unit | `go test ./internal/handler/ -run TestPostLocationValidation` | Wave 0 |
| LSVC-01 | POST /location lat out of range returns 400 | unit | `go test ./internal/handler/ -run TestPostLocationRangeCheck` | Wave 0 |
| LSVC-02 | POST /location writes GEOADD + SET to Redis with 30s TTL | integration | `go test ./internal/redisstore/ -run TestWriteLocation` | Wave 0 |
| LSVC-03 | GET /location/{id} returns 200 with cached position | integration | `go test ./internal/handler/ -run TestGetLocation` | Wave 0 |
| LSVC-03 | GET /location/{id} for unknown driver returns 404 | unit | `go test ./internal/handler/ -run TestGetLocationNotFound` | Wave 0 |
| LSVC-04 | GET /metrics returns 200 with Prometheus text format | unit | `go test ./internal/handler/ -run TestMetricsEndpoint` | Wave 0 |
| LSVC-04 | location_updates_received_total increments on valid POST | unit | `go test ./internal/metrics/ -run TestCounterIncrement` | Wave 0 |
| LSVC-05 | Docker image builds to static binary FROM scratch | smoke | `docker build location-service/ && docker inspect --format='{{.Size}}'` | manual |

**Integration test strategy:** Use `miniredis` (github.com/alicebob/miniredis/v2) for Redis mocking and a mock Kafka producer interface for handler tests — avoids requiring live infrastructure for unit/integration tests.

### Sampling Rate

- **Per task commit:** `cd location-service && go test ./...`
- **Per wave merge:** `cd location-service && go test ./... -race -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `location-service/go.mod` — module declaration and dependency versions
- [ ] `location-service/internal/handler/location_test.go` — covers LSVC-01, LSVC-03
- [ ] `location-service/internal/redisstore/store_test.go` — covers LSVC-02
- [ ] `location-service/internal/metrics/metrics_test.go` — covers LSVC-04
- [ ] Dependency: `go get github.com/alicebob/miniredis/v2` — test-only, for Redis mocking

---

## Sources

### Primary (HIGH confidence)

- `pkg.go.dev/github.com/twmb/franz-go/pkg/kgo` — ProduceSync, RequiredAcks, RecordDeliveryTimeout, DisableIdempotentWrite
- `pkg.go.dev/github.com/twmb/franz-go/pkg/kadm` — ListBrokers health check
- `pkg.go.dev/github.com/redis/go-redis/v9` — GeoAdd, Set, Get, Pipeline, Ping signatures
- `pkg.go.dev/github.com/go-chi/chi/v5` — URLParam, middleware.Logger, Router wiring
- `prometheus.io/docs/guides/go-application/` — custom registry, promauto.With, promhttp.HandlerFor

### Secondary (MEDIUM confidence)

- `github.com/go-chi/chi/releases` — v5.2.3 is the current tag (verified via GitHub)
- `redis.uptrace.dev/guide/go-redis-pipelines.html` — Pipeline vs TxPipeline tradeoffs
- `github.com/twmb/franz-go/blob/master/docs/producing-and-consuming.md` — ProduceSync pattern

### Tertiary (LOW confidence)

- WebSearch result: "Go 1.24 is the latest stable version" — unverified; use whatever is in project go.mod; actual version does not affect API patterns used here

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries verified via pkg.go.dev official docs
- Architecture: HIGH — patterns verified against official documentation
- Pitfalls: HIGH (Pitfalls 1, 2, 3) / MEDIUM (Pitfalls 4, 5, 6) — franz-go acks incompatibility and zero-value validation are well-documented; pipeline error handling and scratch TLS certs are from primary sources

**Research date:** 2026-03-05
**Valid until:** 2026-04-04 (30 days — stable libraries with infrequent breaking changes)

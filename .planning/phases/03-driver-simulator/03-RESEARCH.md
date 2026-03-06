# Phase 3: Driver Simulator - Research

**Researched:** 2026-03-06
**Domain:** Go goroutine-per-driver GPS simulator with Redis seeding and HTTP emission
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**ID scheme**
- Driver IDs: `driver-1`, `driver-2`, ... `driver-N` (prefixed strings, not plain integers or UUIDs)
- Customer IDs: `customer-1`, `customer-2`, ... `customer-N`
- Sequential 1:1 assignment: `driver-1 <-> customer-1`, `driver-2 <-> customer-2`, etc.
- `DRIVER_COUNT` controls both driver count and customer count — no separate `CUSTOMER_COUNT` env var
- Customer IDs appear as-is in URLs: `/track/customer-1` (no transformation needed)
- Redis assignment keys: `customer:customer-{N}:driver` → `"driver-{N}"` and `driver:driver-{N}:customer` → `"customer-{N}"`

**HTTP target**
- `LOCATION_SERVICE_URL` env var, defaulting to `http://location-service:8080` — configurable so Phase 5 can switch to `http://nginx:80` without code changes
- One shared `http.Client` across all driver goroutines — goroutine-safe, pools TCP connections efficiently
- Pure fire-and-forget service: no HTTP server, no `/health`, no `/metrics` endpoint

**Startup behavior**
- Ping Redis in a retry loop at startup (consistent with location-service pattern) — fatal exit if Redis unreachable after retries
- Seed assignments before starting movement loop — overwrite existing keys (idempotent, no DEL step needed)
- Assignment keys have no TTL — persist for the lifetime of the stack; Redis flushes on `docker compose down`
- `depends_on: condition: service_healthy` in docker-compose for Redis (belt-and-suspenders alongside the ping loop)

**Error tolerance**
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

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SIM-01 | Seed `customer:{id}:driver` and `driver:{id}:customer` keys into Redis before movement loop starts | Redis SET pattern with go-redis/v9; idempotent write loop covered |
| SIM-02 | Each goroutine moves point-to-point within São Paulo bbox at 20–60 km/h, picks new destination on arrival | Haversine distance + bearing formula; time.Ticker for movement cadence |
| SIM-03 | POST /location with bearing, speed_kmh, emitted_at on EMIT_INTERVAL_MS cadence per goroutine | locationPayload schema from location-service; time.Ticker pattern |
| SIM-04 | Docker image uses multi-stage build FROM scratch final stage | Identical Dockerfile pattern to location-service confirmed |
| SIM-05 | DRIVER_COUNT and EMIT_INTERVAL_MS configurable via env vars | envOr() helper pattern from location-service; strconv.Atoi for integers |
</phase_requirements>

---

## Summary

Phase 3 is a pure Go client-side service: no HTTP server, no Kafka, no complex frameworks. The simulator seeds Redis with driver-customer assignment pairs at startup, then spawns N goroutines that each maintain a simulated driver's position and emit HTTP POST requests to the Location Service on a timer. The technical complexity concentrates in two areas: geographic movement math (haversine bearing + incremental position update within the São Paulo bounding box) and reliable goroutine lifecycle (clean startup sequencing, error-tolerant tick loop, graceful shutdown via context cancellation).

The existing location-service codebase provides direct templates for every structural pattern: the `envOr()` helper for config, the Redis ping-retry startup pattern, the `go-redis/v9` client initialization, and the multi-stage `FROM scratch` Dockerfile. The simulator's `locationPayload` JSON shape is already defined and validated by the Location Service — the simulator just needs to produce exactly that schema (pointer fields not needed on the sender side since all fields are always present).

**Primary recommendation:** Build simulator as `module mototaxi/simulator` under `simulator/`, replicating the location-service module structure with `cmd/simulator/main.go` and thin `internal/` packages for seeding, movement math, and the HTTP emitter. Use `time.NewTicker` for the emit loop and `math` package for haversine — no external geo libraries needed.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/redis/go-redis/v9` | v9.18.0 | Redis SET for assignment seeding + Ping at startup | Same version as location-service — consistent dep versions across the mono-repo |
| Go standard `net/http` | stdlib | Shared `http.Client` for POST /location | No external HTTP client needed; stdlib Client is goroutine-safe with connection pooling |
| Go standard `math` | stdlib | `math.Sin`, `math.Cos`, `math.Atan2` for haversine + bearing | No external geo library needed; these 4 functions cover all movement math |
| Go standard `encoding/json` | stdlib | Marshal locationPayload to JSON body | Already used in location-service; same pattern |
| Go standard `time` | stdlib | `time.NewTicker` for emit cadence, `time.RFC3339` for emitted_at | All timing and timestamp formatting |
| Go standard `math/rand` | stdlib | Random destination within bbox, random speed per segment | `rand.Float64()` for uniform random in range |
| Go standard `context` | stdlib | Cancellation signal to goroutines for graceful shutdown | Standard Go concurrency pattern |
| Go standard `sync` | stdlib | `sync.WaitGroup` to wait for all goroutines before exit | Standard goroutine lifecycle |
| Go standard `log` | stdlib | `log.Printf` for error logging, `log.Fatalf` for fatal startup errors | Consistent with location-service (no structured logger needed here) |
| Go standard `os` | stdlib | `os.Getenv` via `envOr()` helper | Env var config |
| Go standard `strconv` | stdlib | `strconv.Atoi` for DRIVER_COUNT and EMIT_INTERVAL_MS (string → int) | Required for integer env vars |
| Go standard `fmt` | stdlib | `fmt.Sprintf` for ID construction ("driver-%d") | ID generation |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/alicebob/miniredis/v2` | v2.37.0 | In-process Redis for seeder unit tests | Test-only dependency; same version as location-service |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `math` | `github.com/paulmach/orb` or similar geo library | External geo library adds dep weight; haversine is ~20 lines of math — no library justified |
| stdlib `net/http` | `github.com/go-resty/resty` | No benefit; stdlib client with `http.NewRequest` is sufficient for fire-and-forget POST |
| `time.NewTicker` | Custom sleep loop | Ticker self-corrects for drift; sleep loop accumulates drift over time |

**Installation:**
```bash
# From simulator/ directory
go mod init mototaxi/simulator
go get github.com/redis/go-redis/v9@v9.18.0
# Test only:
go get github.com/alicebob/miniredis/v2@v2.37.0
```

---

## Architecture Patterns

### Recommended Project Structure

```
simulator/
├── cmd/
│   └── simulator/
│       └── main.go        # env config, Redis init, seed, launch goroutines, wait
├── internal/
│   ├── seeder/
│   │   └── seeder.go      # SeedAssignments(ctx, client, driverCount) error
│   ├── geo/
│   │   └── geo.go         # Bearing(), Distance(), NextPosition(), BboxClamp()
│   └── emitter/
│       └── emitter.go     # RunDriver(ctx, id, httpClient, locationURL, intervalMs)
├── Dockerfile
└── go.mod
```

### Pattern 1: Startup Sequence

**What:** Linear startup — config validation → Redis ping retry → seed → goroutine launch
**When to use:** All startup code; mirrors location-service pattern exactly

```go
// main.go pattern (mirrors location-service/cmd/location-service/main.go)
func main() {
    redisAddr         := envOr("REDIS_ADDR", "redis:6379")
    locationURL       := envOr("LOCATION_SERVICE_URL", "http://location-service:8080")
    driverCount       := mustInt(envOr("DRIVER_COUNT", "10"))
    emitIntervalMs    := mustInt(envOr("EMIT_INTERVAL_MS", "1000"))

    rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
    pingWithRetry(rdb) // fatal exit if unreachable

    if err := seeder.SeedAssignments(context.Background(), rdb, driverCount); err != nil {
        log.Fatalf("seed assignments: %v", err)
    }

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    client := &http.Client{Timeout: 5 * time.Second}
    var wg sync.WaitGroup
    for i := 1; i <= driverCount; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            emitter.RunDriver(ctx, id, client, locationURL, emitIntervalMs)
        }(i)
    }
    wg.Wait()
}
```

### Pattern 2: Redis Assignment Seeding

**What:** Write both forward and reverse assignment keys with `MSet` (single round-trip for all 2N keys)
**When to use:** SIM-01 implementation

```go
// seeder/seeder.go
func SeedAssignments(ctx context.Context, rdb *redis.Client, n int) error {
    pairs := make([]interface{}, 0, n*4)
    for i := 1; i <= n; i++ {
        driverID   := fmt.Sprintf("driver-%d", i)
        customerID := fmt.Sprintf("customer-%d", i)
        pairs = append(pairs,
            fmt.Sprintf("customer:%s:driver", customerID), driverID,
            fmt.Sprintf("driver:%s:customer", driverID), customerID,
        )
    }
    return rdb.MSet(ctx, pairs...).Err()
}
```

Note: `MSet` is idempotent — re-running simply overwrites existing keys. No TTL set (as decided).

### Pattern 3: Ping Retry Loop

**What:** Retry Redis Ping up to N times with sleep between attempts before fatal exit
**When to use:** Startup, before SeedAssignments

```go
func pingWithRetry(rdb *redis.Client) {
    const maxAttempts = 10
    const retryInterval = 2 * time.Second
    ctx := context.Background()
    for i := 1; i <= maxAttempts; i++ {
        if err := rdb.Ping(ctx).Err(); err == nil {
            log.Printf("redis connected on attempt %d", i)
            return
        }
        log.Printf("redis ping attempt %d/%d failed, retrying in %v", i, maxAttempts, retryInterval)
        time.Sleep(retryInterval)
    }
    log.Fatalf("redis unreachable after %d attempts", maxAttempts)
}
```

### Pattern 4: Driver Goroutine (Tick Loop)

**What:** Each goroutine maintains private state (current position, destination, speed) and ticks on a `time.Ticker`
**When to use:** SIM-02 + SIM-03

```go
// emitter/emitter.go
func RunDriver(ctx context.Context, id int, client *http.Client, locationURL string, intervalMs int) {
    driverID := fmt.Sprintf("driver-%d", id)
    ticker   := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
    defer ticker.Stop()

    cur := geo.RandomPoint()         // random start within bbox
    dst := geo.RandomPoint()         // random destination
    speed := geo.RandomSpeed()       // 20-60 km/h, constant per leg

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            bearing  := geo.Bearing(cur, dst)
            cur       = geo.StepToward(cur, dst, speed, float64(intervalMs)/1000.0)
            if geo.Arrived(cur, dst) {
                dst   = geo.RandomPoint()
                speed = geo.RandomSpeed()
            }
            emitLocation(client, locationURL, driverID, cur.Lat, cur.Lng, bearing, speed)
        }
    }
}
```

### Pattern 5: Geographic Math (Haversine)

**What:** Standard haversine formula for Earth-surface distance and bearing calculation
**When to use:** SIM-02 — bearing, step-toward, arrival detection

```go
// geo/geo.go — all using stdlib math only
const (
    earthRadiusKm = 6371.0
    latMin, latMax = -23.65, -23.45  // São Paulo bbox
    lngMin, lngMax = -46.75, -46.55
)

type Point struct{ Lat, Lng float64 }

// Bearing returns the initial bearing in degrees [0, 360) from a to b.
func Bearing(a, b Point) float64 {
    lat1 := a.Lat * math.Pi / 180
    lat2 := b.Lat * math.Pi / 180
    dLng := (b.Lng - a.Lng) * math.Pi / 180
    x := math.Sin(dLng) * math.Cos(lat2)
    y := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLng)
    θ := math.Atan2(x, y) * 180 / math.Pi
    return math.Mod(θ+360, 360)
}

// DistanceKm returns the haversine distance in km between two points.
func DistanceKm(a, b Point) float64 {
    dLat := (b.Lat - a.Lat) * math.Pi / 180
    dLng := (b.Lng - a.Lng) * math.Pi / 180
    lat1 := a.Lat * math.Pi / 180
    lat2 := b.Lat * math.Pi / 180
    s := math.Sin(dLat/2)*math.Sin(dLat/2) +
        math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
    return 2 * earthRadiusKm * math.Asin(math.Sqrt(s))
}

// StepToward moves point a toward b at speedKmh over elapsedSec seconds,
// returning the new position (clamped to bbox if needed).
func StepToward(a, b Point, speedKmh, elapsedSec float64) Point {
    distKm := DistanceKm(a, b)
    stepKm := speedKmh * elapsedSec / 3600.0
    if stepKm >= distKm {
        return clamp(b) // arrived
    }
    fraction := stepKm / distKm
    newLat := a.Lat + fraction*(b.Lat-a.Lat)
    newLng := a.Lng + fraction*(b.Lng-a.Lng)
    return clamp(Point{newLat, newLng})
}

// Arrived returns true when cur is within 10m of dst.
func Arrived(cur, dst Point) bool {
    return DistanceKm(cur, dst) < 0.01
}

func clamp(p Point) Point {
    return Point{
        Lat: math.Max(latMin, math.Min(latMax, p.Lat)),
        Lng: math.Max(lngMin, math.Min(lngMax, p.Lng)),
    }
}

func RandomPoint() Point {
    return Point{
        Lat: latMin + rand.Float64()*(latMax-latMin),
        Lng: lngMin + rand.Float64()*(lngMax-lngMin),
    }
}

func RandomSpeed() float64 {
    return 20.0 + rand.Float64()*40.0 // [20, 60) km/h
}
```

### Pattern 6: HTTP Emit (fire-and-forget)

**What:** Construct JSON body, POST to Location Service, log non-200 and network errors, continue
**When to use:** Every tick, SIM-03

```go
// locationPayload matches the schema validated by location-service exactly
type locationPayload struct {
    DriverID  string  `json:"driver_id"`
    Lat       float64 `json:"lat"`
    Lng       float64 `json:"lng"`
    Bearing   float64 `json:"bearing"`
    SpeedKmh  float64 `json:"speed_kmh"`
    EmittedAt string  `json:"emitted_at"`  // time.Now().UTC().Format(time.RFC3339)
}

func emitLocation(client *http.Client, url, driverID string, lat, lng, bearing, speed float64) {
    payload := locationPayload{
        DriverID:  driverID,
        Lat:       lat,
        Lng:       lng,
        Bearing:   bearing,
        SpeedKmh:  speed,
        EmittedAt: time.Now().UTC().Format(time.RFC3339),
    }
    body, _ := json.Marshal(payload) // struct with no nil fields; marshal cannot fail
    resp, err := client.Post(url+"/location", "application/json", bytes.NewReader(body))
    if err != nil {
        log.Printf("[%s] emit error: %v", driverID, err)
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        log.Printf("[%s] emit got HTTP %d", driverID, resp.StatusCode)
    }
}
```

### Pattern 7: Dockerfile (FROM scratch, multi-stage)

**What:** Identical to location-service Dockerfile; static binary, no OS layer
**When to use:** SIM-04

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o simulator ./cmd/simulator

# Runtime stage
FROM scratch
COPY --from=builder /app/simulator /simulator
ENTRYPOINT ["/simulator"]
```

Note: No `EXPOSE` needed — simulator has no inbound HTTP server.

### Pattern 8: Docker Compose Service Block

**What:** Add `simulator` service to `docker-compose.yml` with correct `depends_on`

```yaml
simulator:
  build:
    context: ./simulator
    dockerfile: Dockerfile
  restart: unless-stopped
  depends_on:
    redis:
      condition: service_healthy
    location-service:
      condition: service_started
  environment:
    REDIS_ADDR: redis:6379
    LOCATION_SERVICE_URL: http://location-service:8080
    DRIVER_COUNT: ${DRIVER_COUNT:-10}
    EMIT_INTERVAL_MS: ${EMIT_INTERVAL_MS:-1000}
  networks:
    - mototaxi
```

Note: `location-service` has no healthcheck (FROM scratch, no shell), so `service_started` is the correct condition.

### Anti-Patterns to Avoid

- **One goroutine per tick instead of per driver:** Do not use a single goroutine with a loop over all drivers. Each driver needs independent position state and timing.
- **Sleeping instead of ticking:** `time.Sleep(interval)` accumulates drift. `time.NewTicker` self-corrects.
- **Reading response body without closing:** Every `resp.Body` must be closed even on non-200 to allow TCP connection reuse in the shared `http.Client`.
- **Creating a new `http.Client` per goroutine:** This defeats connection pooling. One shared client is goroutine-safe.
- **Using `int` env var without validation:** `strconv.Atoi` on empty string returns 0 which would launch 0 goroutines silently. Always fall back to the string default before conversion.
- **Setting TTL on assignment keys:** Assignment keys must persist for the stack lifetime — no TTL.
- **Logging every successful emit:** At 10 drivers × 1/sec that's 10 log lines/sec of noise. Log errors only.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Goroutine-safe HTTP connection pooling | Custom connection pool | `net/http.Client` (shared instance) | stdlib Client already does connection pooling, keep-alive, and is goroutine-safe |
| Redis connectivity with retry | Custom reconnect logic | `pingWithRetry()` using `rdb.Ping()` | go-redis handles reconnection internally; ping loop just gates startup |
| JSON serialization | Manual string building | `encoding/json.Marshal` | Struct tags handle all quoting, escaping, and field naming |
| Ticker drift correction | `time.Sleep` accumulation | `time.NewTicker` | Ticker uses wall clock — no drift accumulation |

**Key insight:** This service has no novel infrastructure needs. It is almost entirely standard library code plus go-redis for the seeding step.

---

## Common Pitfalls

### Pitfall 1: Integer Env Var Default Before Conversion

**What goes wrong:** `envOr("DRIVER_COUNT", "10")` returns `"10"` but if env is set to empty string, `Atoi("")` returns `(0, err)`. Zero goroutines launch silently.

**Why it happens:** `os.Getenv` returns `""` for both "unset" and "set to empty". The `envOr` helper already handles this correctly (`if v != "" { return v }`) so the default kicks in. But if someone writes `strconv.Atoi(os.Getenv("DRIVER_COUNT"))` directly without the fallback, they get 0 silently.

**How to avoid:** Always use the `envOr()` pattern before `Atoi`. Optionally add a guard: if `driverCount <= 0 { log.Fatalf(...) }`.

### Pitfall 2: Response Body Leak

**What goes wrong:** After `client.Post(...)`, if the caller reads status but does not drain and close `resp.Body`, the underlying TCP connection is not returned to the pool. Under 10+ goroutines at 1/sec this causes connection exhaustion within minutes.

**Why it happens:** Go's `http.Client` docs explicitly require the caller to close the response body.

**How to avoid:** Always `defer resp.Body.Close()`. Also discard unread body bytes with `io.Discard` if not reading: `io.Copy(io.Discard, resp.Body)` before close (ensures TCP reuse even on non-200).

### Pitfall 3: Bbox Escape Due to Floating Point Step Overshoot

**What goes wrong:** `StepToward` computes a fractional position. Floating point arithmetic can produce a value microscopically outside `[latMin, latMax]` even when the calculation should land exactly on the boundary.

**Why it happens:** IEEE 754 rounding.

**How to avoid:** Apply `clamp()` unconditionally on every returned point, including the destination itself. The bbox is small enough that clamping never causes visible jumping.

### Pitfall 4: Goroutine Leak on Context Cancel

**What goes wrong:** `emitter.RunDriver` goroutine doesn't check `ctx.Done()` in the select, so when the context is cancelled (SIGINT), goroutines continue running and the process doesn't exit cleanly.

**Why it happens:** `ticker.C` always fires; without a `ctx.Done()` arm in the select, the goroutine ignores cancellation.

**How to avoid:** Always have both `<-ctx.Done()` and `<-ticker.C` arms in the select inside `RunDriver`. Combined with `wg.Wait()` in main, this ensures clean shutdown.

### Pitfall 5: location-service Not Ready When Simulator Starts

**What goes wrong:** `depends_on: location-service: condition: service_started` doesn't wait for location-service to be accepting requests — only that the container process started. First few emit attempts get connection refused and are logged as errors.

**Why it happens:** `FROM scratch` image has no shell so no custom healthcheck is easy to add without a TCP probe binary.

**How to avoid:** The decided error-tolerance policy (log and skip on error) handles this cleanly — early-tick failures are transient and self-resolve. No additional mechanism needed. The ping-retry loop gates Redis only; HTTP failures on early ticks are expected and acceptable.

### Pitfall 6: MSet Argument Ordering

**What goes wrong:** `redis.MSet` takes variadic `interface{}` as alternating key-value pairs. Appending in wrong order produces keys pointing at wrong values.

**Why it happens:** The interface is `MSet(ctx, key1, val1, key2, val2, ...)` — easy to accidentally swap key and value when building the slice in a loop.

**How to avoid:** Build pairs explicitly: append key then value in the same statement. Unit test with miniredis verifying both forward and reverse key values.

---

## Code Examples

Verified patterns from existing project codebase:

### envOr helper (from location-service/cmd/location-service/main.go)

```go
func envOr(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}
```

### Integer env var (new for simulator)

```go
func mustInt(s string) int {
    v, err := strconv.Atoi(s)
    if err != nil || v <= 0 {
        log.Fatalf("invalid integer config value %q: %v", s, err)
    }
    return v
}
// Usage:
driverCount := mustInt(envOr("DRIVER_COUNT", "10"))
```

### go-redis Client init (from location-service/internal/redisstore/store.go)

```go
rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
```

### go-redis MSet for assignment seeding

```go
// go-redis/v9 MSet with variadic interface{} pairs
err := rdb.MSet(ctx, "key1", "val1", "key2", "val2").Err()
```

### Emit payload schema (must match location-service/internal/handler/location.go exactly)

The location-service validates these fields with pointer types to distinguish zero from absent. The simulator always provides all fields, so plain value types are correct on the sender side:

```go
type locationPayload struct {
    DriverID  string  `json:"driver_id"`
    Lat       float64 `json:"lat"`
    Lng       float64 `json:"lng"`
    Bearing   float64 `json:"bearing"`
    SpeedKmh  float64 `json:"speed_kmh"`
    EmittedAt string  `json:"emitted_at"`  // RFC3339
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate `go.mod` per-service was unusual | Standard in Go mono-repos with distinct binaries | Go modules since 2019 | Each service builds independently; Phase 3 follows this established pattern |
| `http.Client` with no timeout | Always set explicit `Timeout` on `http.Client` | Best practice since Go 1.x | Without timeout, a hung location-service causes goroutine to block indefinitely |

**Deprecated/outdated:**
- `ioutil.ReadAll`: Use `io.ReadAll` (Go 1.16+). The project uses Go 1.24 — use `io.ReadAll` and `io.Discard`.

---

## Open Questions

1. **HTTP client timeout value**
   - What we know: Claude's discretion — no hard requirement
   - What's unclear: Whether 5s is too long (blocks goroutine) or too short (São Paulo Docker network is fast)
   - Recommendation: Use 5s. Docker internal network is sub-millisecond; 5s is generous enough to absorb momentary location-service GC pauses without blocking goroutines long enough to cause visible emit-cadence drift.

2. **Ping retry count and interval**
   - What we know: Claude's discretion
   - Recommendation: 10 attempts × 2s = 20s max wait. Matches the Redis healthcheck `retries: 10` in docker-compose. Beyond that, the problem is systemic and operator intervention is needed.

3. **Speed per leg: constant or re-randomized each tick**
   - What we know: Claude's discretion; requirement says 20–60 km/h range
   - Recommendation: Constant per leg (randomized once per destination). This produces smoother, more realistic movement (real vehicles don't randomly change speed every second). Re-randomize speed when a new destination is picked.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no external framework) |
| Config file | none — `go test ./...` from `simulator/` directory |
| Quick run command | `cd simulator && go test ./internal/...` |
| Full suite command | `cd simulator && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SIM-01 | `SeedAssignments` writes correct forward + reverse keys with correct values | unit | `cd simulator && go test ./internal/seeder/... -run TestSeedAssignments -v` | Wave 0 |
| SIM-01 | Seeding is idempotent (re-run does not error, values unchanged) | unit | `cd simulator && go test ./internal/seeder/... -run TestSeedIdempotent -v` | Wave 0 |
| SIM-02 | `Bearing()` returns correct degree for known point pairs | unit | `cd simulator && go test ./internal/geo/... -run TestBearing -v` | Wave 0 |
| SIM-02 | `StepToward()` moves correct fractional distance per tick | unit | `cd simulator && go test ./internal/geo/... -run TestStepToward -v` | Wave 0 |
| SIM-02 | `StepToward()` never returns a point outside the São Paulo bbox | unit | `cd simulator && go test ./internal/geo/... -run TestBboxClamp -v` | Wave 0 |
| SIM-02 | `Arrived()` returns true when within arrival threshold | unit | `cd simulator && go test ./internal/geo/... -run TestArrived -v` | Wave 0 |
| SIM-03 | `emitLocation()` sends correct JSON body matching locationPayload schema | unit | `cd simulator && go test ./internal/emitter/... -run TestEmitPayload -v` | Wave 0 |
| SIM-03 | `emitLocation()` logs error on non-200 and does not panic | unit | `cd simulator && go test ./internal/emitter/... -run TestEmitNon200 -v` | Wave 0 |
| SIM-04 | Docker image builds without error | smoke | `docker build -t simulator-test ./simulator` | Wave 0 |
| SIM-05 | `mustInt(envOr(...))` returns default when env unset; fatals on zero/negative | unit | `cd simulator && go test ./... -run TestEnvConfig -v` | Wave 0 |

### Sampling Rate

- **Per task commit:** `cd simulator && go test ./internal/...`
- **Per wave merge:** `cd simulator && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `simulator/internal/seeder/seeder_test.go` — covers SIM-01 (uses miniredis)
- [ ] `simulator/internal/geo/geo_test.go` — covers SIM-02 (pure math, no external deps)
- [ ] `simulator/internal/emitter/emitter_test.go` — covers SIM-03 (uses `httptest.NewServer`)
- [ ] `simulator/go.mod` — module scaffold with go-redis/v9 and miniredis/v2
- [ ] `simulator/go.sum` — generated by `go mod tidy`

---

## Sources

### Primary (HIGH confidence)

- Existing project code: `location-service/cmd/location-service/main.go` — envOr, startup ping, fatal patterns
- Existing project code: `location-service/internal/redisstore/store.go` — go-redis/v9 client init, Ping, pipeline
- Existing project code: `location-service/internal/handler/location.go` — locationPayload schema (exact JSON field names and types)
- Existing project code: `location-service/Dockerfile` — FROM scratch multi-stage pattern verbatim
- Existing project code: `docker-compose.yml` — service block structure, network name, depends_on patterns
- Existing project code: `.env.example` — DRIVER_COUNT, EMIT_INTERVAL_MS already documented
- Go stdlib documentation: `time.NewTicker`, `net/http.Client`, `encoding/json`, `math`, `sync.WaitGroup`, `context` — all verified standard library

### Secondary (MEDIUM confidence)

- Haversine formula: Standard geographic math, widely documented. The formulas above are the canonical atan2-based implementation used universally. Verified against multiple sources.
- go-redis/v9 `MSet` variadic interface{} signature: Consistent with go-redis v9 docs and usage patterns in location-service codebase.

### Tertiary (LOW confidence)

- None — all claims supported by primary or secondary sources.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use in the project; no new dependencies except miniredis (test-only, already in location-service go.mod)
- Architecture: HIGH — patterns derived directly from existing location-service code; no speculation
- Pitfalls: HIGH — identified from Go stdlib documentation and project code review; haversine clamp pitfall from IEEE 754 properties
- Geographic math: HIGH — haversine formulas are standard; bbox values from REQUIREMENTS.md

**Research date:** 2026-03-06
**Valid until:** 2026-06-06 (stable Go stdlib; go-redis v9 is stable; no fast-moving dependencies)

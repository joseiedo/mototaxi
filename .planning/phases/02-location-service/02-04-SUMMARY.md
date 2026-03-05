---
phase: 02-location-service
plan: 04
subsystem: api
tags: [go, docker, kafka, redis, prometheus, chi]

# Dependency graph
requires:
  - phase: 02-location-service
    provides: kafka Producer interface (Plan 01), Redis Store interface (Plan 02), Prometheus Metrics (Plan 03), HTTP handler (Plans 01-02)
provides:
  - "location-service/cmd/location-service/main.go: entry point wiring all packages"
  - "location-service/Dockerfile: FROM scratch static binary multi-stage build"
  - "docker-compose.yml: location-service block with health-gated depends_on"
  - "Updated handler.NewHandler to accept *metrics.Metrics with instrumentation"
affects: [03-push-server, 05-nginx-gateway, 07-observability]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "FROM scratch Docker image for minimal attack surface and tiny binary size"
    - "Startup ping pattern: fail fast before accepting HTTP traffic"
    - "Metrics instrumentation inline in handler: Inc() after validation, Observe() around I/O calls"

key-files:
  created:
    - location-service/cmd/location-service/main.go
    - location-service/Dockerfile
  modified:
    - location-service/internal/handler/location.go
    - location-service/internal/handler/location_test.go
    - docker-compose.yml

key-decisions:
  - "HandleHealth added to handler (was missing from earlier plans) to satisfy /health route requirement"
  - "metrics.UpdatesReceived.Inc() called after validation (not after full success) to count inbound attempts"
  - "No CA certs in FROM scratch image: Redis and Redpanda are plain TCP on internal Docker network"

patterns-established:
  - "main.go pattern: env → deps → pings → handler → routes → serve"
  - "Docker: multi-stage golang:alpine builder + FROM scratch runtime"

requirements-completed: [LSVC-05]

# Metrics
duration: 2min
completed: 2026-03-05
---

# Phase 2 Plan 4: Wire main.go + Dockerfile + docker-compose Integration Summary

**main.go wires kafka Producer, Redis Store, and Prometheus Metrics into chi HTTP server; Dockerfile builds a FROM scratch static binary; docker-compose adds location-service with health-gated depends_on**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-05T00:00:05Z
- **Completed:** 2026-03-05T00:01:51Z
- **Tasks:** 1 of 2 (Task 2 is checkpoint:human-verify — awaiting smoke test)
- **Files modified:** 5

## Accomplishments
- Created `cmd/location-service/main.go` wiring all four packages (kafka, redisstore, metrics, handler) with startup pings and chi router
- Updated `handler.NewHandler` to accept `*metrics.Metrics`; added `HandleHealth`; instrumented `HandlePostLocation` with `UpdatesReceived.Inc()`, `KafkaDuration.Observe()`, `RedisDuration.Observe()`
- Created `Dockerfile` with golang:1.24-alpine builder stage and FROM scratch runtime — produces static binary with no OS layer
- Added `location-service` block to `docker-compose.yml` with `depends_on: redis (service_healthy)` and `redpanda-init (service_completed_successfully)`

## Task Commits

Each task was committed atomically:

1. **Task 1: Write main.go, Dockerfile, and docker-compose service block** - `d43786e` (feat)
2. **Task 2: Smoke test — full stack** - awaiting human verification

**Plan metadata:** TBD (docs: complete plan)

## Files Created/Modified
- `location-service/cmd/location-service/main.go` - Entry point: env vars, deps init, startup pings, chi router with 4 routes
- `location-service/Dockerfile` - Multi-stage build: golang:1.24-alpine builder + FROM scratch final stage
- `location-service/internal/handler/location.go` - Updated NewHandler signature, added HandleHealth, metrics instrumentation
- `location-service/internal/handler/location_test.go` - Updated newTestRouter to pass metrics.NewMetrics() instance
- `docker-compose.yml` - Added location-service service block before prometheus

## Decisions Made
- `HandleHealth` added to handler (was missing from earlier plans): plan specified GET /health route in main.go, handler needed the method
- `metrics.UpdatesReceived.Inc()` called after validation passes (before I/O) to count inbound valid attempts, not only successes
- No CA certificates copied into FROM scratch image: all dependencies (Redis, Redpanda) are plain TCP on Docker internal network, no TLS

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added HandleHealth method to handler**
- **Found during:** Task 1 (writing main.go)
- **Issue:** main.go registers `r.Get("/health", h.HandleHealth)` but prior handler had no `HandleHealth` method
- **Fix:** Added `HandleHealth(w, r)` that calls `w.WriteHeader(http.StatusOK)` — exactly as specified in plan action section
- **Files modified:** location-service/internal/handler/location.go
- **Verification:** `go build ./...` passes; handler test file updated and tests pass
- **Committed in:** d43786e (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical method)
**Impact on plan:** Required for /health route — plan specified it in action section. No scope creep.

## Issues Encountered
None — `go build ./...` and `go test ./... -race -count=1` both passed on first attempt.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Full location-service Docker image and compose integration complete
- Awaiting human smoke test verification (Task 2 checkpoint)
- After verification: Phase 02 is complete; Phase 03 (push server) can begin

---
*Phase: 02-location-service*
*Completed: 2026-03-05*

---
phase: 02-location-service
plan: "02"
subsystem: location-service
tags: [go, redis, go-redis, miniredis, chi, tdd, pipeline, geoadd]

# Dependency graph
requires:
  - phase: 02-01
    provides: kafka-producer-interface, post-location-handler, go-module
provides:
  - redis-store-interface
  - redisstore-write-location-pipeline
  - redisstore-read-location
  - get-location-handler
  - handler-redis-injection
affects: [02-03, 02-04, 02-05, 04-push-server]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - redis-pipeline-no-txpipeline (GeoAdd + Set sent in single round-trip without MULTI/EXEC overhead)
    - store-interface-for-mockability (redisstore.Store interface enables mock in handler tests)
    - sentinel-error-for-miss (ErrNotFound sentinel distinguishes 404 from 503 in handler)
    - constructor-injection-layered (handler receives both kafka.Producer and redisstore.Store)

key-files:
  created:
    - location-service/internal/redisstore/store.go
    - location-service/internal/redisstore/store_test.go
  modified:
    - location-service/internal/handler/location.go
    - location-service/internal/handler/location_test.go

key-decisions:
  - "Pipeline (not TxPipeline) for GeoAdd+Set: no atomicity benefit from MULTI/EXEC, less overhead"
  - "ErrNotFound sentinel in redisstore package: handler distinguishes 404 from 503 without string matching"
  - "mockStore in handler_test.go: keeps handler tests free of real Redis; miniredis covers real behavior in store_test.go"
  - "Redis key construction (driver:{id}:latest, drivers:geo) lives in redisstore package, not handler — cohesion rule"

patterns-established:
  - "Store interface defined in redisstore package — same pattern as kafka.Producer in kafka package"
  - "Sentinel errors as package-level vars for type-safe comparison across package boundaries"

requirements-completed: [LSVC-02, LSVC-03]

# Metrics
duration: 4min
completed: "2026-03-05"
---

# Phase 2 Plan 2: Redis Store and GET /location/{driverID} Summary

**go-redis v9 Pipeline store (GeoAdd + SET with 30s TTL) and chi GET handler with ErrNotFound sentinel, backed by 6 miniredis tests and 4 new handler tests.**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-05T23:44:00Z
- **Completed:** 2026-03-05T23:48:26Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Implemented `redisstore.Store` interface with `WriteLocation` (Pipeline: GeoAdd + Set 30s TTL), `ReadLocation` (GET with ErrNotFound sentinel), and `Ping`
- 6 miniredis-backed tests: write, TTL range (28-30s), read hit, read miss, redis-down error, pipeline verification
- Extended `Handler` to accept `redisstore.Store` via constructor injection; `HandlePostLocation` now calls `WriteLocation` after Kafka publish
- Added `HandleGetLocation` for `GET /location/{driverID}`: 200+JSON on hit, 404 on miss, 503 on redis error
- All 15 handler tests pass (11 existing + 4 new); full suite passes with `-race`

## Task Commits

Each task was committed atomically:

1. **Task 1: Redis store with miniredis tests** - `763e67d` (feat)
2. **Task 2: Handler extension with Redis injection and GET endpoint** - `87a85c5` (feat)

_Note: TDD tasks — RED phase caused compile/test failures confirmed before GREEN implementation._

## Files Created/Modified

- `location-service/internal/redisstore/store.go` - Store interface, redisStore implementation, ErrNotFound sentinel
- `location-service/internal/redisstore/store_test.go` - 6 miniredis-backed tests covering write, TTL, read hit/miss/down, pipeline
- `location-service/internal/handler/location.go` - Handler struct updated with redis field; NewHandler accepts Store; HandleGetLocation added
- `location-service/internal/handler/location_test.go` - mockStore added; all POST tests updated; 4 new GET tests added

## Decisions Made

1. **Pipeline not TxPipeline:** `s.client.Pipeline()` used (not `TxPipeline`) per RESEARCH.md Pattern 3. No atomicity benefit from MULTI/EXEC for GeoAdd+Set; plain pipeline avoids extra overhead.

2. **ErrNotFound sentinel in redisstore:** `var ErrNotFound = errors.New("driver not found")` exported from `redisstore` package. Handler uses `err == redisstore.ErrNotFound` for type-safe 404 vs 503 branching without string matching.

3. **mockStore in handler tests only:** Real Redis behavior is fully covered by miniredis in `store_test.go`. Handler tests use a simple `mockStore` struct — isolation kept clean.

4. **Key construction in redisstore:** `"driver:"+driverID+":latest"` and `"drivers:geo"` are constructed inside `WriteLocation`/`ReadLocation` methods, not in the handler. Follows cohesion rule — data-access logic belongs with the data-access layer.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `miniredis.Miniredis` does not expose `HGetAll` — the test was adjusted to use go-redis `GeoPos` to verify the GeoAdd entry. Behavior unchanged, equivalent verification.

## User Setup Required

None - no external service configuration required. Tests use miniredis (in-memory).

## Next Phase Readiness

- `redisstore.Store` interface is ready for use by Push Server (Phase 4) via the `driver:{id}:latest` key
- Handler now wires both Kafka and Redis; Phase 3 (main.go wiring) can create `redisstore.NewStore(addr)` and pass to `handler.NewHandler`
- No blockers

---
*Phase: 02-location-service*
*Completed: 2026-03-05*

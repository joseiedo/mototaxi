---
phase: 03-driver-simulator
plan: 04
subsystem: simulator
tags: [go, http, emitter, goroutine, context, tdd, wave3]

# Dependency graph
requires:
  - phase: 03-driver-simulator
    plan: 02
    provides: SeedAssignments seeder package
  - phase: 03-driver-simulator
    plan: 03
    provides: geo package with Bearing, StepToward, Arrived, RandomPoint, RandomSpeed
provides:
  - simulator/internal/emitter/emitter.go: emitLocation function, RunDriver goroutine loop, locationPayload type
  - RunDriver: exported entry point for each driver goroutine in main.go
affects:
  - main.go: calls RunDriver per driver goroutine for process lifetime

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Fire-and-forget HTTP emit: log on error/non-200, always drain response body via io.Copy(io.Discard)
    - Context-cancellable ticker loop: select on ctx.Done() and ticker.C
    - São Paulo bbox as package-level var passed explicitly to geo.RandomPoint/RandomSpeed
    - New leg on geo.Arrived: pick new RandomPoint + RandomSpeed for continuous movement

key-files:
  created: []
  modified:
    - simulator/internal/emitter/emitter.go
    - simulator/internal/emitter/emitter_test.go

key-decisions:
  - "emitLocation returns nil on non-200 (fire-and-forget): 503 is logged via log.Printf, not surfaced to caller; matches existing test contract"
  - "saoPauloBbox defined as package-level var in emitter package: RunDriver signature stays clean (no bbox param) while geo package remains bbox-agnostic"
  - "io.Copy(io.Discard, resp.Body) always called before Close to drain TCP pool; prevents connection pool exhaustion under high frequency ticking"

requirements-completed:
  - SIM-03

# Metrics
duration: 2min
completed: 2026-03-06
---

# Phase 3 Plan 04: Driver Emitter and Tick Loop Summary

**emitLocation POSTs RFC3339-timestamped locationPayload JSON to Location Service; RunDriver goroutine ticks at intervalMs, steps toward destination using geo package, exits cleanly on context cancellation — all 3 emitter tests GREEN**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-06T17:30:08Z
- **Completed:** 2026-03-06T17:31:56Z
- **Tasks:** 1 (TDD: RED test + GREEN implementation)
- **Files modified:** 2

## Accomplishments

- Added `TestRunDriverCancels` to emitter_test.go (RED commit): cancelled context must cause goroutine exit within 100ms
- Implemented `emitter.go` with `locationPayload` struct, `emitLocation`, and `RunDriver`
- `emitLocation`: marshals struct to JSON, POSTs with Content-Type application/json, always drains and closes response body, logs on network error or non-200, returns nil
- `RunDriver`: ticker loop, computes bearing and steps position each tick, picks new destination+speed on arrival, exits on ctx.Done()
- All 3 tests GREEN: TestEmitPayload, TestEmitNon200, TestRunDriverCancels
- Full internal suite GREEN: seeder, geo, and emitter packages all pass

## Task Commits

Each task was committed atomically:

1. **RED: Add TestRunDriverCancels** - `e61c76b` (test)
2. **GREEN: Implement emitter.go** - `33177af` (feat)

## Files Created/Modified

- `simulator/internal/emitter/emitter.go` - Full implementation: locationPayload, emitLocation (fire-and-forget HTTP POST), RunDriver (context-cancellable ticker loop)
- `simulator/internal/emitter/emitter_test.go` - Added TestRunDriverCancels; existing TestEmitPayload and TestEmitNon200 now pass GREEN

## Decisions Made

- `emitLocation` returns `nil` (not `error`) on non-200 — consistent with fire-and-forget design and matches the existing test file's `if err != nil` contract. 503 and other non-200 codes are logged but not propagated.
- `saoPauloBbox` is a package-level `var` in the emitter package. This keeps `RunDriver`'s signature clean (`id, client, locationURL, intervalMs`) while still passing explicit bbox/speed-range args to `geo.RandomPoint` and `geo.RandomSpeed` — respecting the geo package's bbox-agnostic design from plan 03-03.
- TCP connection pool safety: `io.Copy(io.Discard, resp.Body)` is always called before `resp.Body.Close()` to fully drain the response, preventing connection pool exhaustion at high tick frequencies.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Signature adaptation] emitLocation uses struct arg, not individual fields**
- **Found during:** RED phase review
- **Issue:** Plan `<behavior>` section shows `emitLocation(client, serverURL, "driver-42", -23.55, -46.65, 90.0, 35.5)` with individual parameters, but the Wave 0 test stub (created in 03-01) already used `emitLocation(client, url, payload locationPayload)` returning `error`
- **Fix:** Kept the existing test's struct-based signature. Both approaches are equivalent; struct is cleaner and more extensible. Test assertions remain identical.
- **Files modified:** emitter.go only (test file already matched this approach)

## Self-Check: PASSED

- `simulator/internal/emitter/emitter.go` exists with full implementation
- `simulator/internal/emitter/emitter_test.go` contains TestRunDriverCancels
- Commit `e61c76b` (RED test) exists in git log
- Commit `33177af` (GREEN implementation) exists in git log
- `go test ./internal/emitter/... -run "TestEmit|TestRunDriver" -v` shows PASS for all 3 tests
- `go test ./internal/... -v` shows all 9 tests across seeder, geo, emitter packages PASS

---
*Phase: 03-driver-simulator*
*Completed: 2026-03-06*

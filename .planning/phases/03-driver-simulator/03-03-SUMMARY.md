---
phase: 03-driver-simulator
plan: 03
subsystem: testing
tags: [go, geo, haversine, math, simulator, tdd, wave2]

# Dependency graph
requires:
  - phase: 03-driver-simulator
    plan: 01
    provides: Wave 0 stub geo_test.go with RED test stubs for Bearing, StepToward, BboxClamp, Arrived
affects:
  - 03-04: emitter implementation calls geo.RandomPoint(bbox), geo.RandomSpeed(min, max), geo.Bearing, geo.StepToward, geo.Arrived

provides:
  - simulator/internal/geo/geo.go: fully implemented geographic math package
  - Bearing (atan2-based, [0, 360) normalized), DistanceKm (haversine), StepToward (linear interpolation + no-overshoot), Arrived (0.01 km threshold), BboxClamp, RandomPoint(bbox), RandomSpeed(min, max)
  - All 6 geo tests GREEN: TestBearing, TestStepToward, TestBboxClamp, TestArrived, TestRandomPoint, TestRandomSpeed

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Haversine distance formula with earthRadiusKm=6371.0 using stdlib math only
    - atan2 bearing normalized via math.Mod(θ+360, 360) to guarantee [0, 360) range
    - Linear interpolation StepToward with stepKm/distKm fraction, returns dst directly on overshoot
    - BboxClamp using math.Max/Min double-clamp applied unconditionally on every returned point
    - Bbox passed as explicit parameter to RandomPoint/RandomSpeed (no package-level global bbox)

key-files:
  created: []
  modified:
    - simulator/internal/geo/geo.go
    - simulator/internal/geo/geo_test.go

key-decisions:
  - "RandomPoint and RandomSpeed take explicit parameters (bbox Bbox, minKmh/maxKmh float64) rather than using package-level São Paulo bbox constants — keeps the geo package reusable and bbox-agnostic"
  - "StepToward returns dst directly (not clamp(dst)) on overshoot — BboxClamp is the caller's responsibility; StepToward itself guarantees no overshoot, clamping is separate concern"
  - "TestRandomPoint and TestRandomSpeed added in RED commit before implementation — Wave 0 stubs only had 4 tests; plan required 6 (TestRandomPoint and TestRandomSpeed were missing)"

patterns-established:
  - "Explicit bbox parameter pattern: geo.RandomPoint(bbox) rather than geo.RandomPoint() with internal constant — matches emitter.go Pattern 4 from RESEARCH.md"
  - "Property tests (100 iterations) for random generators verify probabilistic invariants without seeds"

requirements-completed:
  - SIM-02

# Metrics
duration: 3min
completed: 2026-03-06
---

# Phase 3 Plan 03: Geographic Math Package Summary

**Haversine distance + atan2 bearing geo package with linear StepToward, bbox clamping, arrival detection, and random point/speed generators — all 6 geo tests GREEN**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-06T17:25:25Z
- **Completed:** 2026-03-06T17:27:51Z
- **Tasks:** 1 (TDD: RED tests + GREEN implementation)
- **Files modified:** 2

## Accomplishments

- Added missing TestRandomPoint and TestRandomSpeed to geo_test.go (RED commit)
- Implemented geo.go with Bearing, DistanceKm, StepToward, Arrived, BboxClamp, RandomPoint, RandomSpeed
- All 6 tests pass GREEN: TestBearing (4 cardinal directions ±1°), TestStepToward (fractional movement + no-overshoot), TestBboxClamp, TestArrived (0.01 km threshold), TestRandomPoint (100-iter bbox property), TestRandomSpeed (100-iter [20,60) property)
- Full module `go build ./...` and all other packages remain unaffected

## Task Commits

Each task was committed atomically:

1. **RED: Add TestRandomPoint and TestRandomSpeed** - `ce84f0c` (test)
2. **GREEN: Implement geo package** - `e3592cc` (feat)

## Files Created/Modified

- `simulator/internal/geo/geo.go` - Full geo package: Bearing, DistanceKm, StepToward, Arrived, BboxClamp, RandomPoint, RandomSpeed
- `simulator/internal/geo/geo_test.go` - Added TestRandomPoint (100-iter bbox containment) and TestRandomSpeed (100-iter [20,60) range)

## Decisions Made

- `RandomPoint` and `RandomSpeed` accept explicit `bbox Bbox` and `minKmh, maxKmh float64` parameters respectively, rather than referencing a package-level São Paulo bbox constant. This matches how emitter.go will call them (passing the bbox from config) and keeps the geo package bbox-agnostic for reuse.
- `StepToward` returns `dst` directly (not `clamp(dst)`) when step >= distance — clamping is the caller's responsibility via `BboxClamp`. This separation of concerns avoids silent bbox mutation when the destination itself is already valid.
- Wave 0 test stubs (plan 03-01) had only 4 geo tests (TestBearing, TestStepToward, TestBboxClamp, TestArrived). TestRandomPoint and TestRandomSpeed were specified in this plan's `<behavior>` section — added as the RED commit before implementing.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `simulator/internal/geo` package complete and tested; plan 03-04 (emitter) can call all exported functions
- emitter.RunDriver pattern from RESEARCH.md Pattern 4 uses `geo.RandomPoint()` and `geo.RandomSpeed()` with no arguments — plan 03-04 will need to pass bbox and speed range as explicit arguments to match the implemented signatures

---
*Phase: 03-driver-simulator*
*Completed: 2026-03-06*

---
phase: 01-infrastructure
plan: 02
subsystem: infra
tags: [docker-compose, k6, stress-testing, prometheus, grafana]

# Dependency graph
requires:
  - phase: 01-infrastructure/01-01
    provides: docker-compose.yml with mototaxi network and 9 services
provides:
  - docker-compose.stress.yml k6 overlay referencing mototaxi network as external
  - stress/drivers.js placeholder (driver location load test, Phase 8 full impl)
  - stress/customers.js placeholder (customer WebSocket load test, Phase 8 full impl)
  - stress/latency.js placeholder (end-to-end latency test, Phase 8 full impl)
affects:
  - 08-load-testing (implements full k6 scripts and thresholds)

# Tech tracking
tech-stack:
  added:
    - grafana/k6:latest (load testing tool with Prometheus remote write support)
  patterns:
    - Compose overlay pattern: docker-compose.stress.yml adds only the k6 service; base services inherited from docker-compose.yml; mototaxi network declared external: true
    - Placeholder script pattern: minimal valid k6 scripts in stress/ ensure volume mount is non-empty; full implementation deferred to Phase 8

key-files:
  created:
    - docker-compose.stress.yml
    - stress/drivers.js
    - stress/customers.js
    - stress/latency.js
  modified:
    - .gitignore

key-decisions:
  - "mototaxi network declared external: true in stress overlay because docker-compose.yml owns network creation; overlay must reference it as external"
  - "K6_PROMETHEUS_RW_SERVER_URL points to prometheus:9090/api/v1/write using internal Docker network hostname"

patterns-established:
  - "Compose overlay pattern: use separate overlay file for optional services (k6) that share the base network"

requirements-completed: [INFRA-04]

# Metrics
duration: 3min
completed: 2026-03-05
---

# Phase 1 Plan 02: k6 Stress Overlay Summary

**grafana/k6 stress overlay added as docker-compose.stress.yml with K6_PROMETHEUS_RW_SERVER_URL targeting prometheus:9090/api/v1/write and three placeholder scripts in stress/ for Phase 8 load testing**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-05T21:43:01Z
- **Completed:** 2026-03-05T21:46:00Z
- **Tasks:** 1 of 2 (Task 2 is checkpoint:human-verify)
- **Files modified:** 5

## Accomplishments

- docker-compose.stress.yml with grafana/k6 service, Prometheus remote write env var, and external mototaxi network reference
- Three placeholder k6 scripts in stress/ (drivers.js, customers.js, latency.js) that are minimal valid k6 programs
- `docker compose -f docker-compose.yml -f docker-compose.stress.yml config --quiet` exits 0 (verified)
- .gitignore updated with /data/ docker volumes entry

## Task Commits

Each task was committed atomically:

1. **Task 1: Create stress overlay and k6 script placeholders** - `3847b7a` (feat)

**Plan metadata:** (docs commit below)

## Files Created/Modified

- `docker-compose.stress.yml` - k6 overlay: grafana/k6 image, K6_PROMETHEUS_RW_SERVER_URL=http://prometheus:9090/api/v1/write, ./stress:/scripts volume, mototaxi network external: true
- `stress/drivers.js` - Placeholder driver location load test; full 0→2000 VU ramp in Phase 8
- `stress/customers.js` - Placeholder customer WebSocket load test; full 0→10000 connection ramp in Phase 8
- `stress/latency.js` - Placeholder end-to-end latency test; p50/p95/p99 measurement in Phase 8
- `.gitignore` - Added /data/ docker volumes entry

## Decisions Made

- `mototaxi` network declared `external: true` in the overlay because docker-compose.yml owns network creation; the overlay cannot redefine it.
- K6_PROMETHEUS_RW_PROMETHEUS_RW_SERVER_URL uses the internal Docker hostname `prometheus:9090` (not localhost) so k6 and Prometheus communicate over the shared Docker network.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Stress overlay ready: `docker compose -f docker-compose.yml -f docker-compose.stress.yml up k6` works once the stack is running
- Full k6 scripts (with ramp shapes, thresholds, WebSocket logic) deferred to Phase 8
- Human checkpoint (Task 2) pending: user needs to start the full stack and verify all UIs are accessible

## Self-Check: PASSED

- `docker-compose.stress.yml`: FOUND
- `stress/drivers.js`: FOUND
- `stress/customers.js`: FOUND
- `stress/latency.js`: FOUND
- `.gitignore` updated: VERIFIED
- Commit `3847b7a`: FOUND
- `docker compose -f docker-compose.yml -f docker-compose.stress.yml config --quiet`: Exit 0

---
*Phase: 01-infrastructure*
*Completed: 2026-03-05*

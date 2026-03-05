---
phase: 01-infrastructure
plan: 02
subsystem: infra
tags: [docker-compose, k6, stress-testing, redpanda, prometheus, grafana]

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
  - 07-observability (k6 writes metrics to Prometheus; dashboard added in Phase 7)

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
  - "rpk v25.x compatibility: --brokers flag removed in v25.x CLI; updated redpanda-init.sh to use --kafka-addr (commit 2db9314)"

patterns-established:
  - "Compose overlay pattern: use separate overlay file for optional services (k6) that share the base network"

requirements-completed: [INFRA-04]

# Metrics
duration: ~35min (including stack startup, image pulls, rpk v25.x fix, and human verification)
completed: 2026-03-05
---

# Phase 1 Plan 02: k6 Stress Overlay Summary

**grafana/k6 stress overlay added as docker-compose.stress.yml with K6_PROMETHEUS_RW_SERVER_URL targeting prometheus:9090/api/v1/write and three placeholder scripts in stress/ for Phase 8 load testing; full stack smoke-tested with all 8 services healthy, driver.location topic verified at 4 partitions, and all UIs confirmed accessible**

## Performance

- **Duration:** ~35 min (including stack startup, image pulls, rpk v25.x fix, and human verification)
- **Started:** 2026-03-05T21:43:01Z
- **Completed:** 2026-03-05
- **Tasks:** 2 of 2
- **Files modified:** 5

## Accomplishments

- docker-compose.stress.yml with grafana/k6 service, Prometheus remote write env var, and external mototaxi network reference
- Three placeholder k6 scripts in stress/ (drivers.js, customers.js, latency.js) that are minimal valid k6 programs
- `docker compose -f docker-compose.yml -f docker-compose.stress.yml config --quiet` exits 0 (verified)
- .gitignore updated with /data/ docker volumes entry
- rpk v25.x compatibility fix: `--brokers` flag replaced with `--kafka-addr` in redpanda-init.sh
- Full stack smoke test passed: all 8 services reached running or exited(0) state; Prometheus (:9090), Grafana (:3000), Redpanda Console (:8080) all accessible; driver.location topic verified with 4 partitions

## Task Commits

Each task was committed atomically:

1. **Task 1: Create stress overlay and k6 script placeholders** - `3847b7a` (feat)
2. **Deviation: rpk v25.x compatibility fix** - `2db9314` (fix)
3. **Task 2: Human-verify checkpoint** - APPROVED (no code commit; human verification only)

**Plan metadata:** `b44dcdc` (docs: complete k6 stress overlay plan)

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

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed rpk CLI flag incompatibility with redpanda v25.x**
- **Found during:** Stack smoke test (post-Task 1, during checkpoint verification)
- **Issue:** `infra/redpanda-init.sh` used `--brokers` flag which was removed in rpk v25.x; redpanda-init container exited non-zero, preventing driver.location topic creation
- **Fix:** Updated rpk commands in `infra/redpanda-init.sh` to use `--kafka-addr` flag compatible with v25.x
- **Files modified:** `infra/redpanda-init.sh`
- **Verification:** redpanda-init container exited 0; `rpk topic describe driver.location` confirmed 4 partitions
- **Committed in:** `2db9314`

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug)
**Impact on plan:** Fix was essential — init container failure blocked topic creation and all downstream services. rpk v25.x is a breaking CLI change affecting flag names. No scope creep.

## Issues Encountered

- redpanda-init container exited non-zero initially due to rpk v25.x `--brokers` flag removal. Diagnosed via `docker compose logs redpanda-init`, fixed in `infra/redpanda-init.sh`, verified with topic describe showing 4 partitions.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 1 infrastructure complete and human-verified: all 9 services start cleanly, driver.location topic exists at 4 partitions, all UIs accessible
- Stress overlay ready: `docker compose -f docker-compose.yml -f docker-compose.stress.yml run k6 run /scripts/drivers.js -o experimental-prometheus-rw` will work once Phase 8 populates real scripts
- Full k6 scripts (with ramp shapes, thresholds, WebSocket logic) deferred to Phase 8
- Phase 2 (Location Service): add location-service block to docker-compose.yml; location-service/ directory skeleton already exists
- No blockers.

## Self-Check: PASSED

- `docker-compose.stress.yml`: FOUND
- `stress/drivers.js`: FOUND
- `stress/customers.js`: FOUND
- `stress/latency.js`: FOUND
- `.gitignore` updated: VERIFIED
- Commit `3847b7a` (feat: stress overlay and placeholders): FOUND
- Commit `2db9314` (fix: rpk v25.x compatibility): FOUND
- Commit `b44dcdc` (docs: plan metadata): FOUND
- Human checkpoint Task 2: APPROVED by user (all 8 services healthy, driver.location 4 partitions, all UIs accessible)

---
*Phase: 01-infrastructure*
*Completed: 2026-03-05*

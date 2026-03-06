---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: planning
stopped_at: Completed 03-driver-simulator-03-03-PLAN.md
last_updated: "2026-03-06T17:28:49.645Z"
last_activity: 2026-03-05 — Roadmap created, all 34 v1 requirements mapped to 8 phases
progress:
  total_phases: 8
  completed_phases: 2
  total_plans: 11
  completed_plans: 9
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-05)

**Core value:** Prove that the multi-service architecture holds under load — every design decision demonstrable through Grafana metrics and reproducible experiments.
**Current focus:** Phase 1 - Infrastructure

## Current Position

Phase: 1 of 8 (Infrastructure)
Plan: 0 of TBD in current phase
Status: Ready to plan
Last activity: 2026-03-05 — Roadmap created, all 34 v1 requirements mapped to 8 phases

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: -
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**
- Last 5 plans: -
- Trend: -

*Updated after each plan completion*
| Phase 01-infrastructure P01 | 2 | 2 tasks | 11 files |
| Phase 01-infrastructure P02 | 3 | 1 tasks | 5 files |
| Phase 01-infrastructure P02 | 35 | 2 tasks | 5 files |
| Phase 02-location-service P01 | 205 | 2 tasks | 5 files |
| Phase 02-location-service P02 | 4 | 2 tasks | 4 files |
| Phase 02-location-service P03 | 5 | 1 tasks | 2 files |
| Phase 02-location-service P04 | 2 | 1 tasks | 5 files |
| Phase 03-driver-simulator P01 | 3 | 1 tasks | 10 files |
| Phase 03-driver-simulator P02 | 2 | 1 tasks | 1 files |
| Phase 03-driver-simulator P03 | 3 | 2 tasks | 2 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Go for Location Service + Simulator: goroutines cheapest for N concurrent HTTP emitters
- Elixir/Phoenix for Push Server: BEAM processes for 10k+ stateful long-lived connections
- Redpanda: Kafka-compatible, simpler single-binary deployment for local dev
- Phoenix.PubSub + Redis adapter: cross-replica fan-out without manual connection registry
- Partition count = replicas × multiplier: independent tuning of intra-replica Broadway parallelism
- [Phase 01-infrastructure]: Nginx volume mount deferred to Phase 5: mounting empty conf.d causes nginx startup failure
- [Phase 01-infrastructure]: Partition arithmetic computed in shell script (not compose YAML) because Docker Compose does not support arithmetic interpolation
- [Phase 01-infrastructure]: kafka-exporter and cadvisor use platform: linux/amd64 for macOS M-series Rosetta 2 compatibility
- [Phase 01-infrastructure]: mototaxi network declared external: true in stress overlay because docker-compose.yml owns network creation
- [Phase 01-infrastructure]: K6_PROMETHEUS_RW_SERVER_URL uses internal Docker hostname prometheus:9090 for intra-network Prometheus remote write
- [Phase 01-infrastructure]: mototaxi network declared external: true in stress overlay because docker-compose.yml owns network creation
- [Phase 01-infrastructure]: rpk v25.x compatibility: --brokers flag removed; updated redpanda-init.sh to use --kafka-addr
- [Phase 02-location-service]: Used *float64 pointer fields for lat/lng/bearing/speed_kmh to correctly handle equatorial zero-value coordinates
- [Phase 02-location-service]: Producer interface in kafka package enables mockProducer in handler tests without live Kafka dependency
- [Phase 02-location-service]: emitted_at validated as RFC3339 via time.Parse to catch malformed timestamps at ingest boundary
- [Phase 02-location-service]: Pipeline not TxPipeline for GeoAdd+Set: no atomicity benefit from MULTI/EXEC, less overhead
- [Phase 02-location-service]: ErrNotFound sentinel in redisstore package enables type-safe 404 vs 503 branching in handler
- [Phase 02-location-service]: Custom prometheus.NewRegistry() per Metrics instance prevents already-registered panics and enables safe dependency injection
- [Phase 02-location-service]: FROM scratch Docker image for location-service: static binary with no OS layer, no shell, ~5MB
- [Phase 02-location-service]: No CA certs in FROM scratch image: Redis and Redpanda are plain TCP on Docker internal network
- [Phase 03-driver-simulator]: emitter_test.go uses package emitter (white-box) to access unexported emitLocation and locationPayload
- [Phase 03-driver-simulator]: Wave 0 TDD stubs: stub source files return errors.New(not implemented) so tests compile and fail RED
- [Phase 03-driver-simulator]: Single MSet call chosen over per-key Set loop: one Redis round-trip for all 2*n keys regardless of driver count
- [Phase 03-driver-simulator]: n<=0 guard returns nil without Redis call; idempotency achieved naturally by MSet overwrite semantics
- [Phase 03-driver-simulator]: RandomPoint and RandomSpeed take explicit parameters (bbox, min/max) rather than package-level São Paulo constants — keeps geo package bbox-agnostic
- [Phase 03-driver-simulator]: StepToward returns dst directly on overshoot without clamping — clamping is caller responsibility via BboxClamp

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Session Continuity

Last session: 2026-03-06T17:28:49.643Z
Stopped at: Completed 03-driver-simulator-03-03-PLAN.md
Resume file: None

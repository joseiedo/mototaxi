---
phase: 01-infrastructure
plan: 01
subsystem: infra
tags: [docker-compose, redpanda, redis, prometheus, grafana, nginx, kafka-exporter, cadvisor]

# Dependency graph
requires: []
provides:
  - docker-compose.yml with 9 infrastructure services orchestrated via health checks and init container
  - infra/redpanda-init.sh that creates driver.location topic with PUSH_SERVER_REPLICAS x PARTITION_MULTIPLIER partitions
  - .env.example with 6 documented tunables (DRIVER_COUNT, EMIT_INTERVAL_MS, LOCATION_SERVICE_REPLICAS, PUSH_SERVER_REPLICAS, PARTITION_MULTIPLIER, SECRET_KEY_BASE)
  - Grafana auto-provisioning stubs (datasource + dashboard provider)
  - Directory skeleton for all services (location-service, push-server, simulator, nginx, stress)
affects:
  - 01-02-infrastructure (stress overlay builds on this compose file)
  - 02-location-service (adds service block to docker-compose.yml)
  - 03-driver-simulator (adds service block to docker-compose.yml)
  - 04-push-server (adds service block to docker-compose.yml)
  - 05-nginx-routing (populates nginx/ directory, adds volume mount)
  - 07-observability (adds scrape targets to prometheus.yml, adds dashboard files)

# Tech tracking
tech-stack:
  added:
    - redpandadata/redpanda:v25.2.7 (Kafka-compatible message broker)
    - redpandadata/console:v3.3.2 (Redpanda web UI)
    - redis:7-alpine (in-memory store)
    - prom/prometheus:v2.53.0 (metrics collection)
    - grafana/grafana:11.4.0 (metrics visualization)
    - danielqsj/kafka-exporter:latest (Kafka metrics for Prometheus)
    - gcr.io/cadvisor/cadvisor:latest (container resource metrics)
    - nginx:alpine (reverse proxy / load balancer)
  patterns:
    - Init container pattern: ephemeral redpanda-init creates topic then exits 0; downstream services use service_completed_successfully
    - Health-gate pattern: Redis uses service_healthy with redis-cli ping; downstream services wait before starting
    - Env-driven partition formula: PARTITIONS computed in shell (PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER), not in compose YAML
    - Port exposure policy: only 4 host ports (80, 3000, 8080, 9090); all internal services stay on Docker network

key-files:
  created:
    - docker-compose.yml
    - .env.example
    - .gitignore
    - infra/redpanda-init.sh
    - observability/prometheus.yml
    - observability/grafana/provisioning/datasources/prometheus.yaml
    - observability/grafana/provisioning/dashboards/dashboards.yaml
  modified: []

key-decisions:
  - "Nginx volume mount deferred to Phase 5: mounting empty conf.d dir causes nginx to use invalid config; populated in Phase 5 when nginx.conf exists"
  - "kafka-exporter and cadvisor use platform: linux/amd64 for macOS M-series (Rosetta 2 emulation)"
  - "Partition count calculated in shell script, not compose YAML, because Docker Compose does not support arithmetic interpolation"

patterns-established:
  - "Init container pattern: use service_completed_successfully for topic/schema creation before dependent services start"
  - "Health-gate pattern: use service_healthy with explicit healthcheck test for stateful services (Redis)"
  - "Env-driven config: all tunables in .env, referenced via ${VAR:-default} in compose"

requirements-completed: [INFRA-01, INFRA-02, INFRA-03, INFRA-05]

# Metrics
duration: 2min
completed: 2026-03-05
---

# Phase 1 Plan 01: Infrastructure Skeleton Summary

**Docker Compose skeleton with 9 services, Redpanda init container creating driver.location topic via shell arithmetic, Redis health-gated, and Grafana/Prometheus provisioning stubs auto-configured**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-05T21:38:37Z
- **Completed:** 2026-03-05T21:40:32Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments

- docker-compose.yml with 9 services and correct startup ordering via health checks and init container pattern
- infra/redpanda-init.sh with shell arithmetic for partition count (PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER)
- .env.example documenting all 6 tunables with inline comments; .env gitignored
- Grafana auto-provisioning stubs (Prometheus datasource + dashboard file provider)
- Directory skeleton for all future services with .gitkeep files

## Task Commits

Each task was committed atomically:

1. **Task 1: Create directory skeleton and environment files** - `1ee0e7b` (chore)
2. **Task 2: Write docker-compose.yml with all infrastructure services** - `a98a6fe` (feat)

**Plan metadata:** (docs commit below)

## Files Created/Modified

- `docker-compose.yml` - Full infrastructure orchestration: 9 services, startup ordering, health checks, port policy
- `.env.example` - 6 tunables documented with inline comments (DRIVER_COUNT, EMIT_INTERVAL_MS, LOCATION_SERVICE_REPLICAS, PUSH_SERVER_REPLICAS, PARTITION_MULTIPLIER, SECRET_KEY_BASE)
- `.gitignore` - Excludes .env from version control
- `infra/redpanda-init.sh` - Executable shell script; waits for Redpanda health, creates driver.location topic with computed partition count
- `observability/prometheus.yml` - Minimal valid Prometheus config (scrapes itself; scrape targets added in Phase 7)
- `observability/grafana/provisioning/datasources/prometheus.yaml` - Auto-provisions Prometheus as default datasource
- `observability/grafana/provisioning/dashboards/dashboards.yaml` - Points Grafana at /var/lib/grafana/dashboards directory
- `location-service/.gitkeep`, `push-server/.gitkeep`, `simulator/.gitkeep`, `nginx/.gitkeep`, `stress/.gitkeep` - Directory stubs for future phases

## Decisions Made

- **Nginx volume mount deferred:** Mounting empty conf.d directory causes nginx to fail on startup (no valid config). Mount added in Phase 5 when nginx.conf is populated.
- **platform: linux/amd64 on kafka-exporter and cadvisor:** Required for macOS M-series (Rosetta 2 handles emulation transparently).
- **Partition arithmetic in shell, not compose:** Docker Compose YAML interpolation does not support arithmetic; the `$(( expr ))` lives in redpanda-init.sh where bash evaluates it at runtime.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. Run `docker compose up --build -d` to start all services.

## Next Phase Readiness

- docker-compose.yml is ready for Phase 1 Plan 02 (stress overlay adds k6 service)
- All service directories exist; each subsequent phase adds its Dockerfile and application code
- .env tunables are live: changing PUSH_SERVER_REPLICAS and PARTITION_MULTIPLIER changes topic partition count on next `docker compose up`
- No blockers.

---
*Phase: 01-infrastructure*
*Completed: 2026-03-05*

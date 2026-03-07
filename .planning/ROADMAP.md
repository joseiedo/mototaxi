# Roadmap: Live Driver Location Simulation

## Overview

Eight phases taking the system from a bare Docker skeleton to a fully stress-tested, observable multi-service pipeline. Each phase delivers one complete, independently verifiable capability. The sequence follows hard dependency order: infrastructure first, then each service layer, then the routing and presentation layers on top, then observability wiring, then stress testing that proves the whole thing holds under load.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Infrastructure** - Docker Compose skeleton with Redpanda, Redis, and all config externalized (completed 2026-03-05)
- [x] **Phase 2: Location Service** - Go ingest service: validates, publishes to Kafka, writes to Redis, exposes metrics (completed 2026-03-05)
- [x] **Phase 3: Driver Simulator** - Go simulator seeds assignments and emits realistic GPS updates per driver goroutine (completed 2026-03-06)
- [ ] **Phase 4: Push Server** - Elixir/Phoenix service holds WebSocket connections, consumes Kafka via Broadway, fans out via PubSub
- [ ] **Phase 5: Nginx Routing** - Nginx as dual-role load balancer: least_conn for location-service, ip_hash sticky for push-servers
- [ ] **Phase 6: Frontend** - Static HTML + Leaflet.js tracking and overview pages, served by Nginx
- [ ] **Phase 7: Observability** - Prometheus, cAdvisor, kafka-exporter, and all 4 Grafana dashboards auto-provisioned
- [ ] **Phase 8: Stress Tests** - k6 scripts for driver load, WebSocket load, and end-to-end latency, with metrics in Grafana

## Phase Details

### Phase 1: Infrastructure
**Goal**: The full service skeleton starts cleanly with `docker compose up --build` — Redpanda initialized with correct partitions, Redis ready, all config externalized
**Depends on**: Nothing (first phase)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05
**Success Criteria** (what must be TRUE):
  1. Running `docker compose up --build` on a clean machine brings all infrastructure services up without manual intervention
  2. Redpanda `driver.location` topic exists with `PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER` partitions before any dependent service starts
  3. `.env.example` documents every tunable parameter and changing DRIVER_COUNT, EMIT_INTERVAL_MS, etc. in `.env` takes effect on the next `up`
  4. `docker compose -f docker-compose.yml -f docker-compose.stress.yml up k6` launches the k6 service without errors
**Plans**: 2 plans

Plans:
- [ ] 01-01-PLAN.md — docker-compose.yml skeleton, init script, env files, observability stubs
- [ ] 01-02-PLAN.md — stress overlay, k6 placeholder scripts, full stack smoke verification

### Phase 2: Location Service
**Goal**: The Go Location Service ingests driver GPS updates, validates them, persists to Redis, and publishes to Kafka — ready for downstream consumers
**Depends on**: Phase 1
**Requirements**: LSVC-01, LSVC-02, LSVC-03, LSVC-04, LSVC-05
**Success Criteria** (what must be TRUE):
  1. `POST /location` with a valid driver payload returns HTTP 200, and the message appears in the Redpanda `driver.location` topic
  2. After a `POST /location`, `GET /location/{driver_id}` returns the current position from Redis (and the key expires after 30 seconds of inactivity)
  3. `GET /metrics` returns Prometheus-formatted metrics including `location_updates_received_total`, `kafka_publish_duration_ms`, and `redis_write_duration_ms`
  4. The Docker image builds to a static binary `FROM scratch` final stage (~5MB), with no external runtime dependencies
**Plans**: 4 plans

Plans:
- [ ] 02-01-PLAN.md — Go module scaffold, Kafka producer wrapper, POST /location handler with validation (LSVC-01)
- [ ] 02-02-PLAN.md — Redis store (GEOADD+SET pipeline, GET), GET /location/{id} handler (LSVC-02, LSVC-03)
- [ ] 02-03-PLAN.md — Prometheus metrics package (custom registry, 3 custom metrics, Go runtime collectors) (LSVC-04)
- [ ] 02-04-PLAN.md — main.go wiring, Dockerfile FROM scratch, docker-compose.yml service block, human smoke test (LSVC-05)

### Phase 3: Driver Simulator
**Goal**: The Go Driver Simulator seeds customer-driver assignments into Redis at startup and drives N goroutines emitting realistic GPS updates continuously
**Depends on**: Phase 2
**Requirements**: SIM-01, SIM-02, SIM-03, SIM-04, SIM-05
**Success Criteria** (what must be TRUE):
  1. On startup, Redis contains `customer:{id}:driver` and `driver:{id}:customer` keys for every simulated driver before the movement loop begins
  2. Each driver goroutine moves point-to-point within the São Paulo bounding box at 20–60 km/h, picks a new destination on arrival, and never leaves the bounding box
  3. Every driver goroutine posts `POST /location` with bearing, speed_kmh, and emitted_at on the configured EMIT_INTERVAL_MS cadence
  4. Changing DRIVER_COUNT and EMIT_INTERVAL_MS in `.env` and restarting the simulator changes the number of active goroutines and emission frequency accordingly
**Plans**: 5 plans

Plans:
- [ ] 03-01-PLAN.md — Go module scaffold, Wave 0 test stubs for all packages (SIM-05)
- [ ] 03-02-PLAN.md — Redis assignment seeder: SeedAssignments with MSet, miniredis tests (SIM-01)
- [ ] 03-03-PLAN.md — Geographic math: haversine, bearing, StepToward, bbox clamp, arrival detection (SIM-02)
- [ ] 03-04-PLAN.md — Driver emitter: locationPayload, emitLocation, RunDriver tick loop (SIM-03)
- [ ] 03-05-PLAN.md — main.go wiring, Dockerfile FROM scratch, docker-compose integration, smoke test (SIM-04, SIM-05)

### Phase 4: Push Server
**Goal**: The Elixir/Phoenix Push Server holds customer WebSocket connections, resolves assigned drivers, consumes Kafka with backpressure via Broadway, and fans location updates out via Phoenix.PubSub across all replicas
**Depends on**: Phase 1
**Requirements**: PUSH-01, PUSH-02, PUSH-03, PUSH-04, PUSH-05
**Success Criteria** (what must be TRUE):
  1. A WebSocket client joining `customer:{customer_id}` receives an immediate push with the driver's current position from Redis
  2. After joining, the client continues to receive location updates each time the simulator emits a new position for the assigned driver
  3. With two push-server replicas running, a customer connected to replica A receives updates even when the Kafka message is consumed by replica B (Phoenix.PubSub + Redis adapter delivers cross-replica)
  4. `GET /metrics` (or PromEx endpoint) exposes `push_server_connections_active`, `push_server_messages_delivered_total`, and `push_server_delivery_latency_ms`
**Plans**: 5 plans

Plans:
- [ ] 04-01-PLAN.md — Mix project scaffold, all deps, config skeleton, Wave 0 failing test stubs (PUSH-01 through PUSH-05)
- [ ] 04-02-PLAN.md — Phoenix Endpoint, UserSocket, CustomerChannel: join/3 with Redis lookups + handle_info/2 (PUSH-01, PUSH-02)
- [ ] 04-03-PLAN.md — Broadway pipeline: BroadwayKafka producer, handle_message/3 with PubSub broadcast, handle_failed/2 (PUSH-03)
- [ ] 04-04-PLAN.md — PubSub Redis adapter supervision tree wiring + PromEx custom metrics + telemetry emission (PUSH-04, PUSH-05)
- [ ] 04-05-PLAN.md — Dockerfile (elixir:1.18-alpine builder, alpine:3.21 runtime), docker-compose service block, human smoke test (PUSH-01 through PUSH-05)

### Phase 5: Nginx Routing
**Goal**: Nginx sits in front of all services — routing POST /location to location-service replicas with least_conn, and WebSocket /socket connections to push-server replicas with ip_hash stickiness
**Depends on**: Phase 2, Phase 4
**Requirements**: NGINX-01, NGINX-02
**Success Criteria** (what must be TRUE):
  1. `POST /location` sent to Nginx is distributed across location-service replicas (observable via per-replica request counters in metrics)
  2. WebSocket connections to `/socket` are forwarded with correct Upgrade/Connection headers and do not drop during a 3600-second idle hold
  3. Scaling location-service or push-server replicas and reloading Nginx adds the new upstream without dropping existing connections
**Plans**: TBD

### Phase 6: Frontend
**Goal**: The browser can track a single driver on `/track/{customer_id}` with smooth animated movement, and a dispatcher can view all drivers simultaneously on `/overview` — both pages require no framework and no API key
**Depends on**: Phase 4, Phase 5
**Requirements**: FRONT-01, FRONT-02, FRONT-03, FRONT-04
**Success Criteria** (what must be TRUE):
  1. Opening `/track/{customer_id}` in a browser shows a Leaflet map with a marker that smoothly moves and rotates to match the driver's bearing on each update
  2. The `/track/{customer_id}` page displays driver ID, current speed, last update time, and live end-to-end latency per message
  3. Opening `/overview` in a browser shows all active drivers simultaneously on one map with markers color-coded by speed
  4. The frontend is a single static HTML file (no build step, no framework, no API key) served directly by Nginx
**Plans**: TBD

### Phase 7: Observability
**Goal**: Prometheus scrapes all services and cAdvisor, k6 can push metrics via remote write, and all 4 Grafana dashboards auto-provision at startup with no manual configuration
**Depends on**: Phase 2, Phase 4
**Requirements**: OBS-01, OBS-02, OBS-03, OBS-04, OBS-05, OBS-06, OBS-07, OBS-08
**Success Criteria** (what must be TRUE):
  1. After `docker compose up`, all 4 Grafana dashboards (Pipeline Health, Connection Health, Resource Usage, Stress Test) are present and populated with data — no manual import or configuration required
  2. Pipeline Health dashboard shows location updates/sec, Kafka consumer lag, Kafka publish latency p95, and end-to-end latency p50/p95/p99
  3. Connection Health dashboard shows total active WebSocket connections, per-replica connections, messages delivered/sec, BEAM process count, and BEAM memory breakdown
  4. Resource Usage dashboard shows CPU % per container, memory per container, Go goroutines, Go GC pause p99, and BEAM scheduler utilization
  5. Redpanda Console is accessible at `localhost:8080` and shows the `driver.location` topic with consumer group lag
**Plans**: TBD

### Phase 8: Stress Tests
**Goal**: Three k6 scripts exercise driver load, WebSocket connection load, and end-to-end latency — all with Prometheus remote write so results appear on the Stress Test dashboard in real time
**Depends on**: Phase 7
**Requirements**: STRESS-01, STRESS-02, STRESS-03, STRESS-04
**Success Criteria** (what must be TRUE):
  1. Running `stress/drivers.js` ramps to 2000 virtual drivers and the Stress Test dashboard shows k6 VUs, HTTP error rate, and p95 request duration — with `http_req_failed < 1%` and `http_req_duration p95 < 100ms` achievable at baseline DRIVER_COUNT
  2. Running `stress/customers.js` ramps to 10000 concurrent WebSocket connections and the dashboard shows connection count growing with no connection errors
  3. Running `stress/latency.js` measures end-to-end `Date.now() - emitted_at` and reports p50/p95/p99 on the dashboard with p95 < 500ms at default settings
  4. All k6 metrics appear on the same Grafana timeline as service metrics, enabling direct correlation between VU ramp and service behavior
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Infrastructure | 2/2 | Complete   | 2026-03-05 |
| 2. Location Service | 4/4 | Complete   | 2026-03-05 |
| 3. Driver Simulator | 6/6 | Complete   | 2026-03-06 |
| 4. Push Server | 2/5 | In Progress|  |
| 5. Nginx Routing | 0/TBD | Not started | - |
| 6. Frontend | 0/TBD | Not started | - |
| 7. Observability | 0/TBD | Not started | - |
| 8. Stress Tests | 0/TBD | Not started | - |

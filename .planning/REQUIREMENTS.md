# Requirements: Live Driver Location Simulation

**Defined:** 2026-03-05
**Core Value:** Prove that the multi-service architecture holds under load — every design decision demonstrable through Grafana metrics and reproducible experiments.

## v1 Requirements

### Infrastructure

- [x] **INFRA-01**: Full stack starts with a single `docker compose up --build` on any machine with Docker installed
- [x] **INFRA-02**: `.env.example` documents all tunable parameters (DRIVER_COUNT, EMIT_INTERVAL_MS, LOCATION_SERVICE_REPLICAS, PUSH_SERVER_REPLICAS, PARTITION_MULTIPLIER, SECRET_KEY_BASE)
- [x] **INFRA-03**: Redpanda init container creates the `driver.location` topic with `PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER` partitions before any dependent service starts
- [x] **INFRA-04**: `docker-compose.stress.yml` overlay adds k6 service so stress tests can be launched with `docker compose -f docker-compose.yml -f docker-compose.stress.yml up k6`
- [x] **INFRA-05**: Services declare correct `depends_on` ordering so the stack starts cleanly without manual intervention

### Location Service

- [x] **LSVC-01**: Location Service accepts `POST /location` with driver JSON payload, validates it, publishes to Kafka topic `driver.location` keyed by `driver_id`, and returns HTTP 200
- [x] **LSVC-02**: Location Service writes `GEOADD drivers:geo` and `SET driver:{id}:latest` (30s TTL) to Redis on each update
- [x] **LSVC-03**: Location Service exposes `GET /location/{driver_id}` returning current position from Redis
- [x] **LSVC-04**: Location Service exposes `GET /metrics` with standard Go runtime metrics plus custom counters: `location_updates_received_total`, `kafka_publish_duration_ms` (histogram), `redis_write_duration_ms` (histogram)
- [x] **LSVC-05**: Location Service Docker image uses multi-stage build with `FROM scratch` final stage (static binary, ~5MB)

### Driver Simulator

- [ ] **SIM-01**: Driver Simulator seeds customer→driver and driver→customer assignments into Redis at startup before the movement loop begins
- [ ] **SIM-02**: Each driver goroutine moves point-to-point within the São Paulo bounding box (lat -23.65 to -23.45, lng -46.75 to -46.55) at a randomized speed of 20–60 km/h, picking a new destination on arrival
- [ ] **SIM-03**: Driver Simulator emits `POST /location` with bearing, speed_kmh, and emitted_at timestamp every `EMIT_INTERVAL_MS` milliseconds per driver goroutine
- [ ] **SIM-04**: Driver Simulator Docker image uses multi-stage build with `FROM scratch` final stage
- [ ] **SIM-05**: DRIVER_COUNT and EMIT_INTERVAL_MS are configurable via environment variables

### Push Server

- [ ] **PUSH-01**: Push Server accepts WebSocket connections and joins clients to a `customer:{customer_id}` Phoenix Channel
- [ ] **PUSH-02**: On channel join, Push Server resolves assigned driver from Redis and immediately pushes the driver's current position to the connecting client
- [ ] **PUSH-03**: Broadway consumes Kafka topic `driver.location` with backpressure and broadcasts each message via `Phoenix.PubSub` to the `driver:{driver_id}` topic
- [ ] **PUSH-04**: Phoenix.PubSub uses the Redis adapter so broadcasts reach all push-server replicas (customer receives updates regardless of which replica they connected to)
- [ ] **PUSH-05**: Push Server exposes Prometheus metrics via PromEx: BEAM memory, scheduler utilization, process count, Phoenix channel events, `push_server_connections_active` (gauge), `push_server_messages_delivered_total` (counter), `push_server_delivery_latency_ms` (histogram measuring `now - emitted_at`)

### Frontend

- [ ] **FRONT-01**: `/track/{customer_id}` page connects via Phoenix Channel JS client, receives location updates, and smoothly moves and rotates a Leaflet marker using bearing and coordinates
- [ ] **FRONT-02**: `/track/{customer_id}` page displays driver ID, current speed, last update time, and live end-to-end latency (`Date.now() - emitted_at`) per message
- [ ] **FRONT-03**: `/overview` page opens one Phoenix Channel per active driver, renders all drivers simultaneously on a single Leaflet map, and color-codes markers by speed
- [ ] **FRONT-04**: Frontend is a single static HTML file served by Nginx, using Leaflet.js with OpenStreetMap tiles — no framework, no API key required

### Nginx

- [ ] **NGINX-01**: Nginx routes `POST /location` and `GET /drivers` to location-service replicas using `least_conn` load balancing
- [ ] **NGINX-02**: Nginx routes WebSocket connections at `/socket` to push-server replicas using `ip_hash` sticky sessions, with correct Upgrade/Connection headers and 3600s read timeout

### Observability

- [ ] **OBS-01**: Prometheus scrapes all services (location-service, push-server) and cAdvisor with `--web.enable-remote-write-receiver` enabled for k6 metrics
- [ ] **OBS-02**: All 4 Grafana dashboards are auto-provisioned at startup with no manual setup: Pipeline Health, Connection Health, Resource Usage, Stress Test
- [ ] **OBS-03**: Pipeline Health dashboard shows: location updates/sec, Kafka consumer lag, Kafka publish latency p95, end-to-end latency p50/p95/p99
- [ ] **OBS-04**: Connection Health dashboard shows: total active WebSocket connections, connections per replica, messages delivered/sec, BEAM process count, BEAM memory breakdown
- [ ] **OBS-05**: Resource Usage dashboard shows: CPU % per container, memory per container, Go goroutines, Go GC pause p99, BEAM scheduler utilization
- [ ] **OBS-06**: Stress Test dashboard shows k6 VUs, HTTP error rate, k6 p95 request duration, service delivery latency p95, Kafka lag, and WebSocket connections — all on the same timeline
- [ ] **OBS-07**: cAdvisor and kafka-exporter are included and scrape configs are wired to Prometheus
- [ ] **OBS-08**: Redpanda Console is available at `localhost:8080` for topic browsing and consumer lag inspection

### Stress Tests

- [ ] **STRESS-01**: `stress/drivers.js` ramps 0→2000 virtual drivers over 5 minutes, each posting to `/location` every 2s, with thresholds: `http_req_failed < 1%`, `http_req_duration p95 < 100ms`
- [ ] **STRESS-02**: `stress/customers.js` ramps 0→10000 concurrent WebSocket connections over 5 minutes, holds them open, and counts messages received, with threshold: no connection errors
- [ ] **STRESS-03**: `stress/latency.js` runs 200 drivers + 200 customers for 3 minutes, measures `Date.now() - emitted_at` per message, and reports p50/p95/p99 with goal p95 < 500ms
- [ ] **STRESS-04**: k6 pushes all metrics to Prometheus via remote write so they appear in the Stress Test Grafana dashboard in real time

## v2 Requirements

### Experiments Documentation

- **EXP-01**: README section documents Experiment 1 (horizontal scaling proof with push-server scale-up)
- **EXP-02**: README section documents Experiment 2 (Kafka consumer lag under burst)
- **EXP-03**: README section documents Experiment 3 (push-server crash and BEAM recovery)
- **EXP-04**: README section documents Experiment 4 (BEAM memory efficiency at 10k connections)
- **EXP-05**: README section documents Experiment 5 (partition multiplier effect on throughput)

### Advanced Features

- **ADV-01**: `/drivers` endpoint on location-service returns list of active drivers from Redis geo set (used by /overview)
- **ADV-02**: Grafana alerting rules for Kafka lag > threshold and error rate spikes

## Out of Scope

| Feature | Reason |
|---------|--------|
| Real user authentication | Simulated assignments only — auth adds complexity with no learning value for this domain |
| Cloud deployment / production hardening | Local Docker only — the goal is architecture mastery, not ops |
| Mobile app | Web frontend covers all learning objectives |
| Real-time chat or non-location features | Out of scope for the driver tracking domain |
| OAuth / external API keys | OpenStreetMap via Leaflet is free and keyless |
| Persistent trip history or analytics | State is ephemeral by design — Redis TTLs reflect this |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 1 | Complete |
| INFRA-02 | Phase 1 | Complete |
| INFRA-03 | Phase 1 | Complete |
| INFRA-04 | Phase 1 | Complete |
| INFRA-05 | Phase 1 | Complete |
| LSVC-01 | Phase 2 | Complete |
| LSVC-02 | Phase 2 | Complete |
| LSVC-03 | Phase 2 | Complete |
| LSVC-04 | Phase 2 | Complete |
| LSVC-05 | Phase 2 | Complete |
| SIM-01 | Phase 3 | Pending |
| SIM-02 | Phase 3 | Pending |
| SIM-03 | Phase 3 | Pending |
| SIM-04 | Phase 3 | Pending |
| SIM-05 | Phase 3 | Pending |
| PUSH-01 | Phase 4 | Pending |
| PUSH-02 | Phase 4 | Pending |
| PUSH-03 | Phase 4 | Pending |
| PUSH-04 | Phase 4 | Pending |
| PUSH-05 | Phase 4 | Pending |
| NGINX-01 | Phase 5 | Pending |
| NGINX-02 | Phase 5 | Pending |
| FRONT-01 | Phase 6 | Pending |
| FRONT-02 | Phase 6 | Pending |
| FRONT-03 | Phase 6 | Pending |
| FRONT-04 | Phase 6 | Pending |
| OBS-01 | Phase 7 | Pending |
| OBS-02 | Phase 7 | Pending |
| OBS-03 | Phase 7 | Pending |
| OBS-04 | Phase 7 | Pending |
| OBS-05 | Phase 7 | Pending |
| OBS-06 | Phase 7 | Pending |
| OBS-07 | Phase 7 | Pending |
| OBS-08 | Phase 7 | Pending |
| STRESS-01 | Phase 8 | Pending |
| STRESS-02 | Phase 8 | Pending |
| STRESS-03 | Phase 8 | Pending |
| STRESS-04 | Phase 8 | Pending |

**Coverage:**
- v1 requirements: 34 total
- Mapped to phases: 34
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-05*
*Last updated: 2026-03-05 after initial definition*

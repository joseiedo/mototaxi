# Live Driver Location Simulation

## What This Is

A production-inspired system simulating real-time driver tracking (99Food / Uber Eats style), built to be run locally with a single `docker compose up`. N drivers move realistically around São Paulo, each exclusively assigned to one customer, with their location fanned out to customer WebSocket connections via Kafka and horizontally scaled Elixir push servers. Full observability via Grafana, stress-testable with k6.

## Core Value

Prove that the multi-service architecture holds under load — every design decision should be demonstrable through Grafana metrics and reproducible experiments.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Full stack runs with a single `docker compose up` on any machine with Docker
- [ ] N drivers move realistically around São Paulo bounding box (point-to-point, randomized speed 20–60 km/h)
- [ ] Each driver is exclusively assigned to one customer (seeded in Redis at startup)
- [ ] Driver GPS updates flow: Simulator → Nginx → Location Service → Kafka → Push Server → WebSocket → Browser
- [ ] Location Service (Go): stateless ingest, validates payload, publishes to Kafka, writes to Redis
- [ ] Push Server (Elixir/Phoenix): holds customer WebSocket connections via Phoenix Channels, Broadway consumes Kafka with backpressure, Phoenix.PubSub + Redis adapter syncs across replicas
- [ ] Driver Simulator (Go): goroutine per driver, realistic movement, seeds Redis assignments at startup
- [ ] Nginx: dual-role — HTTP least_conn load balancer for location-service, ip_hash sticky for WebSocket push-servers
- [ ] Frontend: `/track/{customer_id}` (single driver, smooth marker movement with bearing) and `/overview` (all drivers, dispatcher view) — static HTML + Leaflet.js, no API key
- [ ] Prometheus scrapes all services + cAdvisor; k6 pushes metrics via remote write
- [ ] 4 pre-provisioned Grafana dashboards: Pipeline Health, Connection Health, Resource Usage, Stress Test
- [ ] k6 stress tests: driver load, customer WebSocket load, end-to-end latency measurement
- [ ] 5 documented experiments proving architectural properties (horizontal scaling, Kafka backpressure, BEAM crash recovery, BEAM memory efficiency, partition multiplier effect)

### Out of Scope

- Real persistent user accounts — simulated assignments only (no auth)
- Mobile app — web frontend only
- Cloud deployment / production hardening — local Docker only
- Real-time chat or non-location features
- OAuth or external API keys (OpenStreetMap tiles are free, no key needed)

## Context

- **Study project** — primary goal is deep mastery of each technology through hands-on implementation, not just shipping a demo
- **Machine:** macOS M4 Pro — two images require `platform: linux/amd64` (kafka-exporter, cAdvisor); Rosetta 2 handles emulation
- **Design principle:** Go where the challenge is throughput and simplicity; Elixir where the challenge is concurrency, state, and fault tolerance
- **São Paulo bounding box:** lat -23.65 to -23.45, lng -46.75 to -46.55
- **Scaling parameters externalized via .env:** DRIVER_COUNT, EMIT_INTERVAL_MS, LOCATION_SERVICE_REPLICAS, PUSH_SERVER_REPLICAS, PARTITION_MULTIPLIER
- **Topic partition formula:** PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER (auto-calculated by redpanda-init container)
- **Phoenix.PubSub correctness note:** ip_hash stickiness is a convenience optimization — pub/sub via Redis ensures delivery is correct regardless of which push-server replica a customer lands on

## Constraints

- **Docker:** All services containerized, all config externalized via environment variables
- **No API keys:** Frontend uses OpenStreetMap via Leaflet (free, no registration)
- **Static binary images:** Go services use multi-stage builds with `FROM scratch` final image (~5MB)
- **ARM64 native:** All main services use native ARM64 images; kafka-exporter and cAdvisor need linux/amd64 (already declared in compose)
- **File descriptors:** push-server needs `ulimits.nofile: 65536` for high-connection stress tests

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go for Location Service + Simulator | Goroutines cheapest for N concurrent HTTP emitters; stateless ingest benefits from thin, fast binary | — Pending |
| Elixir/Phoenix for Push Server | BEAM processes (~2KB each) for 10k+ stateful long-lived connections; Phoenix.PubSub solves fan-out natively; Broadway for backpressure-aware Kafka consumption | — Pending |
| Redpanda (Kafka-compatible) | Kafka-compatible protocol, simpler single-binary deployment for local dev | — Pending |
| Phoenix.PubSub + Redis adapter | Cross-replica pub/sub without manual connection registry | — Pending |
| ip_hash sticky sessions (Nginx) | Convenience optimization only — not a correctness requirement (PubSub handles it) | — Pending |
| Partition count = replicas × multiplier | Allows tuning intra-replica Broadway parallelism independently from replica count | — Pending |
| k6 remote write to Prometheus | Unifies stress test metrics with service metrics on same Grafana timeline — critical for bottleneck identification | — Pending |

---
*Last updated: 2026-03-05 after initialization*

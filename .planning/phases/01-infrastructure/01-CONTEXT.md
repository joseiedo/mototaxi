# Phase 1: Infrastructure - Context

**Gathered:** 2026-03-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Docker Compose skeleton that starts cleanly with `docker compose up --build` — Redpanda initialized with correct partitions, Redis ready, all config externalized via `.env`. No service implementation yet; this phase delivers the scaffolding every downstream service will be built into.

</domain>

<decisions>
## Implementation Decisions

### Directory layout

- Service directories at root: `location-service/`, `push-server/`, `simulator/`, `nginx/`, `stress/` alongside `docker-compose.yml`
- Observability config grouped under `observability/`: `observability/prometheus.yml`, `observability/grafana/provisioning/`, `observability/grafana/dashboards/`
- Init scripts under `infra/`: `infra/redpanda-init.sh`
- Each Go service has its own `go.mod` / `go.sum` (separate modules — independent builds, no dependency bleed)
- Final structure:
  ```
  mototaxi/
  ├── docker-compose.yml
  ├── docker-compose.stress.yml
  ├── .env.example
  ├── location-service/      # Go, own go.mod
  ├── push-server/           # Elixir/Phoenix
  ├── simulator/             # Go, own go.mod
  ├── nginx/
  ├── stress/                # k6 scripts
  ├── infra/                 # init scripts
  ├── observability/
  │   ├── prometheus.yml
  │   └── grafana/
  │       ├── provisioning/
  └── .planning/
  ```

### Default .env values

- `DRIVER_COUNT=10` — enough to observe concurrent behavior without hammering the machine during dev
- `EMIT_INTERVAL_MS=1000` — 1 update/sec per driver (10 req/sec baseline with default driver count)
- `LOCATION_SERVICE_REPLICAS=2`
- `PUSH_SERVER_REPLICAS=2`
- `PARTITION_MULTIPLIER=2` — yields 4 partitions by default (2 replicas × 2 multiplier)
- `SECRET_KEY_BASE` — a hardcoded dev-safe random string included in `.env.example` with a comment: "For local dev only — replace in production"

### Port exposure (host ↔ Docker)

Expose only entry points to the host; all internal service ports stay on the Docker network:

| Host port | Service |
|-----------|---------|
| :80 | Nginx (HTTP + WebSocket) |
| :3000 | Grafana |
| :8080 | Redpanda Console |
| :9090 | Prometheus |

Internal only (no host mapping): `location-service:8080`, `push-server:4000`, `redis:6379`, `redpanda broker:9092`

### Readiness / health checks

- **Redpanda:** Init container uses `rpk cluster health --brokers redpanda:9092` in a loop until healthy, then creates the `driver.location` topic. Init container is ephemeral — exits with code 0 after topic creation. Downstream services use `depends_on: condition: service_completed_successfully`.
- **Redis:** Health check via `redis-cli ping` (`interval: 5s`, `timeout: 3s`, `retries: 10`). Services that depend on Redis use `depends_on: condition: service_healthy`.
- Redpanda init container reuses the `redpandadata/redpanda` image (rpk is bundled — no extra image needed).

### Claude's Discretion

- Exact health check intervals/retries for Redpanda itself (beyond the init container pattern)
- Whether to include a `restart: unless-stopped` policy on infrastructure services
- Exact Grafana/Prometheus image versions (use latest stable)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- None — fresh project, no existing code

### Established Patterns

- None yet — this phase establishes the patterns

### Integration Points

- `docker-compose.yml` is the root integration point for all subsequent phases; each service phase adds its service block here
- `.env.example` and `.env` are the single source of truth for all tunable parameters across all phases

</code_context>

<specifics>
## Specific Ideas

- Partition count formula: `PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER` — calculated by the init container at startup from env vars, not hardcoded
- The stress overlay (`docker-compose.stress.yml`) adds only the k6 service; all other services live in the base compose file
- macOS M4 Pro: `kafka-exporter` and `cAdvisor` need `platform: linux/amd64` in their compose service definitions (Rosetta 2 handles emulation)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 01-infrastructure*
*Context gathered: 2026-03-05*

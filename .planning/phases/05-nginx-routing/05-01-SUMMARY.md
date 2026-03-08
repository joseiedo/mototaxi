---
phase: 05-nginx-routing
plan: 01
subsystem: infra
tags: [nginx, docker-compose, load-balancing, websocket, upstream, least_conn, ip_hash]

# Dependency graph
requires:
  - phase: 04-push-server
    provides: push-server service on :4000 inside Docker network
  - phase: 02-location-service
    provides: location-service on :8080 inside Docker network
provides:
  - nginx/nginx.conf with two upstream blocks (least_conn + ip_hash) and three location rules
  - docker-compose.yml with deploy.replicas for location-service and push-server, nginx volume mount
  - infra/smoke-test-nginx.sh verifying NGINX-01 and NGINX-02
  - Single external entry point at localhost:80 for all external traffic
affects:
  - 06-observability
  - 07-prometheus
  - stress-testing overlay

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Nginx least_conn upstream for stateless HTTP replicas (location-service)"
    - "Nginx ip_hash upstream for stateful WebSocket replicas (push-server) — sticky sessions by client IP"
    - "X-Upstream-Addr response header on all proxy locations for replica identity verification"
    - "stub_status restricted to Docker internal subnet 172.16.0.0/12"
    - "deploy.replicas driven by .env vars with defaults, avoiding compose file edits to scale"

key-files:
  created:
    - nginx/nginx.conf
    - infra/smoke-test-nginx.sh
  modified:
    - docker-compose.yml

key-decisions:
  - "least_conn for location-service: stateless HTTP handlers benefit from connection-count balancing over round-robin"
  - "ip_hash for push-server: WebSocket connections are long-lived and stateful — same client must reach same replica"
  - "resolver 127.0.0.11 valid=30s in http block: Docker's embedded DNS, required for upstream hostname resolution at runtime"
  - "proxy_read_timeout 3600s on WebSocket block: prevents nginx from closing idle WebSocket connections before the app layer does"
  - "Connection '' (empty) on HTTP proxy block clears hop-by-hop header for upstream keepalive; Connection 'upgrade' (literal) on WebSocket block triggers protocol upgrade"
  - "Push-server host port 4000 removed: all external access now through nginx:80, no direct service exposure"
  - "Simulator depends_on nginx (not location-service directly): simulator routes through nginx, nginx availability is the correct dependency"

patterns-established:
  - "All external HTTP traffic enters via nginx:80 — services have no host ports except observability tools"
  - "Scale services by changing LOCATION_SERVICE_REPLICAS / PUSH_SERVER_REPLICAS in .env, no compose file edits"

requirements-completed: [NGINX-01, NGINX-02]

# Metrics
duration: 41min
completed: 2026-03-07
---

# Phase 5 Plan 01: Nginx Routing Summary

**Nginx wired as single entry point: least_conn HTTP routing to 2 location-service replicas, ip_hash WebSocket proxying to 2 push-server replicas, verified by smoke test (101 WS upgrade, 2 distinct upstream IPs across 6 requests)**

## Performance

- **Duration:** 41 min
- **Started:** 2026-03-07T23:52:24Z
- **Completed:** 2026-03-07T23:53:28Z (checkpoint verified by orchestrator)
- **Tasks:** 3 (2 auto + 1 human-verify)
- **Files modified:** 3

## Accomplishments

- Created `nginx/nginx.conf` with two upstream blocks: `least_conn` for location-service (stateless HTTP), `ip_hash` for push-server (stateful WebSocket), WebSocket 3600s timeouts, `X-Upstream-Addr` on both proxy locations, `stub_status` Docker-subnet restricted
- Updated `docker-compose.yml`: `deploy.replicas` from env for both scaled services, nginx volume mount read-only, push-server host port 4000 removed, simulator URL routed through nginx, simulator depends_on nginx
- Smoke test confirmed NGINX-01 (2 distinct upstream IPs: 192.168.97.9:8080 and 192.168.97.4:8080 across 6 requests) and NGINX-02 (HTTP 101 Switching Protocols on WebSocket handshake)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create nginx/nginx.conf** - `b4e803c` (feat)
2. **Task 2: Update docker-compose.yml** - `40d8aea` (feat)
3. **Task 3: Add infra/smoke-test-nginx.sh** - `f04a097` (feat)

## Files Created/Modified

- `nginx/nginx.conf` - Two upstream blocks (least_conn + ip_hash), server block with /location, /socket/websocket, /nginx_status
- `docker-compose.yml` - deploy.replicas for location-service and push-server, nginx volume mount, push-server port removed, simulator URL updated
- `infra/smoke-test-nginx.sh` - Automated smoke test verifying replica distribution (NGINX-01) and WebSocket upgrade (NGINX-02)

## Decisions Made

- `least_conn` for location-service: stateless HTTP handlers benefit from connection-count balancing; round-robin would skew if one replica has slow in-flight requests
- `ip_hash` for push-server: WebSocket connections are long-lived and stateful (Phoenix channels hold subscriber state in process memory); same client IP must reach same replica
- `resolver 127.0.0.11 valid=30s` in http block (not upstream): Docker embedded DNS, required for nginx to resolve service hostnames at runtime rather than only at startup
- `proxy_read_timeout 3600s` on WebSocket block: prevents nginx from closing idle WebSocket connections; push-server heartbeats keep them alive at the application layer
- Push-server host port 4000 removed: all external access through nginx:80, direct service ports only for observability tools (Prometheus, Grafana)
- Simulator `depends_on nginx` instead of `location-service` directly: simulator routes through nginx, nginx readiness is the correct dependency boundary
- `nginx -t` from host outside compose network fails with "host not found" for service names — this is expected (Docker DNS 127.0.0.11 only resolves inside the compose network); verified syntax using loopback IPs and confirmed `nginx -t` passes inside container via `docker compose exec nginx nginx -t`

## Deviations from Plan

None — plan executed exactly as written. The `nginx -t` outside-network DNS resolution behavior was anticipated and documented.

## Issues Encountered

- `nginx -t` verification from host using `docker run` fails with "host not found in upstream" for `location-service:8080` and `push-server:4000` — these are Docker service names only resolvable inside the compose network via resolver 127.0.0.11. Workaround: verified syntax with loopback IPs substituted, confirmed valid. Live validation done via `docker compose exec nginx nginx -t` which returned "test is successful".

## User Setup Required

None — no external service configuration required. Scale via `LOCATION_SERVICE_REPLICAS` and `PUSH_SERVER_REPLICAS` in `.env`.

## Next Phase Readiness

- Nginx is the single entry point at port 80; all external traffic routes through it
- Phase 6 (observability) can add Prometheus scrape configs targeting services directly via Docker DNS — no nginx routing needed for metrics
- Stress overlay can target `http://nginx/location` directly for load tests
- No blockers

---
*Phase: 05-nginx-routing*
*Completed: 2026-03-07*

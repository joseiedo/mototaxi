# Phase 5: Nginx Routing - Context

**Gathered:** 2026-03-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Nginx sits in front of all services — routing POST /location (and GET /location/*) to location-service replicas with least_conn, and WebSocket /socket/websocket connections to push-server replicas with ip_hash stickiness. This phase also wires deploy.replicas into docker-compose.yml so scaling from .env works automatically.

New capabilities (frontend serving, Prometheus scraping config) belong in later phases.

</domain>

<decisions>
## Implementation Decisions

### Replica scaling model
- Use Docker Compose `--scale` / Docker DNS discovery: single upstream entry per service (`server location-service:8080;`), Docker DNS resolves to all instances automatically
- Add `deploy: replicas: ${LOCATION_SERVICE_REPLICAS:-2}` to location-service in docker-compose.yml
- Add `deploy: replicas: ${PUSH_SERVER_REPLICAS:-2}` to push-server in docker-compose.yml
- Remove push-server host port mapping (`4000:4000`) — Nginx on port 80 is the only host entry point
- Simulator `LOCATION_SERVICE_URL` points through Nginx: `http://nginx/location` (not directly to location-service)

### Phoenix WebSocket routing
- Route only `/socket/websocket` — pure WebSocket upgrade, no longpoll support needed
- Use `proxy_http_version 1.1`, `Upgrade $http_upgrade`, `Connection "upgrade"` headers
- `proxy_read_timeout 3600s` — matches the 3600s idle hold requirement; Phoenix channel heartbeats handle keepalives
- Only `/socket/websocket` is proxied to push_server upstream; internal paths (/health, /metrics) stay on Docker network

### Nginx config structure
- `nginx/nginx.conf` at root (single file, not conf.d/ split — simple enough for two upstreams)
- Two upstream blocks: `location_service` (least_conn) and `push_server` (ip_hash)
- Add `X-Upstream-Addr $upstream_addr` response header to all proxy location blocks — makes smoke testing observable (curl response reveals which replica handled the request)
- Add `stub_status` endpoint (restricted to Docker internal network) for Phase 7 Prometheus scraping

### Metrics endpoint routing
- Prometheus scrapes services DIRECTLY via Docker DNS — not through Nginx
- Nginx does NOT proxy /metrics — per-replica granularity requires direct scraping
- Nginx exposes its own `stub_status` at `/nginx_status` (allow Docker subnet, deny all) for Phase 7

### Claude's Discretion
- Exact nginx.conf worker_processes / worker_connections tuning
- proxy_connect_timeout and proxy_send_timeout values
- keepalive connections in upstream blocks
- Error page handling

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `docker-compose.yml` nginx stub: already present (`nginx:alpine`, port `80:80`, on `mototaxi` network) — needs volume mount for nginx.conf and depends_on wiring
- `.env.example` / `.env`: already has `LOCATION_SERVICE_REPLICAS` and `PUSH_SERVER_REPLICAS` variables — just wire them to `deploy.replicas`

### Established Patterns
- All services on `mototaxi` bridge network — Docker DNS hostnames work for upstream discovery
- Config externalized via environment variables — consistent with all other services
- location-service already has NO host port mapping (comment says "Nginx fronts this service in Phase 5")

### Integration Points
- `docker-compose.yml`: add `deploy.replicas` to location-service and push-server; remove push-server `ports: 4000:4000`; add nginx volume mount (`./nginx/nginx.conf:/etc/nginx/nginx.conf:ro`); add nginx `depends_on` (location-service, push-server)
- `simulator` service: update `LOCATION_SERVICE_URL` default from `http://location-service:8080` to `http://nginx/location`
- `nginx/nginx.conf`: new file — two upstream blocks + server block with location rules

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard Nginx approaches for the config structure.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 05-nginx-routing*
*Context gathered: 2026-03-07*

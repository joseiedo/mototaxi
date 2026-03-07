# Phase 5: Nginx Routing - Research

**Researched:** 2026-03-07
**Domain:** Nginx reverse proxy, load balancing, WebSocket proxying, Docker Compose replicas
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Replica scaling model:**
- Use Docker Compose `--scale` / Docker DNS discovery: single upstream entry per service (`server location-service:8080;`), Docker DNS resolves to all instances automatically
- Add `deploy: replicas: ${LOCATION_SERVICE_REPLICAS:-2}` to location-service in docker-compose.yml
- Add `deploy: replicas: ${PUSH_SERVER_REPLICAS:-2}` to push-server in docker-compose.yml
- Remove push-server host port mapping (`4000:4000`) — Nginx on port 80 is the only host entry point
- Simulator `LOCATION_SERVICE_URL` points through Nginx: `http://nginx/location` (not directly to location-service)

**Phoenix WebSocket routing:**
- Route only `/socket/websocket` — pure WebSocket upgrade, no longpoll support needed
- Use `proxy_http_version 1.1`, `Upgrade $http_upgrade`, `Connection "upgrade"` headers
- `proxy_read_timeout 3600s` — matches the 3600s idle hold requirement; Phoenix channel heartbeats handle keepalives
- Only `/socket/websocket` is proxied to push_server upstream; internal paths (/health, /metrics) stay on Docker network

**Nginx config structure:**
- `nginx/nginx.conf` at root (single file, not conf.d/ split — simple enough for two upstreams)
- Two upstream blocks: `location_service` (least_conn) and `push_server` (ip_hash)
- Add `X-Upstream-Addr $upstream_addr` response header to all proxy location blocks — makes smoke testing observable (curl response reveals which replica handled the request)
- Add `stub_status` endpoint (restricted to Docker internal network) for Phase 7 Prometheus scraping

**Metrics endpoint routing:**
- Prometheus scrapes services DIRECTLY via Docker DNS — not through Nginx
- Nginx does NOT proxy /metrics — per-replica granularity requires direct scraping
- Nginx exposes its own `stub_status` at `/nginx_status` (allow Docker subnet, deny all) for Phase 7

### Claude's Discretion
- Exact nginx.conf worker_processes / worker_connections tuning
- proxy_connect_timeout and proxy_send_timeout values
- keepalive connections in upstream blocks
- Error page handling

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| NGINX-01 | Nginx routes `POST /location` and `GET /drivers` to location-service replicas using `least_conn` load balancing | Docker DNS with single upstream server entry distributes across replicas; `least_conn` treats each resolved IP as a separate target |
| NGINX-02 | Nginx routes WebSocket connections at `/socket` to push-server replicas using `ip_hash` sticky sessions, with correct Upgrade/Connection headers and 3600s read timeout | `ip_hash` upstream + WebSocket header pattern + `proxy_read_timeout 3600s` fully supported in nginx:alpine |
</phase_requirements>

---

## Summary

Phase 5 wires Nginx as the single external entry point for the mototaxi stack. The work splits into three areas: authoring `nginx/nginx.conf` with two upstream blocks and a server block, updating `docker-compose.yml` to add `deploy.replicas`, mount the config, fix `depends_on`, and remove the push-server host port, and updating the simulator's `LOCATION_SERVICE_URL` to route through Nginx.

The key technical insight is that when nginx resolves a Docker Compose service name (e.g., `location-service`) in an upstream `server` directive, Docker's embedded DNS (127.0.0.11) returns all replica IPs at resolution time. Nginx then treats each IP as an independent upstream target and applies `least_conn` (or `ip_hash`) across all of them. New replicas added after nginx starts are NOT picked up until `nginx -s reload` — this is the intended behavior per the phase success criterion ("reloading Nginx adds the new upstream without dropping existing connections"). The `resolver` directive is NOT needed in the upstream block approach; it is only required for the `set $var` + `proxy_pass $var` variable pattern, which would lose upstream load-balancing semantics.

For WebSocket proxying, nginx requires `proxy_http_version 1.1` and the `Upgrade`/`Connection` headers to negotiate the protocol upgrade. The `proxy_read_timeout 3600s` prevents nginx from closing idle WebSocket connections before Phoenix's channel heartbeat fires. The `ip_hash` method uses the client IP's first three octets to select a replica, providing stickiness so a client always routes to the same push-server replica for the lifetime of its connection.

**Primary recommendation:** Use nginx:alpine with a single `nginx/nginx.conf` file. Two upstream blocks (least_conn for HTTP, ip_hash for WebSocket), one server block, three location rules (/location, /socket/websocket, /nginx_status). Add `resolver 127.0.0.11 valid=30s;` inside the `http` block for robustness on container restarts (does not break upstream behavior). Add `keepalive 32` to each upstream block for connection reuse.

---

## Standard Stack

### Core

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| nginx:alpine | stable (1.27.x as of 2025) | Reverse proxy, load balancer | Lightweight, included in docker-compose.yml stub, no additional deps |
| Docker Compose `deploy.replicas` | Compose v3.x | Scale services declaratively | Already in .env.example, no orchestrator needed |

### Supporting

| Directive | Scope | Purpose | When to Use |
|-----------|-------|---------|-------------|
| `least_conn` | upstream block | HTTP load balancing by active connections | location-service (stateless, connection count meaningful) |
| `ip_hash` | upstream block | Sticky sessions by client IP (3 octets) | push-server (WebSocket, must stay on same replica) |
| `keepalive N` | upstream block | Reuse connections to backends | Both upstreams — reduces TCP handshake overhead |
| `stub_status` | location block | Nginx internal metrics | Restricted to Docker subnet for Phase 7 Prometheus |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Single `server hostname:port` in upstream | Multiple explicit `server ip:port` entries | Explicit IPs require updating config per scale event; hostname + Docker DNS is automatic |
| `ip_hash` for WebSocket sticky | `hash $remote_addr` | `ip_hash` is equivalent for IPv4, simpler syntax |
| Single nginx.conf | conf.d/ split files | conf.d/ not needed for two upstreams; adds complexity |

**Installation:** No install needed — image is `nginx:alpine` already in docker-compose.yml.

---

## Architecture Patterns

### Recommended Project Structure

```
nginx/
└── nginx.conf        # Single file: events block, http block, two upstreams, server block

docker-compose.yml    # Add deploy.replicas, volume mount, depends_on
```

### Pattern 1: Two Upstream Blocks, One Server Block

**What:** All routing decisions live in a single `server` block with `location` directives matching path prefixes.
**When to use:** Two backend clusters with different load-balancing semantics (HTTP vs WebSocket).

```nginx
# Source: https://nginx.org/en/docs/http/ngx_http_upstream_module.html
# Source: https://nginx.org/en/docs/http/ngx_http_websocket_module.html

worker_processes auto;

events {
    worker_connections 1024;
}

http {
    resolver 127.0.0.11 valid=30s;   # Docker embedded DNS — enables re-resolution after container restart

    upstream location_service {
        least_conn;
        server location-service:8080;
        keepalive 32;
    }

    upstream push_server {
        ip_hash;
        server push-server:4000;
        keepalive 32;
    }

    server {
        listen 80;

        # HTTP: location-service
        location /location {
            proxy_pass         http://location_service;
            proxy_http_version 1.1;
            proxy_set_header   Connection "";
            proxy_set_header   Host $host;
            proxy_set_header   X-Real-IP $remote_addr;
            add_header         X-Upstream-Addr $upstream_addr;
            proxy_connect_timeout 5s;
            proxy_send_timeout    30s;
            proxy_read_timeout    30s;
        }

        # WebSocket: push-server Phoenix channel
        location /socket/websocket {
            proxy_pass            http://push_server;
            proxy_http_version    1.1;
            proxy_set_header      Upgrade $http_upgrade;
            proxy_set_header      Connection "upgrade";
            proxy_set_header      Host $host;
            proxy_set_header      X-Real-IP $remote_addr;
            add_header            X-Upstream-Addr $upstream_addr;
            proxy_connect_timeout 5s;
            proxy_send_timeout    3600s;
            proxy_read_timeout    3600s;
        }

        # Nginx metrics — restricted to Docker internal network
        location /nginx_status {
            stub_status;
            allow 172.16.0.0/12;   # Docker default bridge subnet range
            allow 127.0.0.1;
            deny  all;
        }
    }
}
```

### Pattern 2: Docker Compose `deploy.replicas` with env variable

**What:** Wire `LOCATION_SERVICE_REPLICAS` and `PUSH_SERVER_REPLICAS` from `.env` into `deploy.replicas`.
**When to use:** All stateless services that need horizontal scaling.

```yaml
# Source: Docker Compose spec — deploy.replicas
location-service:
  build:
    context: ./location-service
  deploy:
    replicas: ${LOCATION_SERVICE_REPLICAS:-2}
  # ... rest of service definition

push-server:
  build:
    context: ./push-server
  deploy:
    replicas: ${PUSH_SERVER_REPLICAS:-2}
  # ports: "4000:4000"  <- REMOVE this line

nginx:
  image: nginx:alpine
  restart: unless-stopped
  ports:
    - "80:80"
  volumes:
    - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
  depends_on:
    - location-service
    - push-server
  networks:
    - mototaxi
```

### Pattern 3: Simulator URL updated to go through Nginx

```yaml
simulator:
  environment:
    LOCATION_SERVICE_URL: ${LOCATION_SERVICE_URL:-http://nginx/location}
```

### Anti-Patterns to Avoid

- **`Connection ""` missing for keepalive:** When `keepalive` is set in upstream, the `proxy_set_header Connection ""` directive MUST be set in HTTP location blocks to clear the hop-by-hop Connection header. Omitting it causes keep-alive negotiation failures with backends.
- **`Connection "upgrade"` literal string (not variable) for WebSocket:** For WebSocket locations, use `Connection "upgrade"` (the literal string). For regular HTTP locations, use `Connection ""` (empty, clearing the header). Using `$connection_upgrade` variable requires a map block that is not needed here since the two routes are separate.
- **ip_hash with proxy_http_version 1.0:** Always set `proxy_http_version 1.1` in WebSocket location — HTTP/1.0 does not support Upgrade.
- **No `proxy_read_timeout` on WebSocket location:** Default nginx `proxy_read_timeout` is 60s, which will kill idle WebSocket connections before the 3600s requirement.
- **Host port on push-server with replicas:** Docker Compose will refuse to start multiple replicas of a service with a `ports` host mapping (only one container can bind a host port). Remove `ports: "4000:4000"` from push-server.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Sticky WebSocket sessions | Custom token-based routing, app-layer session registry | `ip_hash` in upstream | ip_hash is deterministic, zero-overhead, no state needed |
| Per-replica load distribution | Custom proxy middleware | `least_conn` directive | Nginx tracks active connections per resolved IP natively |
| Nginx metrics collection | Custom metrics endpoint | `stub_status` module | Built into nginx:alpine, scraped by nginx-prometheus-exporter in Phase 7 |
| Dynamic upstream discovery | consul-template, nginx-plus, custom DNS watcher | Docker DNS + `nginx -s reload` | Sufficient for this scale; Nginx Plus needed only for zero-downtime upstream updates |

**Key insight:** Nginx open-source with Docker DNS covers all Phase 5 needs. Nginx Plus's dynamic upstream re-resolution is explicitly not needed — the phase success criterion only requires reload (not zero-downtime hot discovery) to add new replicas.

---

## Common Pitfalls

### Pitfall 1: Docker Compose replicas + host ports conflict

**What goes wrong:** `docker compose up` fails with "port is already allocated" or refuses to start multiple containers for a service that has a `ports:` host binding.
**Why it happens:** Only one container can own a host port — Docker cannot bind `4000:4000` for two push-server replicas simultaneously.
**How to avoid:** Remove `ports: "4000:4000"` from push-server in docker-compose.yml. Nginx on port 80 is the sole host entry point.
**Warning signs:** `docker compose up` error mentioning "address already in use" or only one replica starting.

### Pitfall 2: Nginx resolves Docker hostname to one IP (caching)

**What goes wrong:** All traffic goes to a single replica even when multiple replicas are running.
**Why it happens:** Nginx resolves the hostname once at startup and caches it. If only one replica was running at that moment (race condition), or if DNS returned one IP, nginx sticks to it.
**How to avoid:** Include `resolver 127.0.0.11 valid=30s;` in the `http` block. This tells nginx to use Docker's DNS and re-resolve at the specified TTL. Ensure nginx starts AFTER replicas are healthy (depends_on).
**Warning signs:** `X-Upstream-Addr` response header always shows the same IP regardless of request.

### Pitfall 3: WebSocket connections drop after 60 seconds

**What goes wrong:** Phoenix channel clients silently disconnect after ~60s of inactivity.
**Why it happens:** Default `proxy_read_timeout` in nginx is 60s. Nginx closes the upstream connection when no data is read in 60s. Phoenix heartbeat (default 30s) keeps the channel alive but nginx kills the TCP connection.
**How to avoid:** Set `proxy_read_timeout 3600s` AND `proxy_send_timeout 3600s` on the `/socket/websocket` location block. Both must be set.
**Warning signs:** WebSocket connections close exactly at 60s intervals; Phoenix logs show "transport closed" without client disconnect.

### Pitfall 4: Missing `Upgrade`/`Connection` headers causes 101 failure

**What goes wrong:** WebSocket clients get HTTP 400 or 502 instead of 101 Switching Protocols.
**Why it happens:** By default, nginx does not forward hop-by-hop headers. The WebSocket upgrade handshake requires `Upgrade: websocket` and `Connection: upgrade` to be forwarded.
**How to avoid:** Add `proxy_set_header Upgrade $http_upgrade;` and `proxy_set_header Connection "upgrade";` to the WebSocket location block.
**Warning signs:** Browser devtools shows status 400/502 on the WebSocket request; Phoenix JS client logs "transport closed" immediately on connect.

### Pitfall 5: `add_header X-Upstream-Addr` not visible in WebSocket responses

**What goes wrong:** Smoke test using wscat/websocat cannot observe which replica handled the WebSocket.
**Why it happens:** `add_header` in the WebSocket location is applied to the HTTP 101 upgrade response headers — visible in the handshake, not in subsequent frames.
**How to avoid:** Use curl to test the HTTP location (`POST /location`) for the `X-Upstream-Addr` header. The header IS present on the 101 response but tools like wscat may not display it. Use `curl -v` to see raw upgrade response headers.
**Warning signs:** Only relevant during smoke testing; not a runtime issue.

### Pitfall 6: `deploy.replicas` requires Docker Compose v3 `deploy` key

**What goes wrong:** `deploy: replicas` is silently ignored in non-swarm Compose or causes a parse error in v2 YAML.
**Why it happens:** The `deploy` key is part of the Compose v3 spec and is respected by modern `docker compose` (v2 CLI plugin) even without Swarm mode.
**How to avoid:** The existing docker-compose.yml already uses v3 syntax (no explicit `version:` field — Compose CLI v2 auto-detects). Verify with `docker compose config` that `deploy.replicas` appears in the resolved config.
**Warning signs:** Running `docker compose up` starts only one replica; `docker compose ps` shows only one container for the service.

---

## Code Examples

### Complete nginx.conf

```nginx
# Source: nginx.org/en/docs/http/ngx_http_upstream_module.html
#         nginx.org/en/docs/http/load_balancing.html

worker_processes auto;

events {
    worker_connections 1024;
}

http {
    resolver 127.0.0.11 valid=30s;

    upstream location_service {
        least_conn;
        server location-service:8080;
        keepalive 32;
    }

    upstream push_server {
        ip_hash;
        server push-server:4000;
        keepalive 32;
    }

    server {
        listen 80;

        location /location {
            proxy_pass         http://location_service;
            proxy_http_version 1.1;
            proxy_set_header   Connection "";
            proxy_set_header   Host $host;
            proxy_set_header   X-Real-IP $remote_addr;
            add_header         X-Upstream-Addr $upstream_addr always;
            proxy_connect_timeout 5s;
            proxy_send_timeout    30s;
            proxy_read_timeout    30s;
        }

        location /socket/websocket {
            proxy_pass            http://push_server;
            proxy_http_version    1.1;
            proxy_set_header      Upgrade $http_upgrade;
            proxy_set_header      Connection "upgrade";
            proxy_set_header      Host $host;
            proxy_set_header      X-Real-IP $remote_addr;
            add_header            X-Upstream-Addr $upstream_addr always;
            proxy_connect_timeout 5s;
            proxy_send_timeout    3600s;
            proxy_read_timeout    3600s;
        }

        location /nginx_status {
            stub_status;
            allow 172.16.0.0/12;
            allow 127.0.0.1;
            deny  all;
        }
    }
}
```

### Smoke-test commands

```bash
# Verify least_conn distribution — run multiple times, observe different IPs in X-Upstream-Addr
curl -si -X POST http://localhost/location \
  -H "Content-Type: application/json" \
  -d '{"driver_id":"d1","lat":-23.5,"lng":-46.6,"bearing":90.0,"speed_kmh":30.0,"emitted_at":"2026-03-07T10:00:00Z"}' \
  | grep X-Upstream-Addr

# Verify WebSocket upgrade headers (101 response)
curl -si --include \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  -H "Sec-WebSocket-Version: 13" \
  http://localhost/socket/websocket

# Verify nginx_status endpoint reachable from host (Docker host is not in 172.16/12 — may return 403)
curl http://localhost/nginx_status
```

### `docker compose up` with scaling

```bash
# Start with .env defaults (LOCATION_SERVICE_REPLICAS=2, PUSH_SERVER_REPLICAS=2)
docker compose up --build -d

# Verify replica count
docker compose ps

# Add a replica without downtime, then reload nginx
docker compose up --scale location-service=3 -d --no-recreate
docker compose exec nginx nginx -s reload

# Confirm X-Upstream-Addr now shows 3 distinct IPs across requests
for i in $(seq 1 9); do
  curl -s -o /dev/null -D - -X POST http://localhost/location \
    -H "Content-Type: application/json" \
    -d '{"driver_id":"d1","lat":-23.5,"lng":-46.6,"bearing":0,"speed_kmh":20,"emitted_at":"2026-03-07T10:00:00Z"}' \
    | grep X-Upstream-Addr
done
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Explicit `server ip:port` per replica | Single `server hostname:port` + Docker DNS | Docker Compose networking (2016+) | No config change needed when scaling |
| `version: "3"` key in docker-compose.yml | No explicit `version:` key (auto-detected) | Docker Compose CLI v2 (2021) | Simpler files; `deploy.replicas` still works |
| `nginx -s reload` drops connections | Graceful reload: workers finish in-flight requests | nginx has always done this | Safe to reload with active WebSocket connections |

**Deprecated/outdated:**
- `links:` in docker-compose.yml: replaced by network DNS; never use for service discovery
- `docker-compose` (hyphenated v1 Python tool): replaced by `docker compose` (v2 Go plugin)

---

## Open Questions

1. **Docker subnet CIDR for stub_status allow rule**
   - What we know: Docker default bridge uses 172.17.0.0/16; named bridge networks (like `mototaxi`) use sequential subnets in the 172.16.0.0/12 range
   - What's unclear: The exact subnet assigned to `mototaxi` network at runtime (could be 172.18.x, 172.19.x, etc.)
   - Recommendation: Use `allow 172.16.0.0/12` to cover all Docker bridge subnets, plus `allow 127.0.0.1`. This is the standard pattern for Docker-internal access control.

2. **`always` flag on `add_header`**
   - What we know: Without `always`, nginx only adds response headers on 2xx/3xx responses
   - What's unclear: Whether `add_header X-Upstream-Addr ... always` is needed for observability on error responses
   - Recommendation: Include `always` so the header appears on 4xx/5xx responses too — useful for debugging which replica produced an error.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Shell/curl smoke tests (no unit test framework for nginx config) |
| Config file | none — nginx config is validated with `nginx -t` |
| Quick run command | `docker compose exec nginx nginx -t` |
| Full suite command | `docker compose up -d && bash infra/smoke-test-nginx.sh` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| NGINX-01 | POST /location distributed across location-service replicas (different X-Upstream-Addr per request) | smoke | `for i in $(seq 1 6); do curl -s -X POST http://localhost/location -H 'Content-Type: application/json' -d '{"driver_id":"d1","lat":-23.5,"lng":-46.6,"bearing":0,"speed_kmh":20,"emitted_at":"2026-03-07T10:00:00Z"}' -o /dev/null -D - | grep X-Upstream-Addr; done` | ❌ Wave 0 |
| NGINX-02 | WebSocket /socket/websocket upgrades correctly (101) and stays open for 3600s | smoke | `curl -si -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' -H 'Sec-WebSocket-Version: 13' http://localhost/socket/websocket` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `docker compose exec nginx nginx -t`
- **Per wave merge:** Full smoke test script
- **Phase gate:** Both NGINX-01 and NGINX-02 manually verified before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `nginx/nginx.conf` — the config file itself (primary deliverable of this phase)
- [ ] `infra/smoke-test-nginx.sh` — optional helper script for distribution and WebSocket tests
- [ ] Nginx config validation: `nginx -t` in CI/pre-commit not wired (acceptable for local-dev project)

---

## Sources

### Primary (HIGH confidence)

- [nginx.org/en/docs/http/ngx_http_upstream_module.html](https://nginx.org/en/docs/http/ngx_http_upstream_module.html) — `least_conn`, `ip_hash`, `keepalive`, `$upstream_addr` variable
- [nginx.org/en/docs/http/load_balancing.html](https://nginx.org/en/docs/http/load_balancing.html) — DNS multi-IP resolution with upstream, load balancing methods
- [docs.nginx.com — HTTP Load Balancer](https://docs.nginx.com/nginx/admin-guide/load-balancer/http-load-balancer/) — upstream configuration patterns

### Secondary (MEDIUM confidence)

- [ecostack.dev — Load Balancing Docker Compose Replicas Using Nginx](https://ecostack.dev/posts/load-balancing-docker-compose-replicas-using-nginx/) — confirmed single server hostname + Docker DNS works for replica distribution
- [oneuptime.com — Nginx WebSocket Proxy](https://oneuptime.com/blog/post/2026-01-25-nginx-websocket-proxy/view) — WebSocket header pattern and proxy_read_timeout 3600s
- [codegenes.net — Nginx DNS Re-Resolution in Docker](https://www.codegenes.net/blog/nginx-does-not-re-resolve-dns-names-in-docker/) — resolver 127.0.0.11 placement: http block, not upstream block
- [websocket.org — Nginx WebSocket Guide](https://websocket.org/guides/infrastructure/nginx/) — ip_hash + WebSocket sticky session pattern

### Tertiary (LOW confidence)

- WebSearch aggregated findings on Docker bridge subnet CIDR (172.16.0.0/12 for named networks) — verified directionally against Docker networking docs but not pinned to a specific release note

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — nginx:alpine is the declared image; directives verified against official nginx docs
- Architecture: HIGH — upstream DNS pattern confirmed by official docs + multiple community sources; Docker Compose `deploy.replicas` behavior confirmed
- Pitfalls: HIGH — port conflict and WebSocket timeout pitfalls are well-documented; Docker DNS caching is a known nginx behavior documented officially
- Validation: MEDIUM — smoke test commands are correct but no existing test infrastructure; all gaps listed in Wave 0

**Research date:** 2026-03-07
**Valid until:** 2026-06-07 (stable nginx OSS; Docker Compose behavior stable)

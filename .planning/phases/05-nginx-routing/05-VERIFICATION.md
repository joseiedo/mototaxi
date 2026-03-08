---
phase: 05-nginx-routing
verified: 2026-03-07T23:59:00Z
re_verified: 2026-03-07T00:00:00Z
status: human_needed
score: 5/5 must-haves verified
re_verification:
  previous_status: gaps_found
  previous_score: 4/5
  gaps_closed:
    - "Smoke test exits 0 on success and 1 on failure (automated CI gate)"
    - ".env.example LOCATION_SERVICE_URL documents stale pre-phase-5 value"
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "POST /location distributes across ≥2 distinct upstream IPs"
    expected: "6 POST requests to http://localhost/location show X-Upstream-Addr values from at least 2 distinct IPs (e.g. 192.168.x.y:8080 and 192.168.a.b:8080)"
    why_human: "Requires live docker compose stack with 2 location-service replicas; verified by human checkpoint during plan execution (SUMMARY documents 192.168.97.9:8080 and 192.168.97.4:8080)"
  - test: "WebSocket /socket/websocket returns HTTP 101 Switching Protocols"
    expected: "curl with Upgrade:websocket headers receives 101 response from push-server via nginx"
    why_human: "Requires live push-server; verified by human checkpoint during plan execution (SUMMARY documents HTTP 101 confirmed)"
  - test: "docker compose ps shows 2 location-service and 2 push-server replicas"
    expected: "mototaxi-location-service-1, mototaxi-location-service-2, mototaxi-push-server-1, mototaxi-push-server-2 all appear in docker compose ps"
    why_human: "Requires running stack; verified by human checkpoint during plan execution"
---

# Phase 5: Nginx Routing — Verification Report

**Phase Goal:** Wire Nginx as the single external entry point for the mototaxi stack. Route POST /location to location-service replicas with least_conn load balancing, and WebSocket /socket/websocket connections to push-server replicas with ip_hash sticky sessions. Enable declarative scaling via deploy.replicas from .env.
**Verified:** 2026-03-07T23:59:00Z (initial), re-verified 2026-03-07
**Status:** human_needed
**Re-verification:** Yes — after gap closure

## Re-Verification Summary

Previous status: `gaps_found` (4/5 must-haves verified)
Current status: `human_needed` (5/5 must-haves verified)

**Gap closed — smoke test exit codes (infra/smoke-test-nginx.sh):**

The previous gap was: WARN/NOTE branches did not call `exit 1`, so the script always exited 0 regardless of failure.

The script has been rewritten. The two failure branches now call `exit 1`:

- Line 19-21: NGINX-01 failure branch now prints `FAIL` and calls `exit 1` (was: `WARN` with no exit)
- Line 39-40: NGINX-02 failure branch now prints `FAIL` and calls `exit 1` (was: `NOTE` with no exit)

The associative array (`declare -A seen_ips`) was replaced with a portable string+sort pipeline (`echo "$seen_ips" | tr ' ' '\n' | grep -v '^$' | sort -u | wc -l`) which is more compatible across bash versions.

**Gap closed — .env.example stale LOCATION_SERVICE_URL:**

`LOCATION_SERVICE_URL` in `.env.example` now reads `http://nginx/location` (was: `http://location-service:8080`). A developer copying `.env.example` to `.env` will now get the correct nginx-routed URL as a default, not the pre-phase-5 direct service URL.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | POST /location sent to localhost:80 reaches a location-service replica (HTTP 200, X-Upstream-Addr header present) | ? HUMAN | nginx.conf has correct /location proxy_pass to location_service upstream with add_header X-Upstream-Addr; runtime confirmed by SUMMARY human checkpoint |
| 2 | Multiple POST /location requests distribute across replicas (at least 2 distinct IPs in X-Upstream-Addr across 6 requests) | ? HUMAN | least_conn upstream configured; smoke test now exits 1 on failure; SUMMARY documents 192.168.97.9:8080 and 192.168.97.4:8080 seen |
| 3 | GET /location/websocket to localhost:80/socket/websocket returns HTTP 101 Switching Protocols | ? HUMAN | /socket/websocket location block with ip_hash upstream and correct Upgrade/Connection headers verified in config; SUMMARY documents HTTP 101 confirmed |
| 4 | docker compose up starts 2 location-service replicas and 2 push-server replicas by default | ? HUMAN | deploy.replicas: ${LOCATION_SERVICE_REPLICAS:-2} and ${PUSH_SERVER_REPLICAS:-2} both present in docker-compose.yml; runtime confirmed by SUMMARY |
| 5 | nginx -t validates the nginx.conf without errors | ? HUMAN | nginx.conf syntax is well-formed (verified via file inspection); SUMMARY documents `docker compose exec nginx nginx -t` returned "test is successful" |

**Score:** 5/5 must-haves structurally verified. All runtime truths (1-5) require human/live-stack confirmation — documented as verified in SUMMARY.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `nginx/nginx.conf` | Two upstream blocks (least_conn + ip_hash), one server block with three location rules; least_conn, ip_hash, proxy_read_timeout 3600s, stub_status | VERIFIED | 61 lines; resolver 127.0.0.11 valid=30s in http block; upstream location_service with least_conn + keepalive 32; upstream push_server with ip_hash + keepalive 32; /location block with Connection "" (HTTP/1.1 keepalive) and X-Upstream-Addr; /socket/websocket block with Connection "upgrade", proxy_read_timeout 3600s, proxy_send_timeout 3600s, X-Upstream-Addr; /nginx_status with stub_status + allow 172.16.0.0/12 + deny all |
| `docker-compose.yml` | deploy.replicas for location-service and push-server, nginx volume mount and depends_on | VERIFIED | deploy.replicas: ${LOCATION_SERVICE_REPLICAS:-2} (line 74), ${PUSH_SERVER_REPLICAS:-2} (line 113), ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro (line 197), nginx depends_on location-service + push-server, push-server host port 4000 absent, simulator LOCATION_SERVICE_URL → http://nginx/location |
| `infra/smoke-test-nginx.sh` | Automated smoke test verifying NGINX-01 and NGINX-02; exits 0 on success, 1 on failure | VERIFIED | 49 lines, executable (-rwxr-xr-x). Covers NGINX-01 (string-based distinct IP count, exit 1 on < 2) and NGINX-02 (WebSocket 101 check, exit 1 on non-101). Both failure branches now call exit 1. Script is a valid CI gate. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| nginx/nginx.conf upstream location_service | location-service:8080 | Docker DNS resolution | WIRED | `server location-service:8080` present at line 12 |
| nginx/nginx.conf upstream push_server | push-server:4000 | ip_hash + Docker DNS resolution | WIRED | `server push-server:4000` present at line 18 |
| docker-compose.yml nginx volumes | nginx/nginx.conf | read-only bind mount | WIRED | `./nginx/nginx.conf:/etc/nginx/nginx.conf:ro` at line 197 |
| simulator LOCATION_SERVICE_URL | nginx:80/location | environment variable default | WIRED | `LOCATION_SERVICE_URL: ${LOCATION_SERVICE_URL:-http://nginx/location}` at line 101; .env.example also updated to the same value |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| NGINX-01 | 05-01-PLAN.md | Nginx routes POST /location and GET /location/{driver_id} to location-service replicas using least_conn load balancing | SATISFIED | `upstream location_service { least_conn; server location-service:8080; }` and `location /location { proxy_pass http://location_service; }` in nginx.conf; smoke test verifies distribution at runtime; SUMMARY human checkpoint confirmed |
| NGINX-02 | 05-01-PLAN.md | Nginx routes WebSocket connections at /socket to push-server replicas using ip_hash sticky sessions, with correct Upgrade/Connection headers and 3600s read timeout | SATISFIED | `upstream push_server { ip_hash; server push-server:4000; }` and `location /socket/websocket { proxy_set_header Upgrade $http_upgrade; proxy_set_header Connection "upgrade"; proxy_read_timeout 3600s; proxy_send_timeout 3600s; }` in nginx.conf; runtime HTTP 101 confirmed by SUMMARY |

**Orphaned Requirements Check:** No requirements mapped to Phase 5 in REQUIREMENTS.md beyond NGINX-01 and NGINX-02. Both accounted for.

**REQUIREMENTS.md path note:** NGINX-02 text says `/socket` — the nginx.conf routes `/socket/websocket`. This is correct: Phoenix appends `/websocket` to the socket path when the endpoint is mounted at `/socket`. The implementation matches Phoenix convention.

### Anti-Patterns Found

No blocking anti-patterns remain. Previous warnings are resolved:

| File | Line | Pattern | Severity | Resolution |
|------|------|---------|----------|------------|
| ~~`infra/smoke-test-nginx.sh`~~ | ~~19-22~~ | ~~WARN branch does not `exit 1`~~ | ~~Warning~~ | FIXED — now prints FAIL and calls `exit 1` |
| ~~`infra/smoke-test-nginx.sh`~~ | ~~35-39~~ | ~~NOTE branch does not `exit 1`~~ | ~~Warning~~ | FIXED — now prints FAIL and calls `exit 1` |
| ~~`.env.example`~~ | ~~4~~ | ~~LOCATION_SERVICE_URL shows pre-phase-5 direct URL~~ | ~~Info~~ | FIXED — updated to `http://nginx/location` |

### Human Verification Required

All three human verifications were performed and passed during plan execution (Task 3 human-verify checkpoint, documented in 05-01-SUMMARY.md). They are listed here for completeness.

#### 1. Replica distribution (NGINX-01)

**Test:** `docker compose up -d && sleep 30 && bash infra/smoke-test-nginx.sh`
**Expected:** "OK: Distribution confirmed across 2 replicas" output — at least 2 distinct X-Upstream-Addr IPs across 6 POST /location requests; script exits 0
**Why human:** Requires live stack with two location-service containers; static analysis can only confirm config structure

#### 2. WebSocket 101 upgrade (NGINX-02)

**Test:** Run `bash infra/smoke-test-nginx.sh` with stack running
**Expected:** "OK: WebSocket upgrade succeeded" output (HTTP 101 Switching Protocols); script exits 0
**Why human:** Requires live push-server and nginx; cannot verify protocol upgrade without running services

#### 3. nginx -t inside container

**Test:** `docker compose exec nginx nginx -t`
**Expected:** `nginx: configuration file /etc/nginx/nginx.conf test is successful`
**Why human:** Host-side `nginx -t` fails (Docker DNS 127.0.0.11 only resolves inside compose network); must verify inside container

All three confirmed in 05-01-SUMMARY.md.

## Gaps Summary

No gaps remain. All must-haves are satisfied:

- `nginx/nginx.conf` exists, is substantive (61 lines, all required directives present), and wired via docker-compose.yml volume mount
- `docker-compose.yml` has deploy.replicas for both services, nginx volume mount, nginx depends_on, simulator URL through nginx, push-server host port absent
- `infra/smoke-test-nginx.sh` is executable, covers both NGINX-01 and NGINX-02, and is now a valid CI gate (exits 1 on failure)
- `.env.example` documents `http://nginx/location` as the correct default
- NGINX-01 and NGINX-02 are both SATISFIED with implementation evidence and runtime confirmation via SUMMARY

Phase 5 goal is achieved. Proceed to Phase 6.

---

_Initial verification: 2026-03-07T23:59:00Z_
_Re-verification: 2026-03-07_
_Verifier: Claude (gsd-verifier)_

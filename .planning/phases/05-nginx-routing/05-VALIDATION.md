---
phase: 5
slug: nginx-routing
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-07
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Shell/curl smoke tests (no unit test framework for nginx config) |
| **Config file** | none — nginx config is validated with `nginx -t` |
| **Quick run command** | `docker compose exec nginx nginx -t` |
| **Full suite command** | `docker compose up -d && bash infra/smoke-test-nginx.sh` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `docker compose exec nginx nginx -t`
- **After every plan wave:** Run `docker compose up -d && bash infra/smoke-test-nginx.sh`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| nginx-conf | 01 | 1 | NGINX-01, NGINX-02 | config-check | `docker compose exec nginx nginx -t` | ❌ W0 | ⬜ pending |
| compose-replicas | 01 | 1 | NGINX-01, NGINX-02 | smoke | `docker compose config \| grep -A2 replicas` | ✅ | ⬜ pending |
| nginx-01-smoke | 01 | 2 | NGINX-01 | smoke | `for i in $(seq 1 6); do curl -s -X POST http://localhost/location -H 'Content-Type: application/json' -d '{"driver_id":"d1","lat":-23.5,"lng":-46.6,"bearing":0,"speed_kmh":20,"emitted_at":"2026-03-07T10:00:00Z"}' -o /dev/null -D - \| grep X-Upstream-Addr; done` | ❌ W0 | ⬜ pending |
| nginx-02-smoke | 01 | 2 | NGINX-02 | smoke | `curl -si -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' -H 'Sec-WebSocket-Version: 13' http://localhost/socket/websocket` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `nginx/nginx.conf` — the nginx configuration file (primary deliverable)
- [ ] `infra/smoke-test-nginx.sh` — smoke test script for distribution and WebSocket verification

*Note: `nginx -t` is the primary automated validation gate after each task.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| WebSocket holds open for 3600s idle | NGINX-02 | Timeout takes 1 hour to observe directly | Start wscat/websocat connection, verify still alive after 30+ minutes; rely on proxy_read_timeout config review |
| X-Upstream-Addr shows distinct IPs across requests | NGINX-01 | Requires running stack with 2+ replicas | `docker compose up -d`, run curl POST loop, verify ≥2 distinct upstream addresses appear |
| Adding replica + nginx reload picks up new upstream | NGINX-01 | Requires live scaling operation | `docker compose up --scale location-service=3 -d --no-recreate && docker compose exec nginx nginx -s reload`, then verify 3 distinct IPs |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

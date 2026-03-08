---
phase: 6
slug: frontend
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-07
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Browser smoke test via curl + manual browser verification |
| **Config file** | None (static HTML — no test runner config) |
| **Quick run command** | `curl -s -o /dev/null -w "%{http_code}" http://localhost/track/customer-1` |
| **Full suite command** | `curl -f http://localhost/track/customer-1 && curl -f http://localhost/overview` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `curl -f http://localhost/track/customer-1 && curl -f http://localhost/overview`
- **After every plan wave:** Same curl smoke + open browser and confirm map loads
- **Before `/gsd:verify-work`:** Both URLs return 200, map renders, markers animate on first `location_update`
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 6-01-01 | 01 | 0 | FRONT-04 | smoke | `curl -f http://localhost/track/customer-1` | ❌ W0 | ⬜ pending |
| 6-01-02 | 01 | 0 | FRONT-04 | smoke | `curl -f http://localhost/overview` | ❌ W0 | ⬜ pending |
| 6-01-03 | 01 | 1 | FRONT-01 | manual | `curl -f http://localhost/track/customer-1` (HTTP 200) | ❌ W0 | ⬜ pending |
| 6-01-04 | 01 | 1 | FRONT-02 | manual | `curl -f http://localhost/track/customer-1` (HTTP 200) | ❌ W0 | ⬜ pending |
| 6-02-01 | 02 | 1 | FRONT-03 | manual | `curl -f http://localhost/overview` (HTTP 200) | ❌ W0 | ⬜ pending |
| 6-02-02 | 02 | 1 | FRONT-04 | smoke | `curl -f http://localhost/overview` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `nginx/html/track.html` — stubs for FRONT-01, FRONT-02, FRONT-04
- [ ] `nginx/html/overview.html` — stubs for FRONT-03, FRONT-04
- [ ] Nginx `nginx.conf` location blocks for `/track/` and `/overview`
- [ ] `docker-compose.yml` volume mount `./nginx/html:/usr/share/nginx/html:ro`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `/track/{id}` shows Leaflet map with animated, rotating marker | FRONT-01 | Browser rendering — map display and animation not verifiable via CLI | Open browser, navigate to `/track/customer-1`, confirm Leaflet map loads, start simulator, observe marker moves and rotates |
| Stats panel shows driver ID, speed, last update, E2E latency | FRONT-02 | DOM inspection required | Open `/track/customer-1`, start simulator, verify stats panel updates with each `location_update` event |
| `/overview` renders all active drivers with speed-coded markers | FRONT-03 | Browser rendering — multi-marker display and color coding not verifiable via CLI | Open `/overview`, start simulator with multiple drivers, verify all active driver markers appear with color coding |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

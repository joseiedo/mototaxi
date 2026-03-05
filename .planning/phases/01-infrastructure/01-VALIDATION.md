---
phase: 1
slug: infrastructure
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-05
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Shell-based smoke tests (no dedicated test framework for pure infra phase) |
| **Config file** | None — tests are ad-hoc `docker compose` invocations |
| **Quick run command** | `docker compose config --quiet` |
| **Full suite command** | `docker compose up --build -d && sleep 30 && docker compose ps` |
| **Estimated runtime** | ~60 seconds (dominated by image pulls on first run) |

---

## Sampling Rate

- **After every task commit:** Run `docker compose config --quiet`
- **After every plan wave:** Run `docker compose up --build -d && sleep 30 && docker compose ps`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | INFRA-01 | smoke | `docker compose up --build -d; docker compose ps` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | INFRA-02 | unit | `grep -cE "DRIVER_COUNT\|EMIT_INTERVAL_MS\|LOCATION_SERVICE_REPLICAS\|PUSH_SERVER_REPLICAS\|PARTITION_MULTIPLIER\|SECRET_KEY_BASE" .env.example` (expect 6) | ❌ W0 | ⬜ pending |
| 1-01-03 | 01 | 1 | INFRA-03 | smoke | `docker compose exec redpanda rpk topic describe driver.location --brokers localhost:9092` | ❌ W0 | ⬜ pending |
| 1-01-04 | 01 | 1 | INFRA-04 | smoke | `docker compose -f docker-compose.yml -f docker-compose.stress.yml config` | ❌ W0 | ⬜ pending |
| 1-01-05 | 01 | 1 | INFRA-05 | smoke | `docker compose up --build -d && sleep 30 && docker compose ps --format json` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `infra/redpanda-init.sh` — must be created before any smoke test runs
- [ ] `.env` copied from `.env.example` — required for `docker compose up` to work
- [ ] `observability/prometheus.yml` — must exist (even as stub) before Prometheus starts
- [ ] `observability/grafana/provisioning/datasources/prometheus.yaml` — required for auto-provisioning
- [ ] Docker Desktop 4.x+ or Docker Engine with Compose plugin v2.3+ — verify with `docker compose version`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `docker compose up --build` completes on a clean machine | INFRA-01 | Requires clean Docker environment without cached layers | Pull all images fresh: `docker system prune -af && docker compose up --build` |
| Redpanda `driver.location` partition count matches formula | INFRA-03 | Requires running stack to verify | `docker compose exec redpanda rpk topic describe driver.location --brokers localhost:9092 \| grep "Partition Count"` — compare to `PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

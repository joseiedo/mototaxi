---
phase: 2
slug: location-service
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-05
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib) |
| **Config file** | none — standard Go test conventions |
| **Quick run command** | `cd location-service && go test ./...` |
| **Full suite command** | `cd location-service && go test ./... -race -count=1` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd location-service && go test ./...`
- **After every plan wave:** Run `cd location-service && go test ./... -race -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 01 | 0 | LSVC-01 | unit | `cd location-service && go test ./internal/handler/ -run TestPostLocation` | ❌ W0 | ⬜ pending |
| 2-01-02 | 01 | 0 | LSVC-01 | unit | `cd location-service && go test ./internal/handler/ -run TestPostLocationValidation` | ❌ W0 | ⬜ pending |
| 2-01-03 | 01 | 0 | LSVC-01 | unit | `cd location-service && go test ./internal/handler/ -run TestPostLocationRangeCheck` | ❌ W0 | ⬜ pending |
| 2-02-01 | 01 | 0 | LSVC-02 | integration | `cd location-service && go test ./internal/redisstore/ -run TestWriteLocation` | ❌ W0 | ⬜ pending |
| 2-03-01 | 01 | 0 | LSVC-03 | integration | `cd location-service && go test ./internal/handler/ -run TestGetLocation` | ❌ W0 | ⬜ pending |
| 2-03-02 | 01 | 0 | LSVC-03 | unit | `cd location-service && go test ./internal/handler/ -run TestGetLocationNotFound` | ❌ W0 | ⬜ pending |
| 2-04-01 | 01 | 0 | LSVC-04 | unit | `cd location-service && go test ./internal/handler/ -run TestMetricsEndpoint` | ❌ W0 | ⬜ pending |
| 2-04-02 | 01 | 0 | LSVC-04 | unit | `cd location-service && go test ./internal/metrics/ -run TestCounterIncrement` | ❌ W0 | ⬜ pending |
| 2-05-01 | 01 | 1 | LSVC-05 | smoke | `docker build location-service/ && docker inspect --format='{{.Size}}' $(docker images -q location-service:latest)` | manual | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `location-service/go.mod` — module declaration with all dependency versions
- [ ] `location-service/internal/handler/location_test.go` — stubs for LSVC-01, LSVC-03
- [ ] `location-service/internal/redisstore/store_test.go` — stubs for LSVC-02
- [ ] `location-service/internal/metrics/metrics_test.go` — stubs for LSVC-04
- [ ] `go get github.com/alicebob/miniredis/v2` — test-only Redis mock dependency

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Docker image builds to static binary FROM scratch (~5MB) | LSVC-05 | Requires Docker daemon; verifies final binary size not automatable in Go test | `docker build location-service/ -t location-service:test && docker inspect --format='{{.Size}}' location-service:test` — expect < 10MB |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

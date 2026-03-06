---
phase: 4
slug: push-server
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-03-06
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | ExUnit (built into Elixir/Mix) |
| **Config file** | `push-server/test/test_helper.exs` — Wave 0 gap |
| **Quick run command** | `cd push-server && mix test --no-start` |
| **Full suite command** | `cd push-server && mix test` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd push-server && mix test --no-start 2>&1 | tail -5`
- **After every plan wave:** Run `cd push-server && mix test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 4-01-01 | 01 | 0 | PUSH-01 | unit | `mix test test/push_server_web/user_socket_test.exs -x` | Wave 0 | pending |
| 4-02-01 | 02 | 0 | PUSH-02 | unit | `mix test test/push_server_web/customer_channel_test.exs -x` | Wave 0 | pending |
| 4-02-02 | 02 | 0 | PUSH-02 | unit | `mix test test/push_server_web/customer_channel_test.exs -x` | Wave 0 | pending |
| 4-02-03 | 02 | 0 | PUSH-02 | unit | `mix test test/push_server_web/customer_channel_test.exs -x` | Wave 0 | pending |
| 4-02-04 | 02 | 0 | PUSH-02 | unit | `mix test test/push_server_web/customer_channel_test.exs -x` | Wave 0 | pending |
| 4-03-01 | 03 | 1 | PUSH-03 | unit | `mix test test/push_server/pipeline_test.exs -x` | Wave 0 | pending |
| 4-03-02 | 03 | 1 | PUSH-03 | unit | `mix test test/push_server/pipeline_test.exs -x` | Wave 0 | pending |
| 4-04-01 | 04 | 1 | PUSH-04 | integration | `mix test test/push_server/pubsub_test.exs -x` | Wave 0 | pending |
| 4-05-01 | 05 | 2 | PUSH-05 | smoke | `mix test test/push_server/prom_ex_test.exs -x` | Wave 0 | pending |
| 4-05-02 | 05 | 2 | PUSH-05 | integration | `mix test test/push_server_web/metrics_test.exs -x` | Wave 0 | pending |

*Status: pending / green / red / flaky*

---

## PUSH-02 Behavioral Coverage (Plan 02, Task 2)

All four join/3 cases from CONTEXT.md locked decisions are covered by `customer_channel_test.exs` using `Phoenix.ChannelTest` + `Mox` (see 04-02-PLAN.md Task 2):

| Case | Test Description | Assertion |
|------|-----------------|-----------|
| 4-02-01 | Happy path: known customer, driver resolved, position present | `subscribe_and_join` returns `{:ok, _, _}`; `assert_push "location_update"` |
| 4-02-02 | Unknown customer: `customer:{id}:driver` key absent (nil) | returns `{:error, %{reason: "unknown_customer"}}` |
| 4-02-03 | Service unavailable: Redis connection error | returns `{:error, %{reason: "service_unavailable"}}` |
| 4-02-04 | TTL expired: `driver:*:latest` is nil | join returns `{:ok, _, _}`; `refute_push "location_update"` |

`mix test test/push_server_web/customer_channel_test.exs --no-start` exercises all four cases.

---

## Wave 0 Requirements

- [ ] `push-server/mix.exs` — project file with all deps (phoenix, broadway_kafka, phoenix_pubsub_redis, prom_ex, redix, mox)
- [ ] `push-server/config/config.exs` — compile-time defaults
- [ ] `push-server/config/runtime.exs` — env-var driven runtime config
- [ ] `push-server/config/test.exs` — test env config: `config :push_server, :redis_client, PushServer.RedixMock`
- [ ] `push-server/test/test_helper.exs` — ExUnit.start()
- [ ] `push-server/test/support/mocks.ex` — `Mox.defmock(PushServer.RedixMock, for: PushServer.RedixBehaviour)`
- [ ] `push-server/test/support/channel_case.ex` — `PushServerWeb.ChannelCase` wrapping Phoenix.ChannelTest
- [ ] `push-server/test/push_server_web/user_socket_test.exs` — PUSH-01: socket connect + channel route
- [ ] `push-server/test/push_server_web/customer_channel_test.exs` — PUSH-02: all four join/3 behavioral cases
- [ ] `push-server/test/push_server/pipeline_test.exs` — PUSH-03: Broadway handle_message/3 and handle_failed/2
- [ ] `push-server/test/push_server/pubsub_test.exs` — PUSH-04: PubSub cross-process broadcast
- [ ] `push-server/test/push_server/prom_ex_test.exs` — PUSH-05: PromEx module compiles
- [ ] `push-server/test/push_server_web/metrics_test.exs` — PUSH-05: GET /metrics returns 200 with metric names

**Channel testing note:** Use `Phoenix.ChannelTest` helpers (`connect/3`, `subscribe_and_join/3`, `push/3`, `assert_push/3`, `refute_push/2`). Mock Redix via `Mox` — no real Redis or Kafka needed for unit tests. The `:redis_client` compile_env in CustomerChannel is set to `PushServer.RedixMock` in the test environment via `config/test.exs`.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Cross-replica PubSub delivery (replica A receives message consumed by replica B) | PUSH-04 | Requires 2 Docker containers running simultaneously | `docker-compose up --scale push-server=2`, connect wscat to each replica, trigger driver location emit, verify both receive update |
| GET /metrics returns `push_server_delivery_latency_ms` histogram | PUSH-05 | Requires real Kafka message flow end-to-end | Start full stack, send location event, `curl localhost/metrics \| grep delivery_latency` |
| Docker image builds successfully | PUSH-01 | `docker build` takes 2-5+ minutes, violates <15s feedback budget | Run as part of Plan 05 smoke test: `docker compose up --build` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s (docker build moved to manual-only; structural Dockerfile verify is grep-based)
- [x] `nyquist_compliant: true` set in frontmatter

**PUSH-02 coverage confirmed:** All four join/3 behavioral cases (happy path, unknown_customer, service_unavailable, TTL-expired) are implemented as `Phoenix.ChannelTest` + `Mox` tests in `customer_channel_test.exs`. Running `mix test test/push_server_web/customer_channel_test.exs --no-start` exercises all four cases.

**Approval:** pending execution

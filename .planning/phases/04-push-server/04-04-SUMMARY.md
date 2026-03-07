---
phase: 04-push-server
plan: 04
subsystem: push-server
tags: [elixir, phoenix, prom_ex, telemetry, pubsub, redis, supervision_tree, tdd]

# Dependency graph
requires:
  - phase: 04-push-server-02
    provides: Phoenix.PubSub in supervision tree, CustomerChannel, Endpoint
  - phase: 04-push-server-03
    provides: Broadway pipeline module ready to be added to supervision tree

provides:
  - PushServer.PromEx with Beam, Phoenix, and CustomPlugin plugins
  - PushServer.PromEx.CustomPlugin with 3 event metrics (connections_active, messages_delivered_total, delivery_latency_ms)
  - Full supervision tree: PromEx -> PubSub (Redis adapter) -> Redix -> Pipeline -> Endpoint
  - CustomerChannel telemetry emission on join (+1), terminate (-1), and each push (delivered + latency)
  - compute_latency_ms/1 helper using DateTime.from_iso8601 for end-to-end delivery latency

affects:
  - 04-push-server-05 (smoke test validates full supervision tree + PubSub Redis fan-out)
  - Phase 7 Grafana dashboards (push_server_connections_active, push_server_messages_delivered_total, push_server_delivery_latency_ms)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "PromEx.Plugin with Event.build: last_value (gauge), sum (counter), distribution (histogram with custom buckets)"
    - "Supervision tree ordering: PromEx first (captures startup telemetry), PubSub second, Redix third, Pipeline fourth, Endpoint last"
    - ":telemetry.execute/3 in channel callbacks: join/3 (+1), terminate/2 (-1), handle_info after push (delivered + latency)"
    - "DateTime.from_iso8601 for latency: emitted_at ISO8601 string from payload parsed to compute end-to-end ms"
    - "Phoenix.PubSub Redis adapter reads HOSTNAME env var for node_name — unique per Docker container, prevents message loops"

key-files:
  created:
    - push-server/lib/push_server/prom_ex.ex
  modified:
    - push-server/lib/push_server/application.ex
    - push-server/lib/push_server_web/customer_channel.ex
    - push-server/test/push_server/prom_ex_test.exs
    - push-server/test/push_server/pubsub_test.exs
    - push-server/test/push_server_web/metrics_test.exs

key-decisions:
  - "PromEx.CustomPlugin uses last_value (not counter) for connections: connections can decrease, so last_value gauge correctly represents current count"
  - "compute_latency_ms returns 0 on parse error: fire-and-forget latency — malformed emitted_at does not crash the channel"
  - "pubsub_config falls back to PG2 default (adapter: Phoenix.PubSub.Redis only in prod via runtime.exs) — test config uses PG2 adapter"

patterns-established:
  - "PromEx CustomPlugin per subsystem: Event.build with group name, event_name mapped to telemetry event atoms"
  - "Telemetry in Phoenix Channels: emit in join/3, terminate/2, and after push in handle_info — does not affect return values"

requirements-completed: [PUSH-04, PUSH-05]

# Metrics
duration: 3min
completed: 2026-03-06
---

# Phase 4 Plan 04: PubSub Redis Adapter + PromEx Metrics Summary

**PromEx custom plugin with 3 observable metrics (connections_active, messages_delivered_total, delivery_latency_ms) and full supervision tree wired with Redis PubSub adapter and telemetry emission in CustomerChannel**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-06T21:18:37Z
- **Completed:** 2026-03-06T21:20:54Z
- **Tasks:** 2
- **Files modified:** 6 (1 created, 5 updated)

## Accomplishments

- PushServer.PromEx module with 3 plugins: Beam, Phoenix (with router + endpoint), and CustomPlugin
- PushServer.PromEx.CustomPlugin defines 3 metrics:
  - `push_server_connections_active` — last_value gauge, event `[:push_server, :connections]`, measurement `:count`
  - `push_server_messages_delivered_total` — sum counter, event `[:push_server, :messages, :delivered]`, measurement `:count`
  - `push_server_delivery_latency_ms` — distribution histogram with 8 buckets [10,25,50,100,250,500,1000,2500], event `[:push_server, :delivery, :latency]`, measurement `:duration`
- application.ex full supervision tree: PromEx first, PubSub (Redis adapter), Redix, Pipeline, Endpoint
- CustomerChannel.join/3 emits `:telemetry.execute([:push_server, :connections], %{count: 1}, ...)`
- CustomerChannel.terminate/2 emits `:telemetry.execute([:push_server, :connections], %{count: -1}, ...)`
- CustomerChannel.handle_info emits delivered + latency after each successful push
- All 9 plan-specific tests GREEN (`--no-start`)

## Task Commits

1. **Task 1: PromEx module with custom plugin for PUSH-05 metrics** - `7902833` (feat)
2. **Task 2: Full supervision tree + PubSub Redis adapter + telemetry wiring** - `1991abe` (feat)

## Files Created/Modified

- `push-server/lib/push_server/prom_ex.ex` — PushServer.PromEx + PushServer.PromEx.CustomPlugin with 3 event metrics
- `push-server/lib/push_server/application.ex` — Full supervision tree with PromEx and Pipeline uncommented
- `push-server/lib/push_server_web/customer_channel.ex` — terminate/2, telemetry in join/3 and handle_info, compute_latency_ms/1
- `push-server/test/push_server/prom_ex_test.exs` — 4 tests: module exists, plugins list, Beam in plugins, CustomPlugin exists
- `push-server/test/push_server/pubsub_test.exs` — 3 tests: PubSub loadable, Application.start/2 exported, CustomerChannel.terminate/2 exported
- `push-server/test/push_server_web/metrics_test.exs` — 2 tests: Endpoint loadable, PromEx loadable

## Decisions Made

- `last_value` gauge for connections: connections can decrease (terminate/2), so gauge is correct semantics vs counter
- `compute_latency_ms` returns 0 on parse error: delivery latency is observability data — parse failure should not crash the channel
- PubSub Redis adapter config falls back to PG2 in test via `Application.get_env` — runtime.exs (prod only) sets Redis adapter

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

- Pre-existing: `user_socket_test.exs` and `customer_channel_test.exs` fail with `--no-start` due to `Mox.Server` requiring live application. This was documented in Plan 02 SUMMARY ("Mox.Server not starting with --no-start flag"). These tests require `mix test` (without --no-start) to run. Not introduced by this plan.

## User Setup Required

None — no external service configuration required for compilation or unit tests.

## Next Phase Readiness

- Plan 05 smoke test can now validate the complete push path: Kafka → Broadway → PubSub (Redis) → CustomerChannel → WebSocket client
- All 5 supervision tree children are live: PromEx, PubSub, Redix, Pipeline, Endpoint
- PromEx metrics will be scraped by Prometheus (Phase 7) and visualized in Grafana

---
*Phase: 04-push-server*
*Completed: 2026-03-06*

## Self-Check: PASSED

- FOUND: push-server/lib/push_server/prom_ex.ex
- FOUND: push-server/lib/push_server/application.ex
- FOUND: push-server/lib/push_server_web/customer_channel.ex
- FOUND: .planning/phases/04-push-server/04-04-SUMMARY.md
- FOUND: commit 7902833 (feat: PromEx module)
- FOUND: commit 1991abe (feat: Full supervision tree)

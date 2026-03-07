---
phase: 04-push-server
plan: 03
subsystem: api
tags: [elixir, broadway, broadway_kafka, phoenix_pubsub, kafka, elixir_tdd]

# Dependency graph
requires:
  - phase: 04-push-server-01
    provides: Mix project scaffold, application.ex supervision tree stub
  - phase: 04-push-server-02
    provides: Phoenix.PubSub in supervision tree, CustomerChannel, Endpoint

provides:
  - Broadway pipeline consuming driver.location Kafka topic
  - handle_message/3 — JSON decode + Phoenix.PubSub broadcast to driver:{id}
  - handle_failed/2 — logs at :warning and returns messages unchanged
  - Runtime-computed processor concurrency from env vars

affects:
  - 04-push-server-04 (uncomments Pipeline child in application.ex)
  - 04-push-server-05 (smoke test verifies end-to-end PubSub fan-out)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Broadway pipeline: BroadwayKafka producer + handle_message/3 + handle_failed/2"
    - "hosts in keyword list format [redpanda: 9092] — not string — per BroadwayKafka requirement"
    - "Processor concurrency = PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER read at runtime from env"
    - "broadcast! with %Phoenix.Socket.Broadcast{event: location_update} to driver:{id} topic"

key-files:
  created:
    - push-server/lib/push_server/pipeline.ex
  modified:
    - push-server/test/push_server/pipeline_test.exs

key-decisions:
  - "kafka_hosts read from Application.get_env(:push_server, :kafka_hosts, [redpanda: 9092]) so config.exs can override for tests without env var tricks"
  - "handle_failed/2 returns messages unchanged — BroadwayKafka always commits offsets regardless of ack/discard semantics"

patterns-established:
  - "handle_message/3 uses with for dual-error paths (JSON decode failure + missing driver_id) — single else clause marks failed"
  - "Pipeline unit tests use Code.ensure_loaded? + function_exported? pattern (no live Broadway process needed)"

requirements-completed: [PUSH-03]

# Metrics
duration: 5min
completed: 2026-03-06
---

# Phase 4 Plan 03: Broadway Pipeline Summary

**Broadway pipeline consuming driver.location Kafka topic via BroadwayKafka, broadcasting JSON-decoded location events as Phoenix.Socket.Broadcast to PushServer.PubSub driver:{id} topics**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-06T21:15:00Z
- **Completed:** 2026-03-06T21:16:40Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Broadway pipeline module with BroadwayKafka producer, handle_message/3, and handle_failed/2
- handle_message/3 broadcasts %Phoenix.Socket.Broadcast{event: "location_update"} to PushServer.PubSub on success; marks message failed on JSON decode error or missing driver_id — does not crash
- handle_failed/2 logs at :warning with raw data and status, returns messages unchanged
- Processor concurrency computed at runtime from PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER environment variables
- All 4 pipeline_test.exs tests pass GREEN including handle_failed/2 behavior test

## Task Commits

1. **Task 1: Broadway pipeline — handle_message/3 with JSON decode and PubSub broadcast** - `2924640` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified
- `push-server/lib/push_server/pipeline.ex` - Broadway pipeline: start_link/1, handle_message/3, handle_failed/2
- `push-server/test/push_server/pipeline_test.exs` - 4 tests: module exists, exports, handle_failed/2 behavior

## Decisions Made
- `kafka_hosts` read from `Application.get_env(:push_server, :kafka_hosts, [redpanda: 9092])` so test config can override without env var tricks
- `handle_failed/2` returns messages unchanged — BroadwayKafka always commits Kafka offsets regardless

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Pipeline module is ready; Plan 04 (application.ex wiring) can uncomment `PushServer.Pipeline` child and add `PushServer.PromEx`
- Plan 05 smoke test will validate end-to-end flow: Kafka message -> handle_message/3 -> PubSub -> CustomerChannel

---
*Phase: 04-push-server*
*Completed: 2026-03-06*

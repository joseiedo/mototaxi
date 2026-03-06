# Phase 4: Push Server - Context

**Gathered:** 2026-03-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Elixir/Phoenix service that holds customer WebSocket connections via Phoenix Channels, resolves assigned drivers from Redis on join, consumes the Kafka `driver.location` topic with backpressure via Broadway, fans location updates to connected clients via Phoenix.PubSub + Redis adapter across all replicas, and exposes PromEx metrics. No frontend, no Nginx routing — this phase delivers the push-server binary and Docker image only.

</domain>

<decisions>
## Implementation Decisions

### Channel message format
- Phoenix event name: `"location_update"` — used for both the initial join push (PUSH-02) and ongoing Broadway-driven updates (PUSH-03). One event name, one frontend handler.
- Payload fields: pass-through location payload plus `driver_id` added:
  ```json
  {
    "driver_id": "driver-1",
    "lat": -23.52,
    "lng": -46.63,
    "bearing": 142.5,
    "speed_kmh": 34.2,
    "emitted_at": "2026-03-06T17:40:02Z"
  }
  ```
- `driver_id` is included so the `/overview` page (Phase 6) can identify which driver moved without a separate lookup.
- No transformation beyond adding `driver_id` — all other fields pass through unchanged from the Kafka message.

### Join error handling
- Unknown customer ID (no `customer:{customer_id}:driver` Redis key): return `{:error, %{reason: "unknown_customer"}}` — reject the join cleanly.
- Valid customer ID but `driver:{id}:latest` has expired (30s TTL): join succeeds (`:ok`), skip the initial push, wait for the next Kafka-driven `location_update`. No stale data, no nil coordinates.
- Redis unreachable during join: return `{:error, %{reason: "service_unavailable"}}` — reject the join.
- All join errors logged at `Logger.warning` with `customer_id` and the failure reason.

### Broadway pipeline shape
- Single Broadway pipeline consuming all assigned Kafka partitions (no per-partition pipeline split).
- Processor concurrency: read `PUSH_SERVER_REPLICAS` and `PARTITION_MULTIPLIER` from env at startup; set `concurrency: replicas * multiplier`. Processor count matches partition count — makes `PARTITION_MULTIPLIER` tuning directly observable in Grafana (Experiment 5).
- No batching: `batch_size: 1`. Each location update is independent and should be broadcast immediately to minimize latency.
- Failed messages (JSON decode error, PubSub crash): `handle_failed/2` logs at `:warning` with the raw message payload, then acks (discards). Location updates are ephemeral — one missed update is acceptable; retrying poison-pill messages is not.

### Docker release packaging
- Multi-stage Dockerfile: builder stage uses `elixir:1.18-alpine`, runs `MIX_ENV=prod mix release`.
- Final stage: `alpine:3.21` with `apk add --no-cache libstdc++ openssl ncurses-libs`. BEAM runtime bundled in the OTP release.
- Phoenix endpoint port: `4000` inside the container. Nginx (Phase 5) will proxy WebSocket connections from `:80` to `push-server:4000`.
- `SECRET_KEY_BASE` injected via environment variable (already in `.env.example`).
- `ulimits.nofile: 65536` in docker-compose for high-connection stress tests (PROJECT.md constraint).

### Claude's Discretion
- Elixir application/OTP supervision tree layout (Application, Supervisor hierarchy)
- Phoenix Endpoint and Router configuration details
- BroadwayKafka vs. brod adapter choice for Broadway Kafka producer
- Redix vs. other Redis client for Phoenix.PubSub Redis adapter
- PromEx dashboard vs. custom metrics module structure
- Exact `handle_message/3` implementation details
- `mix.exs` dependency versions (use latest compatible)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `simulator/internal/emitter/emitter.go` `locationPayload`: the JSON shape arriving in Kafka messages — `driver_id`, `lat`, `lng`, `bearing`, `speed_kmh`, `emitted_at` (RFC3339). Broadway consumers must decode this shape.
- Redis assignment keys from Phase 3: `customer:customer-{N}:driver` → `"driver-{N}"` (string); `driver:{id}:latest` → JSON position blob with 30s TTL (written by location-service Phase 2).
- `.env.example`: `PUSH_SERVER_REPLICAS=2`, `PARTITION_MULTIPLIER=2`, `SECRET_KEY_BASE` already documented. Broadway processor count formula: `PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER`.

### Established Patterns
- Separate `mix.exs` per service — push-server gets its own Mix project under `push-server/`.
- Multi-stage Dockerfile with minimal final image — Go services use FROM scratch; Elixir equivalent is `alpine:3.21` + OTP release.
- Env-var-driven config — all tunable parameters via environment variables, no hardcoded values.
- Kafka topic: `driver.location`, broker: `redpanda:9092` on the `mototaxi` Docker network.
- Redis at `redis:6379` on the `mototaxi` Docker network.
- `platform: linux/amd64` needed only for kafka-exporter and cAdvisor (Phase 1 decision) — push-server is native ARM64.

### Integration Points
- `docker-compose.yml` — Phase 4 adds the `push-server` service block with `depends_on: redis (healthy), redpanda (healthy), redpanda-init (completed)`.
- Nginx (Phase 5) will route `/socket` to `push-server:4000` with ip_hash stickiness. Phoenix endpoint must be at port 4000.
- Frontend (Phase 6) will open a Phoenix Channel JS client to `customer:{customer_id}` and listen for `"location_update"` events.
- Broadway consumes from `driver.location` topic; message key = `driver_id` (string), value = location JSON.

</code_context>

<specifics>
## Specific Ideas

- Broadway processor count is derived from env at runtime: `System.get_env("PUSH_SERVER_REPLICAS") |> String.to_integer() * System.get_env("PARTITION_MULTIPLIER") |> String.to_integer()`. This makes Experiment 5 (partition multiplier effect) directly demonstrable by changing `.env` and observing Grafana.
- `emitted_at` in the broadcast payload is the original RFC3339 timestamp from the simulator — the frontend computes `Date.now() - emitted_at` for end-to-end latency display (PUSH-05 delivery_latency_ms histogram should do the same server-side).
- PubSub topic for broadcasting: `"driver:#{driver_id}"` — the channel subscribes to this topic on join so it receives updates when Broadway broadcasts for that driver.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 04-push-server*
*Context gathered: 2026-03-06*

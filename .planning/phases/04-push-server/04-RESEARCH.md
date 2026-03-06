# Phase 4: Push Server - Research

**Researched:** 2026-03-06
**Domain:** Elixir/Phoenix Channels, Broadway/Kafka, Phoenix.PubSub Redis adapter, PromEx metrics
**Confidence:** HIGH

## Summary

The Push Server is an Elixir/Phoenix application that manages WebSocket connections for customers via Phoenix Channels, consumes the `driver.location` Kafka topic with Broadway, fans updates via Phoenix.PubSub (Redis adapter for cross-replica delivery), and exposes Prometheus metrics via PromEx. The BEAM VM is the right runtime for this service: 10k+ lightweight processes, one per channel connection, with built-in cross-process messaging.

The architecture is well-understood by the Elixir ecosystem. Phoenix Channels handle the WebSocket transport layer, Broadway handles backpressure-driven Kafka consumption, and Phoenix.PubSub with the Redis adapter decouples message consumption from message delivery across replicas. The three concerns (WebSocket sessions, Kafka consumption, cross-replica fanout) remain fully independent, which is the core design insight.

The main non-obvious area is the PubSub subscription pattern: the channel process subscribes to `"driver:{driver_id}"` via `MyApp.Endpoint.subscribe/1` inside `join/3`, then Broadway broadcasts to that topic via `Phoenix.PubSub.broadcast!/3`. The channel's `handle_info/2` receives the broadcast and calls `push/3` to deliver it to the specific WebSocket client.

**Primary recommendation:** Use `phoenix_pubsub_redis ~> 3.0` with `Redix` under the hood, `broadway_kafka ~> 0.4` for Kafka consumption, `prom_ex ~> 1.11` for metrics. The pattern is: Broadway broadcasts to PubSub topic → Channel process receives via `handle_info/2` → `push/3` delivers to client.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Channel message format:**
- Phoenix event name: `"location_update"` for both join push (PUSH-02) and Broadway-driven updates (PUSH-03)
- Payload: pass-through location fields plus `driver_id` added:
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
- No transformation beyond adding `driver_id`

**Join error handling:**
- Unknown customer ID (no `customer:{customer_id}:driver` key): `{:error, %{reason: "unknown_customer"}}`
- Valid customer, expired `driver:{id}:latest` (30s TTL): join succeeds, skip initial push, wait for Kafka
- Redis unreachable during join: `{:error, %{reason: "service_unavailable"}}`
- All errors logged at `Logger.warning` with `customer_id` and reason

**Broadway pipeline shape:**
- Single pipeline, no per-partition split
- Processor concurrency: `PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER` from env at startup
- No batching: `batch_size: 1` (each update independent, minimize latency)
- `handle_failed/2`: log at `:warning` with raw payload, ack (discard). Location updates are ephemeral.

**Docker release packaging:**
- Builder: `elixir:1.18-alpine`, `MIX_ENV=prod mix release`
- Final: `alpine:3.21` + `apk add --no-cache libstdc++ openssl ncurses-libs`
- Port: `4000` inside container
- `SECRET_KEY_BASE` via env var
- `ulimits.nofile: 65536` in docker-compose

### Claude's Discretion
- Elixir application/OTP supervision tree layout (Application, Supervisor hierarchy)
- Phoenix Endpoint and Router configuration details
- BroadwayKafka vs. brod adapter choice for Broadway Kafka producer
- Redix vs. other Redis client for Phoenix.PubSub Redis adapter
- PromEx dashboard vs. custom metrics module structure
- Exact `handle_message/3` implementation details
- `mix.exs` dependency versions (use latest compatible)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PUSH-01 | Push Server accepts WebSocket connections and joins clients to `customer:{customer_id}` Phoenix Channel | Phoenix Channel + UserSocket setup; `channel "customer:*"` route |
| PUSH-02 | On channel join, resolve assigned driver from Redis and immediately push current position | Redix GET for `customer:{id}:driver`, then GET `driver:{id}:latest`; use `push/3` in `join/3` after `{:ok, socket}` return |
| PUSH-03 | Broadway consumes `driver.location` with backpressure; broadcasts via Phoenix.PubSub to `driver:{driver_id}` | BroadwayKafka producer + `handle_message/3` + `Phoenix.PubSub.broadcast!/3` |
| PUSH-04 | Phoenix.PubSub uses Redis adapter for cross-replica broadcast | `phoenix_pubsub_redis ~> 3.0` in supervision tree with `host: "redis"`, `node_name` from env |
| PUSH-05 | PromEx metrics: BEAM + Phoenix channel events + custom gauge/counter/histogram | `prom_ex ~> 1.11`, `PromEx.Plugins.Beam`, `PromEx.Plugins.Phoenix`, custom plugin for connections/delivered/latency |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| phoenix | ~> 1.7 | Channels, Endpoint, Router | Official framework; Channels are battle-tested for 10k+ WebSocket connections |
| phoenix_pubsub | ~> 2.1 | Pub/Sub fanout within/across nodes | Bundled with Phoenix; decouples Kafka consumer from channel processes |
| phoenix_pubsub_redis | ~> 3.0 | Redis adapter for cross-replica PubSub | Only official adapter for cross-node Phoenix.PubSub without Erlang cluster setup |
| broadway | ~> 1.2 | Concurrent, backpressure-aware message pipeline | Official dashbit library; handles GenStage backpressure, concurrency, ack semantics |
| broadway_kafka | ~> 0.4 | BroadwayKafka Kafka producer for Broadway | Official dashbit connector; uses `brod` under the hood; Kafka-compatible with Redpanda |
| redix | ~> 1.5 | Redis client for channel join lookups | Pure Elixir, fast, used by `phoenix_pubsub_redis` internally; direct use for join logic |
| prom_ex | ~> 1.11 | Prometheus metrics + BEAM metrics | Purpose-built for Elixir; provides BEAM + Phoenix plugins + custom metric framework |
| jason | ~> 1.4 | JSON decode for Kafka message payloads | Default JSON library in Phoenix ecosystem; fast, pure Elixir |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| plug_cowboy | ~> 2.7 | HTTP server adapter | Required for Phoenix Endpoint without LiveView |
| telemetry | ~> 1.2 | Telemetry events (PromEx dependency) | Implicit; PromEx wires into telemetry events automatically |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| broadway_kafka | KafkaEx | BroadwayKafka is maintained by dashbit (Broadway authors); KafkaEx is community-maintained and less integrated |
| phoenix_pubsub_redis | Erlang distribution (libcluster) | Erlang clustering requires network config; Redis adapter is simpler for Docker Compose |
| prom_ex | :telemetry_metrics + prometheus_ex | PromEx provides built-in BEAM/Phoenix plugins; custom prometheus_ex requires more boilerplate |
| redix (direct) | Cachex, Nebulex | Redix is the standard raw Redis client; higher-level abstractions add complexity without benefit here |

**Installation:**
```bash
mix phx.new push_server --no-html --no-ecto --no-mailer --no-dashboard --no-assets --no-gettext
cd push_server
# Then add deps to mix.exs and mix deps.get
```

Minimal `mix.exs` deps block:
```elixir
defp deps do
  [
    {:phoenix, "~> 1.7"},
    {:phoenix_pubsub, "~> 2.1"},
    {:phoenix_pubsub_redis, "~> 3.0"},
    {:plug_cowboy, "~> 2.7"},
    {:broadway, "~> 1.2"},
    {:broadway_kafka, "~> 0.4"},
    {:redix, "~> 1.5"},
    {:jason, "~> 1.4"},
    {:prom_ex, "~> 1.11"}
  ]
end
```

---

## Architecture Patterns

### Recommended Project Structure
```
push-server/
├── mix.exs
├── config/
│   ├── config.exs          # compile-time defaults
│   └── runtime.exs         # env-var driven runtime config (SECRET_KEY_BASE, Redis, Kafka)
├── lib/
│   push_server/
│   ├── application.ex      # OTP Application, supervision tree
│   ├── endpoint.ex         # Phoenix.Endpoint (port 4000, PubSub, socket mount)
│   ├── router.ex           # Phoenix.Router (minimal — just /metrics via PromEx.Plug)
│   ├── user_socket.ex      # Phoenix.Socket — channel "customer:*" route
│   ├── customer_channel.ex # Phoenix.Channel — join/3, handle_info/2
│   ├── pipeline.ex         # Broadway pipeline — BroadwayKafka producer, handle_message/3
│   └── prom_ex.ex          # PromEx module with plugins + custom plugin
├── Dockerfile
└── .dockerignore
```

### Pattern 1: Supervision Tree (OTP Application)
**What:** Start order matters — PubSub must be up before Endpoint; PromEx must be first.
**When to use:** Always — this is the fixed OTP startup sequence.
**Example:**
```elixir
# lib/push_server/application.ex
defmodule PushServer.Application do
  use Application

  def start(_type, _args) do
    children = [
      # 1. PromEx must be first to capture startup telemetry
      PushServer.PromEx,
      # 2. PubSub (with Redis adapter — configured in runtime.exs)
      {Phoenix.PubSub, Application.get_env(:push_server, :pubsub)},
      # 3. Redix connection pool for channel join lookups
      {Redix, {Application.get_env(:push_server, :redis_url), name: :redix}},
      # 4. Broadway pipeline (starts consuming Kafka immediately)
      PushServer.Pipeline,
      # 5. Phoenix Endpoint (starts accepting connections last)
      PushServerWeb.Endpoint
    ]
    Supervisor.start_link(children, strategy: :one_for_one, name: PushServer.Supervisor)
  end
end
```

### Pattern 2: Phoenix Channel (UserSocket + CustomerChannel)
**What:** WebSocket connections arrive at the Endpoint → UserSocket dispatches to CustomerChannel → join/3 runs Redis lookups and subscribes the process to the driver's PubSub topic.
**When to use:** Core of PUSH-01, PUSH-02.

```elixir
# lib/push_server/user_socket.ex
defmodule PushServerWeb.UserSocket do
  use Phoenix.Socket
  channel "customer:*", PushServerWeb.CustomerChannel
  def connect(_params, socket, _connect_info), do: {:ok, socket}
  def id(_socket), do: nil
end
```

```elixir
# lib/push_server/customer_channel.ex
defmodule PushServerWeb.CustomerChannel do
  use Phoenix.Channel
  require Logger

  def join("customer:" <> customer_id, _params, socket) do
    case resolve_driver(customer_id) do
      {:ok, driver_id} ->
        # Subscribe this process to driver PubSub topic
        PushServerWeb.Endpoint.subscribe("driver:#{driver_id}")
        socket = assign(socket, :driver_id, driver_id)
        # Send initial position (PUSH-02) — fire after join ack
        send(self(), {:push_initial, driver_id})
        {:ok, assign(socket, :customer_id, customer_id)}

      {:error, reason} ->
        Logger.warning("join rejected customer_id=#{customer_id} reason=#{reason}")
        {:error, %{reason: reason}}
    end
  end

  # Handle initial position push (avoids blocking join)
  def handle_info({:push_initial, driver_id}, socket) do
    case get_latest_position(driver_id) do
      {:ok, payload} -> push(socket, "location_update", payload)
      {:skip} -> :ok   # TTL expired — wait for next Kafka message
    end
    {:noreply, socket}
  end

  # Handle Broadway → PubSub → Channel delivery (PUSH-03)
  def handle_info(%Phoenix.Socket.Broadcast{event: event, payload: payload}, socket) do
    push(socket, event, payload)
    {:noreply, socket}
  end

  # Redis lookup: customer:{id}:driver -> driver_id
  defp resolve_driver(customer_id) do
    case Redix.command(:redix, ["GET", "customer:#{customer_id}:driver"]) do
      {:ok, nil} -> {:error, "unknown_customer"}
      {:ok, driver_id} -> {:ok, driver_id}
      {:error, _} -> {:error, "service_unavailable"}
    end
  end

  # Redis lookup: driver:{id}:latest -> JSON position
  defp get_latest_position(driver_id) do
    case Redix.command(:redix, ["GET", "driver:#{driver_id}:latest"]) do
      {:ok, nil} -> {:skip}  # TTL expired
      {:ok, json} -> {:ok, Jason.decode!(json)}
      {:error, _} -> {:skip}
    end
  end
end
```

### Pattern 3: Broadway Pipeline (Kafka Consumer)
**What:** Broadway pulls from Kafka with backpressure. `handle_message/3` decodes JSON, adds `driver_id`, and broadcasts to Phoenix.PubSub. No batching (batch_size: 1 implicit by no batchers config).
**When to use:** Core of PUSH-03.

```elixir
# lib/push_server/pipeline.ex
defmodule PushServer.Pipeline do
  use Broadway
  require Logger

  def start_link(_opts) do
    replicas = System.get_env("PUSH_SERVER_REPLICAS", "2") |> String.to_integer()
    multiplier = System.get_env("PARTITION_MULTIPLIER", "2") |> String.to_integer()
    concurrency = replicas * multiplier

    Broadway.start_link(__MODULE__,
      name: __MODULE__,
      producer: [
        module: {BroadwayKafka.Producer, [
          hosts: [redpanda: 9092],
          group_id: "push_server",
          topics: ["driver.location"]
        ]},
        concurrency: 1
      ],
      processors: [
        default: [concurrency: concurrency]
      ]
    )
  end

  def handle_message(_processor, message, _context) do
    case Jason.decode(message.data) do
      {:ok, payload} ->
        driver_id = Map.fetch!(payload, "driver_id")
        broadcast_payload = Map.put(payload, "driver_id", driver_id)
        Phoenix.PubSub.broadcast!(
          PushServer.PubSub,
          "driver:#{driver_id}",
          %Phoenix.Socket.Broadcast{
            topic: "driver:#{driver_id}",
            event: "location_update",
            payload: broadcast_payload
          }
        )
      {:error, _} ->
        Broadway.Message.failed(message, "json_decode_error")
    end
    message
  end

  def handle_failed(messages, _context) do
    Enum.each(messages, fn msg ->
      Logger.warning("broadway handle_failed data=#{inspect(msg.data)} status=#{inspect(msg.status)}")
    end)
    messages  # ack (discard) — location updates are ephemeral
  end
end
```

### Pattern 4: PromEx Custom Metrics Module
**What:** A single PromEx module that includes built-in plugins plus a custom plugin for the three PUSH-05 metrics.
**When to use:** PUSH-05.

```elixir
# lib/push_server/prom_ex.ex
defmodule PushServer.PromEx do
  use PromEx, otp_app: :push_server

  @impl true
  def plugins do
    [
      PromEx.Plugins.Beam,
      {PromEx.Plugins.Phoenix, router: PushServerWeb.Router, endpoint: PushServerWeb.Endpoint},
      PushServer.PromEx.CustomPlugin
    ]
  end
end

defmodule PushServer.PromEx.CustomPlugin do
  use PromEx.Plugin

  @connections_event [:push_server, :connections]
  @delivered_event   [:push_server, :messages, :delivered]
  @latency_event     [:push_server, :delivery, :latency]

  @impl true
  def event_metrics(_opts) do
    Event.build(:push_server_custom_metrics, [
      # Gauge: active connections (last_value)
      last_value(
        [:push_server, :connections, :active],
        event_name: @connections_event,
        measurement: :count,
        description: "Number of active push server WebSocket connections"
      ),
      # Counter: messages delivered total (sum)
      sum(
        [:push_server, :messages, :delivered, :total],
        event_name: @delivered_event,
        measurement: :count,
        description: "Total number of location_update messages delivered to clients"
      ),
      # Histogram: delivery latency ms (distribution)
      distribution(
        [:push_server, :delivery, :latency, :milliseconds],
        event_name: @latency_event,
        measurement: :duration,
        description: "End-to-end delivery latency from emitted_at to push (ms)",
        reporter_options: [buckets: [10, 25, 50, 100, 250, 500, 1000, 2500]]
      )
    ])
  end
end
```

Emit telemetry from channel code:
```elixir
# On successful push in handle_info:
:telemetry.execute([:push_server, :messages, :delivered], %{count: 1}, %{})
emitted_at = Map.get(payload, "emitted_at")
latency_ms = compute_latency_ms(emitted_at)
:telemetry.execute([:push_server, :delivery, :latency], %{duration: latency_ms}, %{})

# On join/leave (track connections):
:telemetry.execute([:push_server, :connections], %{count: 1}, %{})  # join
:telemetry.execute([:push_server, :connections], %{count: -1}, %{}) # terminate
```

### Pattern 5: Phoenix.PubSub Redis Adapter Configuration
**What:** Replace default PG2 adapter with Redis adapter for cross-replica broadcast.
**When to use:** PUSH-04 — required for multi-replica push-server.

```elixir
# config/runtime.exs
config :push_server, :pubsub,
  name: PushServer.PubSub,
  adapter: Phoenix.PubSub.Redis,
  host: System.get_env("REDIS_HOST", "redis"),
  port: String.to_integer(System.get_env("REDIS_PORT", "6379")),
  node_name: System.get_env("NODE_NAME", "push_server_1")

# endpoint.ex - reference the PubSub name
config :push_server, PushServerWeb.Endpoint,
  pubsub_server: PushServer.PubSub
```

**Critical:** `node_name` must be unique per replica in docker-compose. Use `hostname` or inject via env:
```yaml
# docker-compose.yml push-server service
environment:
  NODE_NAME: "push_server_${HOSTNAME}"
```

### Pattern 6: Endpoint and Router for /metrics
**What:** PromEx exposes `/metrics` via a Plug. The router is minimal — no HTML, no LiveView.

```elixir
# lib/push_server_web/endpoint.ex
defmodule PushServerWeb.Endpoint do
  use Phoenix.Endpoint, otp_app: :push_server

  socket "/socket", PushServerWeb.UserSocket,
    websocket: [timeout: 45_000],
    longpoll: false

  plug PromEx.Plug, prom_ex_module: PushServer.PromEx
  plug PushServerWeb.Router
end

# lib/push_server_web/router.ex
defmodule PushServerWeb.Router do
  use Phoenix.Router
  # minimal — /metrics handled by PromEx.Plug directly on Endpoint
end
```

### Anti-Patterns to Avoid
- **Broadcasting from the channel process:** Broadway must broadcast, not the channel. The channel only subscribes and pushes to its own socket.
- **Blocking join/3 with slow Redis calls:** Use `send(self(), {:push_initial, driver_id})` to defer the initial push after join ack. Do not call `push/3` directly inside `join/3` before returning `{:ok, socket}`.
- **Hardcoded processor concurrency:** Always read `PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER` at runtime from env, not compile time.
- **Using `Phoenix.PubSub.broadcast/3` (3-arity) in Broadway:** Use `broadcast!/3` with the PubSub server name (atom), not the endpoint module.
- **Missing node_name in PubSub config:** `phoenix_pubsub_redis` requires a unique `node_name` per replica. Without it, all replicas share the same name and messages may loop.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Kafka backpressure | Custom GenStage producer | Broadway + BroadwayKafka | Consumer group offset management, partition assignment, rebalancing are handled by brod |
| Cross-replica fanout | Manual Redis pub/sub subscription | Phoenix.PubSub Redis adapter | PubSub adapter handles connection pooling, error recovery, serialization |
| WebSocket transport | Custom WebSocket handler | Phoenix Channels | Phoenix handles upgrade, heartbeats, reconnection, serialization protocol |
| Prometheus metrics | Custom `:telemetry_metrics` wiring | PromEx | BEAM and Phoenix plugins come free; custom plugin framework is documented |
| JSON decode with error handling | Hand-rolled binary parsing | Jason | Jason handles RFC8259 correctly; edge cases in binary strings, unicode escaping |
| Redis connection pooling | Manual connection state | Redix (single named connection) | Redix reconnects automatically; for this scale a single connection is sufficient |

**Key insight:** Every component in this stack has a production-grade Elixir library. The value is in wiring them correctly, not in reimplementing their internals.

---

## Common Pitfalls

### Pitfall 1: PubSub Node Name Collision
**What goes wrong:** Two push-server replicas use the same `node_name` in the Redis PubSub config. Both see each other's broadcasts as external but the Redis pub/sub channel deduplication fails, causing message loops or missed messages.
**Why it happens:** `node_name` defaults to the Erlang `--sname` flag which is not set in containerized deployments without explicit Erlang clustering.
**How to avoid:** Inject `NODE_NAME` as a unique env var per replica in docker-compose (e.g., using the container hostname or a replica index). Read it in `config/runtime.exs`.
**Warning signs:** Customers receive duplicate messages; or updates stop entirely after second replica joins.

### Pitfall 2: Blocking join/3 Before {:ok, socket}
**What goes wrong:** Calling `push/3` inside `join/3` before returning `{:ok, socket}` fails silently — the socket is not yet confirmed so the push is dropped.
**Why it happens:** Phoenix Channels require the join handshake to complete before any messages can be pushed to the client.
**How to avoid:** Use `send(self(), {:push_initial, driver_id})` after building the socket assignment. Handle in `handle_info/2` and call `push/3` there.
**Warning signs:** Initial position is never received by client even though Redis has data.

### Pitfall 3: Broadway `handle_failed/2` Not Acking
**What goes wrong:** Returning messages from `handle_failed/2` without calling `Broadway.Message.ack_failed/1` keeps the pipeline stalled. The pipeline backs up and stops consuming.
**Why it happens:** Broadway treats returned messages from `handle_failed/2` as needing acknowledgment. Returning them un-acked causes the pipeline to wait.
**How to avoid:** In `handle_failed/2`, just log and return the messages as-is. BroadwayKafka's acknowledgment is automatic — it always commits offsets. The message is effectively discarded after `handle_failed/2`.
**Warning signs:** Kafka consumer lag grows; pipeline stops processing after first bad message.

### Pitfall 4: Redix vs phoenix_pubsub_redis Confusion
**What goes wrong:** Both `phoenix_pubsub_redis` and direct `Redix` are in deps. `phoenix_pubsub_redis` uses its own internal Redix connections. Adding a second Redix pool for channel lookups must use a different name (`:redix`) to avoid conflicts.
**Why it happens:** Both libraries use Redix under the hood but register processes differently.
**How to avoid:** Name the direct Redix connection `:redix` and reference it by name in `Redix.command(:redix, ...)`. The PubSub adapter manages its own connections separately.
**Warning signs:** `{:error, :already_started}` during startup; Redis commands routing to wrong connection.

### Pitfall 5: BroadwayKafka Hosts Format
**What goes wrong:** Kafka host configured as a string `"redpanda:9092"` instead of the keyword list `[redpanda: 9092]`. Broadway fails to start with a cryptic brod error.
**Why it happens:** `brod` (the underlying library) expects `{hostname, port}` tuples, which BroadwayKafka exposes as keyword list syntax.
**How to avoid:** Always use keyword list: `hosts: [redpanda: 9092]` not `hosts: "redpanda:9092"`.
**Warning signs:** Broadway fails to start; brod logs show connection refusal to port 0.

### Pitfall 6: Alpine DNS in OTP Releases
**What goes wrong:** The OTP release in the Alpine final image cannot resolve Docker network hostnames (`redis`, `redpanda`) because Alpine 3.14 and earlier had musl libc DNS issues with BEAM's built-in resolver.
**Why it happens:** BEAM uses its own DNS resolver that bypasses the OS resolver on some Alpine versions.
**How to avoid:** Use `alpine:3.21` (locked in CONTEXT.md). Alpine 3.18+ resolved the musl DNS issue. Ensure `/etc/hosts` or `/etc/resolv.conf` are present in the container (they are by default in Docker).
**Warning signs:** `{:nxdomain}` errors connecting to `redis` or `redpanda` despite containers being healthy.

### Pitfall 7: SECRET_KEY_BASE in Compiled Release
**What goes wrong:** `SECRET_KEY_BASE` is set in `config/config.exs` (compile-time) and baked into the release. Changing it requires a full rebuild.
**Why it happens:** Elixir distinguishes compile-time (`config/config.exs`) from runtime config (`config/runtime.exs`).
**How to avoid:** Put `secret_key_base: System.get_env("SECRET_KEY_BASE")` only in `config/runtime.exs`. The Endpoint requires it at startup, not compile time.
**Warning signs:** `invalid secret_key_base` errors only appear at runtime after env var is changed.

---

## Code Examples

Verified patterns from official sources:

### Elixir Runtime Config Pattern (config/runtime.exs)
```elixir
# Source: https://hexdocs.pm/phoenix/releases.html
import Config

if config_env() == :prod do
  config :push_server, PushServerWeb.Endpoint,
    secret_key_base: System.fetch_env!("SECRET_KEY_BASE"),
    http: [port: String.to_integer(System.get_env("PORT", "4000"))]

  config :push_server, :pubsub,
    name: PushServer.PubSub,
    adapter: Phoenix.PubSub.Redis,
    host: System.get_env("REDIS_HOST", "redis"),
    port: String.to_integer(System.get_env("REDIS_PORT", "6379")),
    node_name: System.fetch_env!("NODE_NAME")

  config :push_server, :redis_url,
    "redis://#{System.get_env("REDIS_HOST", "redis")}:#{System.get_env("REDIS_PORT", "6379")}"

  config :push_server, :kafka_hosts,
    [{String.to_atom(System.get_env("KAFKA_HOST", "redpanda")),
      String.to_integer(System.get_env("KAFKA_PORT", "9092"))}]
end
```

### Broadway handle_message/3 with JSON + PubSub Broadcast
```elixir
# Source: https://hexdocs.pm/broadway/Broadway.html
def handle_message(_processor, message, _context) do
  with {:ok, payload}   <- Jason.decode(message.data),
       {:ok, driver_id} <- Map.fetch(payload, "driver_id") do
    broadcast_payload = Map.put(payload, "driver_id", driver_id)
    Phoenix.PubSub.broadcast!(
      PushServer.PubSub,
      "driver:#{driver_id}",
      %Phoenix.Socket.Broadcast{
        topic: "driver:#{driver_id}",
        event: "location_update",
        payload: broadcast_payload
      }
    )
  else
    _ ->
      Broadway.Message.failed(message, "decode_error")
  end
  message
end
```

### Channel handle_info receiving PubSub Broadcast
```elixir
# Source: https://hexdocs.pm/phoenix/Phoenix.Channel.html
alias Phoenix.Socket.Broadcast

def handle_info(%Broadcast{event: event, payload: payload}, socket) do
  push(socket, event, payload)
  {:noreply, socket}
end
```

### Multi-Stage Dockerfile for Elixir/Alpine OTP Release
```dockerfile
# Source: https://hexdocs.pm/phoenix/releases.html (adapted for alpine:3.21)

# --- Build stage ---
FROM elixir:1.18-alpine AS builder
RUN apk add --no-cache build-base git
WORKDIR /app
ENV MIX_ENV=prod
RUN mix local.hex --force && mix local.rebar --force
COPY mix.exs mix.lock ./
RUN mix deps.get --only prod
RUN mix deps.compile
COPY config config
COPY lib lib
RUN mix release

# --- Runtime stage ---
FROM alpine:3.21 AS app
RUN apk add --no-cache libstdc++ openssl ncurses-libs
WORKDIR /app
COPY --from=builder /app/_build/prod/rel/push_server ./
EXPOSE 4000
CMD ["bin/push_server", "start"]
```

### PromEx Plug Wiring in Endpoint
```elixir
# Source: https://hexdocs.pm/prom_ex/readme.html
defmodule PushServerWeb.Endpoint do
  use Phoenix.Endpoint, otp_app: :push_server

  socket "/socket", PushServerWeb.UserSocket,
    websocket: [timeout: 45_000],
    longpoll: false

  # Must come before other plugs; serves GET /metrics
  plug PromEx.Plug, prom_ex_module: PushServer.PromEx

  plug PushServerWeb.Router
end
```

### Latency Computation from emitted_at
```elixir
defp compute_latency_ms(emitted_at_str) do
  with {:ok, emitted_at, _} <- DateTime.from_iso8601(emitted_at_str) do
    DateTime.diff(DateTime.utc_now(), emitted_at, :millisecond)
  else
    _ -> 0
  end
end
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| KafkaEx | broadway_kafka (brod) | 2020+ | BroadwayKafka has better backpressure integration and is maintained by dashbit |
| Phoenix.PubSub.PG2 | Phoenix.PubSub (default) | Phoenix 1.5 | PG2 is now the default; Redis adapter still required for cross-host (non-cluster) multi-replica |
| prom_ex 1.9.x | prom_ex 1.11.0 | Nov 2025 | Current stable; API stable since 1.9 |
| Erlang cluster (libcluster) | Redis PubSub adapter | Project choice | Redis adapter avoids Erlang cookie/network config for Docker Compose deployments |

**Deprecated/outdated:**
- `Phoenix.PubSub.Redis` as a standalone package pre-3.0: earlier versions (2.x) had different node_name handling. Use 3.0.x.
- `:phoenix_pubsub_redis, "~> 2.1"`: the hexdocs for 2.1 show different configuration; use `~> 3.0` for current Phoenix 1.7 compatibility.

---

## Open Questions

1. **Connection count gauge tracking via PromEx polling vs event**
   - What we know: PromEx supports both event-driven and polling metrics. Connection count changes on join/terminate.
   - What's unclear: Whether a `last_value` (gauge) updated via `:telemetry.execute` on join/terminate is reliable under high churn, vs. a polling metric reading `Phoenix.Channel` process count.
   - Recommendation: Use event-driven with an `Agent` holding an atomic counter. On join: increment; on `terminate/2`: decrement. Emit via `:telemetry.execute` on both events.

2. **BroadwayKafka group_id per replica vs shared**
   - What we know: All replicas must share the same `group_id` to partition Kafka load between them (each partition consumed by one replica at a time).
   - What's unclear: Whether the group_id should encode the application name or be dynamically configured.
   - Recommendation: Use a fixed string `"push_server"` as `group_id`. All replicas share this group and Kafka handles partition assignment automatically.

3. **NODE_NAME uniqueness strategy in docker-compose**
   - What we know: `phoenix_pubsub_redis` requires unique `node_name` per process.
   - What's unclear: Docker Compose replica numbering vs. hostname injection.
   - Recommendation: Use `node_name: System.get_env("HOSTNAME", "push_server_default")` in `config/runtime.exs`. Docker sets `$HOSTNAME` to the container ID automatically, which is unique per container.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | ExUnit (built into Elixir/Mix) |
| Config file | `push-server/test/test_helper.exs` — Wave 0 gap |
| Quick run command | `cd push-server && mix test --no-start` |
| Full suite command | `cd push-server && mix test` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PUSH-01 | UserSocket accepts connections, channel route matches `customer:*` | unit | `mix test test/push_server_web/user_socket_test.exs -x` | Wave 0 |
| PUSH-02 | join/3 resolves driver from Redis mock, pushes initial position | unit | `mix test test/push_server_web/customer_channel_test.exs -x` | Wave 0 |
| PUSH-02 | join/3 returns `{:error, "unknown_customer"}` when Redis key absent | unit | `mix test test/push_server_web/customer_channel_test.exs -x` | Wave 0 |
| PUSH-02 | join/3 returns `{:error, "service_unavailable"}` when Redis error | unit | `mix test test/push_server_web/customer_channel_test.exs -x` | Wave 0 |
| PUSH-02 | join succeeds and skips initial push when `driver:*:latest` is nil | unit | `mix test test/push_server_web/customer_channel_test.exs -x` | Wave 0 |
| PUSH-03 | `handle_message/3` decodes JSON, broadcasts correct payload shape | unit | `mix test test/push_server/pipeline_test.exs -x` | Wave 0 |
| PUSH-03 | `handle_failed/2` logs warning and returns messages (no crash) | unit | `mix test test/push_server/pipeline_test.exs -x` | Wave 0 |
| PUSH-04 | PubSub broadcast reaches channel process subscribed to `driver:*` topic | integration | `mix test test/push_server/pubsub_test.exs -x` | Wave 0 |
| PUSH-05 | PromEx module compiles with Beam + Phoenix + custom plugins | smoke | `mix test test/push_server/prom_ex_test.exs -x` | Wave 0 |
| PUSH-05 | GET /metrics returns 200 with `push_server_connections_active` in body | integration | `mix test test/push_server_web/metrics_test.exs -x` | Wave 0 |

### Sampling Rate
- **Per task commit:** `cd push-server && mix test --no-start 2>&1 | tail -5`
- **Per wave merge:** `cd push-server && mix test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `push-server/mix.exs` — project file with all deps
- [ ] `push-server/config/config.exs` — compile-time defaults
- [ ] `push-server/config/runtime.exs` — env-var driven runtime config
- [ ] `push-server/test/test_helper.exs` — ExUnit start
- [ ] `push-server/test/push_server_web/user_socket_test.exs` — PUSH-01
- [ ] `push-server/test/push_server_web/customer_channel_test.exs` — PUSH-01, PUSH-02
- [ ] `push-server/test/push_server/pipeline_test.exs` — PUSH-03
- [ ] `push-server/test/push_server/pubsub_test.exs` — PUSH-04
- [ ] `push-server/test/push_server/prom_ex_test.exs` — PUSH-05
- [ ] `push-server/test/push_server_web/metrics_test.exs` — PUSH-05

**Channel testing note:** Phoenix provides `Phoenix.ChannelTest` with `connect/3`, `subscribe_and_join/3`, `push/3`, `assert_push/3` helpers. No real Redis or Kafka needed for unit tests — mock Redix via `Mox` or a test double module.

---

## Sources

### Primary (HIGH confidence)
- [hexdocs.pm/phoenix/channels.html](https://hexdocs.pm/phoenix/channels.html) — Channel join/3, handle_info/2, push/3, broadcast/3, PubSub subscription pattern
- [hexdocs.pm/phoenix/Phoenix.Channel.html](https://hexdocs.pm/phoenix/Phoenix.Channel.html) — Full Channel callback API
- [hexdocs.pm/broadway/Broadway.html](https://hexdocs.pm/broadway/Broadway.html) — Broadway v1.2.1 start_link options, handle_message/3, handle_failed/2 signatures
- [hexdocs.pm/broadway/apache-kafka.html](https://hexdocs.pm/broadway/apache-kafka.html) — BroadwayKafka integration guide
- [hexdocs.pm/phoenix_pubsub_redis/Phoenix.PubSub.Redis.html](https://hexdocs.pm/phoenix_pubsub_redis/Phoenix.PubSub.Redis.html) — Redis adapter v3.0.1 config options
- [hexdocs.pm/prom_ex/readme.html](https://hexdocs.pm/prom_ex/) — PromEx v1.11.0 installation, plugins, supervision
- [hexdocs.pm/phoenix/releases.html](https://hexdocs.pm/phoenix/releases.html) — Multi-stage Dockerfile, runtime.exs, SECRET_KEY_BASE

### Secondary (MEDIUM confidence)
- [github.com/dashbitco/broadway_kafka README](https://github.com/dashbitco/broadway_kafka/blob/main/README.md) — BroadwayKafka hosts format, mix.exs dep `~> 0.4.1`
- [dockyard.com — Building Your Own Prometheus Metrics with PromEx Part 2](https://dockyard.com/blog/2023/10/03/building-your-own-prometheus-metrics-with-promex-part-2) — Custom plugin patterns (counter/sum, last_value, distribution)
- [hex.pm/packages/broadway_kafka](https://hex.pm/packages/broadway_kafka) — Current version 0.4.4

### Tertiary (LOW confidence)
- WebSearch: Alpine 3.18+ DNS resolution fix for BEAM — flagged for validation; mitigated by locking alpine:3.21 as per CONTEXT.md

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries verified on hexdocs.pm with current versions
- Architecture: HIGH — Phoenix Channel + Broadway + PubSub is well-documented official pattern
- Pitfalls: MEDIUM — node_name pitfall and Alpine DNS verified; handle_failed/2 semantics from Broadway docs
- Validation architecture: HIGH — ExUnit is built-in; Phoenix.ChannelTest is official test helper

**Research date:** 2026-03-06
**Valid until:** 2026-06-06 (stable libraries; phoenix_pubsub_redis 3.x, broadway_kafka 0.4.x, prom_ex 1.11.x are stable)

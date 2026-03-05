# Phase 1: Infrastructure - Research

**Researched:** 2026-03-05
**Domain:** Docker Compose orchestration — Redpanda (Kafka-compatible), Redis, k6, Grafana/Prometheus, Nginx scaffolding
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Directory layout:**
- Service directories at root: `location-service/`, `push-server/`, `simulator/`, `nginx/`, `stress/` alongside `docker-compose.yml`
- Observability config grouped under `observability/`: `observability/prometheus.yml`, `observability/grafana/provisioning/`, `observability/grafana/dashboards/`
- Init scripts under `infra/`: `infra/redpanda-init.sh`
- Each Go service has its own `go.mod` / `go.sum` (separate modules — independent builds, no dependency bleed)

**Default .env values:**
- `DRIVER_COUNT=10`
- `EMIT_INTERVAL_MS=1000`
- `LOCATION_SERVICE_REPLICAS=2`
- `PUSH_SERVER_REPLICAS=2`
- `PARTITION_MULTIPLIER=2`
- `SECRET_KEY_BASE` — hardcoded dev-safe random string in `.env.example` with comment

**Port exposure:**
| Host port | Service |
|-----------|---------|
| :80 | Nginx |
| :3000 | Grafana |
| :8080 | Redpanda Console |
| :9090 | Prometheus |
Internal only (no host mapping): `location-service:8080`, `push-server:4000`, `redis:6379`, `redpanda:9092`

**Readiness / health checks:**
- Redpanda: Init container using `rpk cluster health` in a loop, then `rpk topic create driver.location`. Exits with code 0. Downstream services use `depends_on: condition: service_completed_successfully`.
- Redpanda init container reuses `redpandadata/redpanda` image (rpk bundled).
- Redis: `redis-cli ping` healthcheck (`interval: 5s`, `timeout: 3s`, `retries: 10`). Services use `depends_on: condition: service_healthy`.

**Specifics:**
- Partition count formula: `PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER` — calculated by init container from env vars at startup, NOT hardcoded.
- Stress overlay `docker-compose.stress.yml` adds only the k6 service.
- macOS M4 Pro: `kafka-exporter` and `cAdvisor` need `platform: linux/amd64` in their compose definitions (Rosetta 2 handles emulation).

### Claude's Discretion

- Exact health check intervals/retries for Redpanda itself (beyond the init container pattern)
- Whether to include a `restart: unless-stopped` policy on infrastructure services
- Exact Grafana/Prometheus image versions (use latest stable)

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | Full stack starts with a single `docker compose up --build` on any machine with Docker installed | Docker Compose `depends_on` ordering, health checks, init container pattern documented |
| INFRA-02 | `.env.example` documents all tunable parameters (DRIVER_COUNT, EMIT_INTERVAL_MS, LOCATION_SERVICE_REPLICAS, PUSH_SERVER_REPLICAS, PARTITION_MULTIPLIER, SECRET_KEY_BASE) | Standard `.env` + `.env.example` Docker Compose pattern documented |
| INFRA-03 | Redpanda init container creates `driver.location` topic with `PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER` partitions before any dependent service starts | `rpk topic create -p <N> -r 1 driver.location`; arithmetic must be in shell script, not compose interpolation |
| INFRA-04 | `docker-compose.stress.yml` overlay adds k6 service; launches with `-f docker-compose.yml -f docker-compose.stress.yml up k6` | Docker Compose multi-file merge confirmed; k6 `grafana/k6` image with `experimental-prometheus-rw` output confirmed |
| INFRA-05 | Services declare correct `depends_on` ordering so stack starts cleanly | `service_completed_successfully` and `service_healthy` conditions confirmed in Docker Compose |
</phase_requirements>

---

## Summary

This phase delivers the complete Docker Compose scaffolding for all subsequent service phases. The primary technical challenge is reliable startup ordering: Redpanda must be healthy before the init container runs, the init container must create the topic before application services start, and Redis must pass its health check before services that depend on it launch.

The critical discovery is that Docker Compose variable interpolation does NOT support arithmetic. The partition count formula (`PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER`) cannot be evaluated in the compose file — it must be computed in `infra/redpanda-init.sh` using shell arithmetic (`$((PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER))`). Both environment variables are passed to the init container via the `environment:` block.

The k6 Prometheus remote write integration uses the built-in `experimental-prometheus-rw` output (available since k6 v0.42.0, no custom xk6 build required). Prometheus must be started with `--web.enable-remote-write-receiver` flag. For the stress overlay, a separate `docker-compose.stress.yml` that adds only the k6 service is the correct pattern — the `-f` flag merge approach is confirmed.

**Primary recommendation:** Build the init container pattern around `infra/redpanda-init.sh` invoked as the command of a short-lived container using the same `redpandadata/redpanda` image; compute partition count in shell arithmetic; use `service_completed_successfully` for services that depend on it.

---

## Standard Stack

### Core
| Image | Version | Purpose | Why Standard |
|-------|---------|---------|--------------|
| `redpandadata/redpanda` | `v25.2.x` (pin latest stable) | Kafka-compatible broker, includes rpk CLI | Single binary, no ZooKeeper, Kafka-API compatible, rpk bundled |
| `redpandadata/console` | `v3.x` (pin latest stable) | Topic browser and consumer lag UI at :8080 | Official Redpanda UI, Kafka-API compatible |
| `redis` | `7-alpine` | In-memory store for driver positions and pub/sub adapter | Smallest tested image, Alpine minimal |
| `prom/prometheus` | `v2.x` (latest stable) | Metrics scraping and remote write receiver | Standard Prometheus; must run with `--web.enable-remote-write-receiver` |
| `grafana/grafana` | `11.x` (latest stable) | Dashboard visualization | Official image; provisioning via mounted volumes |
| `grafana/k6` | `latest` | Load testing; stress overlay only | Built-in `experimental-prometheus-rw` output; no xk6 build needed |

### Supporting
| Image | Version | Purpose | When to Use |
|-------|---------|---------|-------------|
| `danielqsj/kafka-exporter` | `latest` | Exports Redpanda broker metrics to Prometheus | Required for Kafka consumer lag in Grafana; supports arm64 natively |
| `gcr.io/cadvisor/cadvisor` | `latest` | Container CPU/memory metrics for Prometheus | Requires `platform: linux/amd64` on macOS M-series (runs under Rosetta 2) |
| `nginx` | `alpine` | Reverse proxy and load balancer scaffolding | Lightweight; config populated in later phase |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `redis:7-alpine` | `redis:7` | Alpine is smaller (~30MB vs ~100MB); adequate for dev |
| `danielqsj/kafka-exporter` | Redpanda's built-in metrics | kafka-exporter exposes standard Kafka metrics Prometheus already knows; Redpanda's native metrics require different scrape config |

**Installation (Docker images pulled on `docker compose up --build`):**
```bash
# No local install needed — all images pulled by Docker Compose
# Verify Docker and Compose plugin:
docker --version        # 24.x+
docker compose version  # 2.x plugin (not legacy docker-compose)
```

---

## Architecture Patterns

### Recommended Project Structure
```
mototaxi/
├── docker-compose.yml          # base stack
├── docker-compose.stress.yml   # k6 overlay (adds k6 service only)
├── .env                        # gitignored, local values
├── .env.example                # committed, documents all tunables
├── location-service/           # Go, own go.mod (Phase 2)
├── push-server/                # Elixir/Phoenix (Phase 4)
├── simulator/                  # Go, own go.mod (Phase 3)
├── nginx/
│   └── nginx.conf              # populated in Phase 5
├── stress/
│   ├── drivers.js
│   ├── customers.js
│   └── latency.js
├── infra/
│   └── redpanda-init.sh        # topic creation init script
└── observability/
    ├── prometheus.yml
    └── grafana/
        ├── provisioning/
        │   ├── datasources/
        │   │   └── prometheus.yaml
        │   └── dashboards/
        │       └── dashboards.yaml
        └── dashboards/
            └── *.json
```

### Pattern 1: Init Container for Topic Creation

**What:** A short-lived container using the broker image that runs `rpk cluster health` until the broker is ready, then creates the topic with the computed partition count, then exits 0.

**When to use:** Any time a Kafka/Redpanda topic must exist before application services start.

**The key constraint:** Docker Compose variable interpolation does not support arithmetic expressions. Partition count must be computed inside the shell script.

**Example `infra/redpanda-init.sh`:**
```bash
#!/bin/bash
set -e

# Wait for broker to be healthy
until rpk cluster health --brokers redpanda:9092 2>&1 | grep -q "Healthy"; do
  echo "Waiting for Redpanda..."
  sleep 2
done

# Compute partition count from env vars (arithmetic must be in shell)
PARTITIONS=$((PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER))

echo "Creating driver.location with ${PARTITIONS} partitions..."
rpk topic create driver.location \
  --partitions "${PARTITIONS}" \
  --replicas 1 \
  --brokers redpanda:9092 \
  --if-not-exists

echo "Topic created. Init complete."
```

**Example compose service block:**
```yaml
# Source: pattern derived from Redpanda official docs and Docker Compose docs
redpanda-init:
  image: redpandadata/redpanda:v25.2.7
  depends_on:
    redpanda:
      condition: service_healthy
  environment:
    PUSH_SERVER_REPLICAS: ${PUSH_SERVER_REPLICAS:-2}
    PARTITION_MULTIPLIER: ${PARTITION_MULTIPLIER:-2}
  entrypoint: ["/bin/bash", "/infra/redpanda-init.sh"]
  volumes:
    - ./infra/redpanda-init.sh:/infra/redpanda-init.sh:ro
  networks:
    - mototaxi
```

**Downstream service depending on init:**
```yaml
location-service:
  depends_on:
    redpanda-init:
      condition: service_completed_successfully
    redis:
      condition: service_healthy
```

### Pattern 2: Redis Health Check

**What:** Redis exposes a `PING` command that returns `PONG` when ready. Use `redis-cli ping` as the test.

**Example:**
```yaml
# Source: Docker Compose official docs pattern
redis:
  image: redis:7-alpine
  healthcheck:
    test: ["CMD", "redis-cli", "ping"]
    interval: 5s
    timeout: 3s
    retries: 10
    start_period: 5s
  networks:
    - mototaxi
```

### Pattern 3: Redpanda Broker Health Check

**What:** Use `rpk cluster health` (bundled in the image) with the `--brokers` flag to verify the broker is ready before the init container runs.

**Note:** `rpk cluster health` reflects cluster-level health; for a single-node dev setup this is equivalent to "broker is ready." The Redpanda Helm charts team notes it should not be used as a Kubernetes readiness probe (because it checks cluster-wide, not individual pod), but for Docker Compose single-node dev it is the correct pattern.

**Example:**
```yaml
redpanda:
  image: redpandadata/redpanda:v25.2.7
  command:
    - redpanda start
    - --smp 1
    - --memory 1G
    - --overprovisioned
    - --node-id 0
    - --kafka-addr PLAINTEXT://0.0.0.0:9092,PLAINTEXT_HOST://0.0.0.0:19092
    - --advertise-kafka-addr PLAINTEXT://redpanda:9092,PLAINTEXT_HOST://localhost:19092
  healthcheck:
    test: ["CMD-SHELL", "rpk cluster health --brokers localhost:9092 | grep -q 'Healthy: true' || exit 1"]
    interval: 10s
    timeout: 5s
    retries: 10
    start_period: 15s
  networks:
    - mototaxi
```

### Pattern 4: Docker Compose Multi-File Overlay (Stress)

**What:** A separate `docker-compose.stress.yml` that adds only the k6 service. The base stack runs as normal; stress tests are opt-in via `-f` flag.

**Example `docker-compose.stress.yml`:**
```yaml
# Source: Docker Compose multi-file merge docs
services:
  k6:
    image: grafana/k6:latest
    environment:
      K6_PROMETHEUS_RW_SERVER_URL: http://prometheus:9090/api/v1/write
      K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM: "true"
    volumes:
      - ./stress:/scripts
    networks:
      - mototaxi
    profiles: []
```

**Launch command:**
```bash
docker compose -f docker-compose.yml -f docker-compose.stress.yml up k6
```

**Run a specific script:**
```bash
docker compose -f docker-compose.yml -f docker-compose.stress.yml \
  run k6 run /scripts/drivers.js -o experimental-prometheus-rw
```

### Pattern 5: .env / .env.example Convention

**What:** `.env` holds local values (gitignored). `.env.example` is committed and documents every parameter.

**Example `.env.example`:**
```bash
# === Driver Simulator ===
DRIVER_COUNT=10               # Number of concurrent simulated drivers
EMIT_INTERVAL_MS=1000         # How often each driver emits a location update (ms)

# === Scaling ===
LOCATION_SERVICE_REPLICAS=2   # How many location-service replicas to run
PUSH_SERVER_REPLICAS=2        # How many push-server replicas to run
PARTITION_MULTIPLIER=2        # Partition count = PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER

# === Security ===
# For local dev only — replace in production
SECRET_KEY_BASE=dev_only_random_string_replace_in_production_abc123xyz789
```

### Anti-Patterns to Avoid

- **Computing partition count in the compose file:** Docker Compose interpolation does not support `$(( ))` arithmetic. Always delegate to the shell script inside the init container.
- **Using `depends_on` without conditions:** Plain `depends_on:` only waits for the container to start, not for it to be healthy or complete. Always pair with `condition: service_healthy` or `condition: service_completed_successfully`.
- **Hardcoding partition count:** The formula `PUSH_SERVER_REPLICAS × PARTITION_MULTIPLIER` must be read from env vars at startup so scaling experiments work without touching the compose file.
- **Exposing internal service ports to host:** Only expose Nginx (:80), Grafana (:3000), Redpanda Console (:8080), and Prometheus (:9090). Internal ports stay Docker-network-only.
- **Using `docker-compose` (v1 CLI):** All commands assume `docker compose` (v2 plugin). v1 is end-of-life.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Topic existence check | Custom wait loop with Kafka client | `rpk topic create --if-not-exists` flag | Idempotent; handles race conditions |
| Partition count from env | Complex compose override or hacky env pre-processing | Shell arithmetic in `infra/redpanda-init.sh` | Standard, readable, debuggable |
| k6 → Prometheus metrics pipeline | Custom metrics forwarder | k6 built-in `experimental-prometheus-rw` output + `--web.enable-remote-write-receiver` in Prometheus | Maintained by Grafana, stable since k6 v0.42.0 |
| Service readiness polling | Custom TCP polling script | Docker Compose `healthcheck` + `depends_on` conditions | Native compose feature; no extra tooling |
| Redis connectivity wait | `sleep 10` in entrypoint scripts | `redis-cli ping` healthcheck + `service_healthy` condition | Deterministic; doesn't over-wait |

**Key insight:** The `depends_on` + `healthcheck` combination in Docker Compose v2 solves the entire startup ordering problem. Anything hand-rolled on top of that introduces timing fragility.

---

## Common Pitfalls

### Pitfall 1: Arithmetic in Docker Compose Interpolation

**What goes wrong:** Writing `${PUSH_SERVER_REPLICAS} * ${PARTITION_MULTIPLIER}` or similar in `docker-compose.yml` produces a literal string, not a number.
**Why it happens:** Docker Compose interpolation is basic string substitution — it does not evaluate expressions.
**How to avoid:** All arithmetic must happen inside the init container shell script: `PARTITIONS=$((PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER))`.
**Warning signs:** Topic is created with "1" partition, or `rpk topic create` fails with invalid argument.

### Pitfall 2: Redpanda Init Container Starts Before Broker Is Ready

**What goes wrong:** Init container runs `rpk topic create` immediately but the broker hasn't finished bootstrapping — the command fails and the container exits non-zero, blocking all downstream services.
**Why it happens:** `depends_on: service_started` (the default) only waits for the container process to launch, not for Redpanda to finish its init sequence (several seconds on first boot).
**How to avoid:** Configure a healthcheck on the Redpanda service and use `depends_on: condition: service_healthy` on the init container. Use a retry loop in `redpanda-init.sh` as a belt-and-suspenders guard.
**Warning signs:** Init container exits with non-zero code on first `docker compose up --build`; subsequent `up` (warm restart) works fine.

### Pitfall 3: service_completed_successfully Not Recognized

**What goes wrong:** Older Docker Desktop versions don't support `condition: service_completed_successfully` and silently fall back or error.
**Why it happens:** This condition was added in Docker Compose v2.3+ / Docker Desktop 4.x. Some CI or older dev machines may have Docker Compose v1.
**How to avoid:** Document minimum requirement: Docker Desktop 4.x+ (macOS), or Docker Engine with Compose plugin v2.3+. Check with `docker compose version`.
**Warning signs:** Compose parses but downstream services start before init container finishes.

### Pitfall 4: cAdvisor Failing to Start on macOS M-Series

**What goes wrong:** `cAdvisor` crashes or fails to start on Apple Silicon without `platform: linux/amd64`.
**Why it happens:** The official `gcr.io/cadvisor/cadvisor` image does not publish a native arm64 variant; it requires Rosetta 2 emulation.
**How to avoid:** Add `platform: linux/amd64` to the cAdvisor (and kafka-exporter) service definition. Rosetta 2 handles the translation automatically on M1/M2/M3/M4 Macs.
**Warning signs:** `cAdvisor` exits immediately with `exec format error` or similar.

### Pitfall 5: k6 Can't Reach Prometheus for Remote Write

**What goes wrong:** k6 sends metrics to `http://prometheus:9090/api/v1/write` but Prometheus returns 404 or connection refused.
**Why it happens:** Prometheus does not enable the remote write receiver endpoint by default; it requires the `--web.enable-remote-write-receiver` flag on startup.
**How to avoid:** Add `--web.enable-remote-write-receiver` to Prometheus's command in the compose file. Also ensure k6 and Prometheus are on the same Docker network.
**Warning signs:** k6 logs show `level=error msg="Failed to push to the output" ... 404` or similar.

### Pitfall 6: Redpanda `--advertise-kafka-addr` Missing

**What goes wrong:** Services running inside Docker can connect to Redpanda, but connections fail intermittently because the broker advertises an address that isn't reachable from within the Docker network.
**Why it happens:** Redpanda must advertise the correct address for the Docker-internal listener (`redpanda:9092`) separately from the host-accessible listener. If only the host-accessible address is advertised, intra-container clients get an unreachable address.
**How to avoid:** Configure two listeners: `PLAINTEXT://0.0.0.0:9092` (internal) with `--advertise-kafka-addr PLAINTEXT://redpanda:9092`, and optionally `PLAINTEXT_HOST://0.0.0.0:19092` (external) with `--advertise-kafka-addr PLAINTEXT_HOST://localhost:19092`.
**Warning signs:** Location service or push-server logs show Kafka connection timeouts after initial connect.

---

## Code Examples

### Full Redpanda Service with Health Check
```yaml
# Source: Redpanda official Docker Compose Labs + rpk cluster health docs
redpanda:
  image: redpandadata/redpanda:v25.2.7
  container_name: redpanda
  command:
    - redpanda start
    - --smp 1
    - --memory 1G
    - --overprovisioned
    - --node-id 0
    - --kafka-addr PLAINTEXT://0.0.0.0:9092
    - --advertise-kafka-addr PLAINTEXT://redpanda:9092
    - --pandaproxy-addr 0.0.0.0:8082
    - --advertise-pandaproxy-addr localhost:8082
    - --schema-registry-addr 0.0.0.0:8081
    - --advertise-schema-registry-addr http://localhost:8081
    - --rpc-addr redpanda:33145
    - --advertise-rpc-addr redpanda:33145
    - --mode dev-container
  healthcheck:
    test: ["CMD-SHELL", "rpk cluster health --brokers localhost:9092 | grep -q 'Healthy: true' || exit 1"]
    interval: 10s
    timeout: 5s
    retries: 10
    start_period: 15s
  networks:
    - mototaxi
```

### Redis Service
```yaml
# Source: Docker Compose healthcheck docs pattern
redis:
  image: redis:7-alpine
  container_name: redis
  healthcheck:
    test: ["CMD", "redis-cli", "ping"]
    interval: 5s
    timeout: 3s
    retries: 10
    start_period: 5s
  networks:
    - mototaxi
```

### Redpanda Init Container Service
```yaml
# Source: Docker Compose depends_on docs + Redpanda rpk topic create docs
redpanda-init:
  image: redpandadata/redpanda:v25.2.7
  container_name: redpanda-init
  depends_on:
    redpanda:
      condition: service_healthy
  environment:
    PUSH_SERVER_REPLICAS: ${PUSH_SERVER_REPLICAS:-2}
    PARTITION_MULTIPLIER: ${PARTITION_MULTIPLIER:-2}
  entrypoint: ["/bin/bash", "/infra/redpanda-init.sh"]
  volumes:
    - ./infra/redpanda-init.sh:/infra/redpanda-init.sh:ro
  networks:
    - mototaxi
```

### Redpanda Console Service
```yaml
# Source: Redpanda Labs docker-compose single-broker
redpanda-console:
  image: redpandadata/console:v3.3.2
  container_name: redpanda-console
  depends_on:
    redpanda-init:
      condition: service_completed_successfully
  environment:
    KAFKA_BROKERS: redpanda:9092
  ports:
    - "8080:8080"
  networks:
    - mototaxi
```

### Prometheus with Remote Write Receiver
```yaml
# Source: Grafana k6 Prometheus remote write docs
prometheus:
  image: prom/prometheus:v2.53.0
  container_name: prometheus
  command:
    - --config.file=/etc/prometheus/prometheus.yml
    - --web.enable-remote-write-receiver
    - --enable-feature=native-histograms
  volumes:
    - ./observability/prometheus.yml:/etc/prometheus/prometheus.yml:ro
  ports:
    - "9090:9090"
  networks:
    - mototaxi
```

### k6 Service in Stress Overlay
```yaml
# Source: Grafana k6 docs + Docker Compose multi-file merge docs
# File: docker-compose.stress.yml
services:
  k6:
    image: grafana/k6:latest
    container_name: k6
    environment:
      K6_PROMETHEUS_RW_SERVER_URL: http://prometheus:9090/api/v1/write
      K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM: "true"
    volumes:
      - ./stress:/scripts
    networks:
      - mototaxi

networks:
  mototaxi:
    external: true
```

### cAdvisor on macOS M-Series
```yaml
# Source: cAdvisor GitHub issues #2763, #2838
cadvisor:
  image: gcr.io/cadvisor/cadvisor:latest
  platform: linux/amd64      # Required on Apple Silicon — Rosetta 2 handles emulation
  container_name: cadvisor
  volumes:
    - /:/rootfs:ro
    - /var/run:/var/run:rw
    - /sys:/sys:ro
    - /var/lib/docker/:/var/lib/docker:ro
  networks:
    - mototaxi
```

### kafka-exporter Targeting Redpanda
```yaml
# Source: danielqsj/kafka-exporter docs — supports linux/arm64 natively
kafka-exporter:
  image: danielqsj/kafka-exporter:latest
  platform: linux/amd64      # Keep for consistency per project decision
  container_name: kafka-exporter
  command:
    - --kafka.server=redpanda:9092
  networks:
    - mototaxi
  depends_on:
    redpanda-init:
      condition: service_completed_successfully
```

### infra/redpanda-init.sh
```bash
#!/bin/bash
set -e

echo "Waiting for Redpanda broker..."
until rpk cluster health --brokers redpanda:9092 2>&1 | grep -q "Healthy: true"; do
  echo "  Broker not ready — retrying in 2s..."
  sleep 2
done
echo "Broker healthy."

# Arithmetic must be computed here — Docker Compose interpolation does NOT support math
PARTITIONS=$(( PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER ))
echo "Creating topic driver.location with ${PARTITIONS} partitions..."

rpk topic create driver.location \
  --partitions "${PARTITIONS}" \
  --replicas 1 \
  --brokers redpanda:9092 \
  --if-not-exists

echo "Init complete."
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `docker-compose` (v1, Python) | `docker compose` (v2, Go plugin) | Docker Desktop 4.x (2022) | v1 is EOL; v2 is default on all current installs |
| InfluxDB for k6 metrics | Prometheus remote write (`experimental-prometheus-rw`) | k6 v0.42.0 (2023) | No custom xk6 build; `grafana/k6` image works directly |
| xk6-output-prometheus-remote extension | Built-in `experimental-prometheus-rw` | k6 v0.42.0; xk6 repo archived March 2025 | Extension repo archived; built-in is the path |
| Plain `depends_on:` (start only) | `depends_on: condition: service_healthy / service_completed_successfully` | Docker Compose 2.3+ | Deterministic startup; eliminates sleep-based hacks |
| Confluent's `cp-kafka` for local dev | Redpanda (`redpandadata/redpanda`) | ~2022 | Single binary, no ZooKeeper, faster cold start, rpk bundled |

**Deprecated/outdated:**
- `docker-compose` v1 CLI: End-of-life, do not use. All commands use `docker compose` (space, not hyphen).
- `xk6-output-prometheus-remote` extension: Archived March 2025. Use built-in `experimental-prometheus-rw` output.
- `sleep N` in entrypoints: Replaced by Docker Compose health check conditions.

---

## Open Questions

1. **Exact health check for Redpanda broker (Healthy vs healthy string)**
   - What we know: `rpk cluster health` outputs a status line; the grep pattern `'Healthy: true'` was found in community examples.
   - What's unclear: The exact output format may vary across Redpanda versions; `rpk cluster info` is an alternative that may be more stable.
   - Recommendation: Use `rpk cluster info --brokers localhost:9092` as the health check command (it returns non-zero on failure), OR test both against the pinned version at implementation time.

2. **kafka-exporter arm64 support**
   - What we know: The `danielqsj/kafka-exporter` project documentation mentions arm64 support; the project decision specifies `platform: linux/amd64` anyway.
   - What's unclear: Whether the current published Docker image actually includes an arm64 manifest or only amd64.
   - Recommendation: Honor the project decision and use `platform: linux/amd64` for both kafka-exporter and cAdvisor for consistency and to avoid any arm64 manifest gaps.

3. **`restart: unless-stopped` policy**
   - What we know: Left to Claude's discretion per CONTEXT.md.
   - Recommendation: Apply `restart: unless-stopped` to long-running infrastructure services (redpanda, redis, prometheus, grafana, redpanda-console, nginx) but NOT to the init container (`redpanda-init`) which must exit cleanly.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Shell-based smoke tests (no dedicated test framework for pure infra phase) |
| Config file | None — tests are ad-hoc `docker compose` invocations |
| Quick run command | `docker compose up --build -d && docker compose ps` |
| Full suite command | See Phase gate below |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INFRA-01 | `docker compose up --build` completes without error | smoke | `docker compose up --build -d; docker compose ps --format json \| jq '[.[] \| select(.State != "running" and .State != "exited")] \| length == 0'` | ❌ Wave 0 |
| INFRA-02 | `.env.example` contains all required keys | unit | `grep -E "DRIVER_COUNT|EMIT_INTERVAL_MS|LOCATION_SERVICE_REPLICAS|PUSH_SERVER_REPLICAS|PARTITION_MULTIPLIER|SECRET_KEY_BASE" .env.example \| wc -l` (expect 6) | ❌ Wave 0 |
| INFRA-03 | `driver.location` topic exists with correct partition count | smoke | `docker compose exec redpanda rpk topic describe driver.location --brokers localhost:9092` | ❌ Wave 0 |
| INFRA-04 | Stress overlay launches k6 without error | smoke | `docker compose -f docker-compose.yml -f docker-compose.stress.yml config` (validate) | ❌ Wave 0 |
| INFRA-05 | All services reach healthy/started state in correct order | smoke | `docker compose up --build -d && sleep 30 && docker compose ps` — no services in "restarting" | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `docker compose config --quiet` (validate compose YAML syntax)
- **Per wave merge:** `docker compose up --build -d && sleep 30 && docker compose ps`
- **Phase gate:** All services healthy, `driver.location` topic exists with correct partitions, before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `infra/redpanda-init.sh` — must be created before any smoke test runs
- [ ] `.env` copied from `.env.example` — required for `docker compose up` to work
- [ ] `observability/prometheus.yml` — must exist (even as stub) before Prometheus starts
- [ ] `observability/grafana/provisioning/datasources/prometheus.yaml` — required for auto-provisioning
- [ ] Docker Desktop 4.x+ or Docker Engine with Compose plugin v2.3+ — verify with `docker compose version`

---

## Sources

### Primary (HIGH confidence)
- [Redpanda official Docker Compose Labs](https://docs.redpanda.com/current/get-started/docker-compose-labs/) — single-broker and three-broker patterns
- [rpk topic create docs](https://docs.redpanda.com/current/reference/rpk/rpk-topic/rpk-topic-create/) — `-p` partitions, `-r` replicas, `--if-not-exists`, `--brokers` flags
- [rpk cluster health docs](https://docs.redpanda.com/current/reference/rpk/rpk-cluster/rpk-cluster-health/) — command syntax and flags
- [Docker Compose startup order docs](https://docs.docker.com/compose/how-tos/startup-order/) — `service_healthy`, `service_completed_successfully` patterns
- [Docker Compose variable interpolation docs](https://docs.docker.com/reference/compose-file/interpolation/) — confirmed no arithmetic support
- [Grafana k6 Prometheus remote write docs](https://grafana.com/docs/k6/latest/results-output/real-time/prometheus-remote-write/) — `experimental-prometheus-rw`, `K6_PROMETHEUS_RW_SERVER_URL`
- [Docker Compose multi-file merge docs](https://docs.docker.com/compose/how-tos/multiple-compose-files/merge/) — overlay file pattern

### Secondary (MEDIUM confidence)
- [oneuptime.com: How to Run Redpanda in Docker (2026-02-08)](https://oneuptime.com/blog/post/2026-02-08-how-to-run-redpanda-kafka-compatible-in-docker/view) — confirmed current patterns
- [cAdvisor GitHub issue #2763](https://github.com/google/cadvisor/issues/2763) — Apple Silicon `platform: linux/amd64` requirement
- [danielqsj/kafka-exporter GitHub](https://github.com/danielqsj/kafka_exporter) — arm64 support confirmed in source; project decision overrides to `linux/amd64`
- xk6-output-prometheus-remote archived March 2025 — built-in `experimental-prometheus-rw` confirmed as replacement

### Tertiary (LOW confidence)
- Community Docker Compose examples with `rpk cluster health` grep patterns — exact output string may vary by version; needs validation at implementation time

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — official docs and Docker Hub confirm all images; versions from releases page
- Architecture patterns: HIGH — all patterns derived from official Docker Compose and Redpanda documentation
- Pitfall: Arithmetic in compose interpolation: HIGH — explicitly stated in Docker Compose docs as unsupported
- Pitfall: cAdvisor on M-series: HIGH — confirmed in cAdvisor GitHub issues
- k6 Prometheus remote write: HIGH — xk6 repo archived March 2025; built-in confirmed since v0.42.0
- rpk health check grep string: LOW — community sourced, needs version-specific validation

**Research date:** 2026-03-05
**Valid until:** 2026-06-05 (stable infrastructure tooling — 90 days reasonable; Redpanda releases frequently so pin versions)

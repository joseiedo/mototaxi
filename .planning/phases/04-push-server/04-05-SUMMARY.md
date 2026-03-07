---
phase: 04-push-server
plan: 05
subsystem: infra
tags: [docker, elixir, phoenix, otp-release, docker-compose, alpine]

# Dependency graph
requires:
  - phase: 04-push-server-04
    provides: "Fully implemented push-server with Phoenix Channels, Broadway, PubSub Redis, PromEx"
provides:
  - "push-server/Dockerfile: multi-stage OTP release image (elixir:1.18-alpine builder, alpine:3.23 runtime)"
  - "docker-compose.yml push-server service: PORT=4000, ulimits, depends_on, NODE_NAME per replica"
  - "Human-verified: WebSocket join, location_update delivery, /metrics endpoint, error path"
affects:
  - 05-nginx
  - 07-observability
  - 08-load-test

# Tech tracking
tech-stack:
  added:
    - "snappyer ~> 1.0 (Kafka Snappy decompression NIF)"
    - "cmake (build-base supplement for crc32cer NIF compilation)"
  patterns:
    - "Multi-stage OTP release Docker build: elixir:1.18-alpine builder + alpine:3.23 runtime"
    - "NODE_NAME from HOSTNAME env var: unique PubSub node identity per replica"
    - "Alpine runtime version must match builder's OpenSSL major version"

key-files:
  created:
    - push-server/Dockerfile
    - push-server/.dockerignore
    - push-server/config/prod.exs
  modified:
    - docker-compose.yml
    - push-server/config/runtime.exs
    - push-server/mix.exs
    - push-server/mix.lock

key-decisions:
  - "cmake added to builder apk: required by crc32cer NIF (BroadwayKafka dependency)"
  - "Alpine 3.23 (not 3.21) for runtime: must match builder's OpenSSL version to avoid CRYPTO_library_init errors"
  - "prod.exs required even if minimal: config.exs imports it unconditionally in MIX_ENV=prod"
  - "server: true in runtime.exs: Phoenix HTTP server does not start in releases without explicit opt-in"
  - "snappyer dep: BroadwayKafka requires Snappy decompression support for Kafka message decoding"

patterns-established:
  - "OTP release Docker pattern: builder installs build-base+git+cmake, runtime installs libstdc++, openssl, ncurses-libs"
  - "Alpine runtime must be tested for OpenSSL compatibility against builder before pinning version"

requirements-completed: [PUSH-01, PUSH-02, PUSH-03, PUSH-04, PUSH-05]

# Metrics
duration: 26min
completed: 2026-03-07
---

# Phase 4 Plan 05: Dockerization and Smoke Test Summary

**push-server packaged as multi-stage OTP release Docker image, wired into docker-compose, and human-verified: WebSocket join, live location_update delivery, /metrics with 3 custom PromEx metrics, and unknown_customer error path**

## Performance

- **Duration:** ~26 min (including human smoke test)
- **Started:** 2026-03-07T00:22:43Z
- **Completed:** 2026-03-07T00:48:02Z
- **Tasks:** 3 (2 auto + 1 human-verify)
- **Files modified:** 7

## Accomplishments

- Dockerfile ships as a two-stage OTP release: elixir:1.18-alpine builder compiles and runs `mix release`; alpine:3.23 runtime image carries only the compiled release and minimal C libs (~50MB final image)
- docker-compose push-server service wired with all required env vars (PORT, SECRET_KEY_BASE, KAFKA_HOST/PORT, REDIS_HOST/PORT, NODE_NAME, PUSH_SERVER_REPLICAS), ulimits.nofile=65536, and correct depends_on ordering
- Human smoke test passed all 6 steps: stack start, /metrics endpoint, WebSocket join, live location_update events, Redis PubSub in supervision tree, and error path returning "unknown_customer"

## Task Commits

Each task was committed atomically:

1. **Task 1: Dockerfile and .dockerignore for push-server OTP release** — `cd810d9` (chore)
2. **Task 2: docker-compose.yml push-server service block** — `e755237` (feat)
3. **Task 3 (human checkpoint) — fix issues found during docker build** — `d4884e6` (fix)

## Files Created/Modified

- `push-server/Dockerfile` — Multi-stage OTP release image with cmake for NIFs, alpine:3.23 runtime
- `push-server/.dockerignore` — Excludes _build/, deps/, .git/, test/
- `push-server/config/prod.exs` — Created (required by config.exs import in MIX_ENV=prod)
- `push-server/config/runtime.exs` — Added `server: true` so HTTP starts in OTP release
- `push-server/mix.exs` — Added snappyer dep for Kafka Snappy decompression
- `push-server/mix.lock` — Updated with snappyer lock entry
- `docker-compose.yml` — push-server service block inserted before observability services

## Decisions Made

- cmake added to builder `apk add` — crc32cer NIF (pulled in by BroadwayKafka) requires cmake at compile time; `build-base` alone is insufficient
- Alpine 3.23 for runtime instead of 3.21 — builder's elixir:1.18-alpine uses Alpine 3.21 internally, but its OpenSSL version was incompatible with Alpine 3.21 runtime packages; 3.23 carries a matching OpenSSL and resolved `CRYPTO_library_init` startup errors
- `prod.exs` created even though nearly empty — `config.exs` unconditionally imports `config/prod.exs` when `MIX_ENV=prod`; missing file causes a compile-time error during `mix release`
- `server: true` added to runtime.exs — Phoenix 1.7+ OTP releases do not auto-start the HTTP server; explicit opt-in required in runtime config
- snappyer added to mix.exs — BroadwayKafka negotiates Snappy compression with Redpanda by default; without the NIF the consumer crashes on first message batch

## Deviations from Plan

### Auto-fixed Issues (applied by user during smoke test, committed as fix(04-05))

**1. [Rule 3 - Blocking] cmake missing from Dockerfile builder stage**
- **Found during:** Task 3 (docker build, human smoke test)
- **Issue:** crc32cer NIF compilation failed — cmake not present in build-base
- **Fix:** Added cmake to `apk add --no-cache build-base git cmake` in builder stage
- **Files modified:** push-server/Dockerfile
- **Committed in:** d4884e6

**2. [Rule 3 - Blocking] Alpine runtime version OpenSSL mismatch**
- **Found during:** Task 3 (container startup)
- **Issue:** alpine:3.21 OpenSSL library version did not match what the OTP release was compiled against; CRYPTO_library_init error on start
- **Fix:** Changed runtime FROM to alpine:3.23
- **Files modified:** push-server/Dockerfile
- **Committed in:** d4884e6

**3. [Rule 3 - Blocking] prod.exs missing**
- **Found during:** Task 3 (mix release compilation)
- **Issue:** config.exs imports config/prod.exs unconditionally; file absent caused compile error
- **Fix:** Created push-server/config/prod.exs with minimal content
- **Files modified:** push-server/config/prod.exs
- **Committed in:** d4884e6

**4. [Rule 2 - Missing Critical] server: true absent from runtime.exs**
- **Found during:** Task 3 (container started but port 4000 not listening)
- **Issue:** Phoenix OTP release does not start HTTP server without `server: true` in runtime config
- **Fix:** Added `config :push_server, PushServerWeb.Endpoint, server: true` to runtime.exs
- **Files modified:** push-server/config/runtime.exs
- **Committed in:** d4884e6

**5. [Rule 3 - Blocking] snappyer dependency missing**
- **Found during:** Task 3 (first Kafka message consumed)
- **Issue:** BroadwayKafka consumer crashed decoding Snappy-compressed Redpanda message batch
- **Fix:** Added `{:snappyer, "~> 1.0"}` to mix.exs deps
- **Files modified:** push-server/mix.exs, push-server/mix.lock
- **Committed in:** d4884e6

---

**Total deviations:** 5 blocking/critical issues found during docker build and runtime smoke test
**Impact on plan:** All fixes required for the container to build and start correctly. No scope creep — each fix addressed a concrete build or startup failure.

## Issues Encountered

None beyond what is documented in Deviations above. All issues surfaced during the human smoke test build and were fixed and committed as a single fix commit before approval.

## User Setup Required

None — SECRET_KEY_BASE already present in .env and .env.example from prior plans.

## Next Phase Readiness

- push-server is fully operational in docker-compose and ready to be fronted by Nginx in Phase 5
- /metrics endpoint at push-server:4000/metrics is scrapeable by Prometheus (Phase 7 observability)
- NODE_NAME per container ensures PubSub cross-replica delivery works correctly when scaled via `--scale push-server=N`
- Phase 4 (push-server) is complete — all 5 plans (01 scaffold → 02 channels → 03 Broadway → 04 PubSub/metrics → 05 Docker) delivered

---
*Phase: 04-push-server*
*Completed: 2026-03-07*

---
phase: 02-location-service
plan: "01"
subsystem: location-service
tags: [go, kafka, franz-go, chi, tdd, http, validation]
dependency_graph:
  requires: []
  provides: [kafka-producer-interface, post-location-handler, go-module]
  affects: [02-02, 02-03, 02-04, 02-05]
tech_stack:
  added:
    - github.com/go-chi/chi/v5 v5.2.5
    - github.com/twmb/franz-go v1.20.7
    - github.com/twmb/franz-go/pkg/kadm v1.17.2
    - github.com/redis/go-redis/v9 v9.18.0
    - github.com/prometheus/client_golang v1.23.2
    - github.com/alicebob/miniredis/v2 v2.37.0
  patterns:
    - constructor-injection (kafka.Producer interface injected into Handler)
    - pointer-fields-for-optional-json (*float64 to distinguish absent from zero)
    - tdd-red-green (tests written before implementation)
key_files:
  created:
    - location-service/go.mod
    - location-service/go.sum
    - location-service/internal/kafka/producer.go
    - location-service/internal/handler/location.go
    - location-service/internal/handler/location_test.go
  modified: []
decisions:
  - "Used *float64 pointer fields for lat/lng/bearing/speed_kmh to correctly handle equatorial zero-value coordinates"
  - "Producer interface defined in kafka package enables mockProducer in handler tests without live Kafka"
  - "emitted_at validated as RFC3339 via time.Parse — catches malformed timestamps early at ingest"
  - "LeaderAck + DisableIdempotentWrite paired to avoid franz-go construction error (Pitfall 1)"
metrics:
  duration_seconds: 205
  completed_date: "2026-03-05"
  tasks_completed: 2
  files_created: 5
---

# Phase 2 Plan 1: Go Module Scaffold and POST /location Handler Summary

**One-liner:** Pure-Go location-service module with franz-go sync Kafka producer and chi HTTP handler implementing POST /location with pointer-field validation and TDD test coverage.

## What Was Built

The foundation of the location-service Go module: a franz-go Kafka producer wrapper and a chi HTTP handler for POST /location with full payload validation and Kafka publish.

### Task 1: Go module and Kafka producer

- Created `go.mod` with module `mototaxi/location-service` (go 1.24)
- Installed all required dependencies: chi, franz-go (kgo + kadm), go-redis/v9, prometheus/client_golang, miniredis
- Implemented `Producer` interface in `internal/kafka/producer.go`: `Publish`, `Ping`, `Close`
- `franzProducer` wraps `*kgo.Client` with `LeaderAck() + DisableIdempotentWrite() + RecordDeliveryTimeout(5s)`
- `Publish` uses `ProduceSync` keyed by `driver_id` to the configured topic
- `Ping` uses `kadm.ListBrokers` to verify broker connectivity

### Task 2: POST /location handler with TDD

- RED: wrote all 11 `TestPostLocation*` test cases using `httptest` and a `mockProducer`
- GREEN: implemented `internal/handler/location.go` to pass all tests
- `locationPayload` uses `*float64` for lat, lng, bearing, speed_kmh (Pitfall 2 avoidance)
- `validate()` method on the payload struct enforces all field and range constraints
- `emitted_at` validated as RFC3339 timestamp
- `HandlePostLocation`: decode → validate → re-encode → kafka.Publish → 200/400/503
- `writeError` helper writes JSON `{"error": "..."}` with appropriate status code

## Commits

| Hash | Description |
|------|-------------|
| 45ee253 | feat(02-01): scaffold Go module and define Kafka producer |
| 02e279d | feat(02-01): implement POST /location handler with validation and Kafka publish |

## Success Criteria

- [x] `go build ./internal/kafka/...` exits 0
- [x] `go test ./internal/handler/ -run TestPostLocation -count=1` exits 0 with 11 tests passing
- [x] `go test ./... -count=1` exits 0
- [x] No CGO-requiring libraries in go.mod (confluent absent)
- [x] locationPayload uses pointer fields for lat/lng/bearing/speed_kmh (4 occurrences of `*float64`)
- [x] TestPostLocationZeroCoords returns 200 (zero coords are valid)

## Decisions Made

1. **Pointer fields for numeric payload fields:** `*float64` for lat, lng, bearing, speed_kmh enables nil-check for "field absent" vs zero-value. Critical for `TestPostLocationZeroCoords` passing correctly (equatorial coordinates are valid).

2. **Producer interface in kafka package:** Placing the `Producer` interface in `internal/kafka/producer.go` allows handler tests to use a `mockProducer` without any test-specific files in the kafka package. The mock in `location_test.go` satisfies the interface via duck typing.

3. **emitted_at RFC3339 validation:** Chose to parse with `time.RFC3339` (not just non-empty check) to catch malformed timestamps at the ingest boundary before they propagate downstream.

4. **LeaderAck + DisableIdempotentWrite:** Paired explicitly per RESEARCH.md Pitfall 1 — franz-go construction fails if LeaderAck is set without DisableIdempotentWrite because idempotent production requires AllISRAcks.

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

All 5 files verified on disk. Both commits (45ee253, 02e279d) confirmed in git log.

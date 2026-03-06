# Mototaxi

> **Work in progress** — Not all services are implemented yet.

Real-time driver tracking simulation (99Food / Uber Eats style).

## Goal

Build a production-inspired system where N drivers move around Sao Paulo, each exclusively assigned to one customer. Driver locations fan out to customer WebSocket connections via Kafka and horizontally scaled Elixir push servers. All observable through Grafana.

## Stack

| Component | Technology | Why |
|-----------|------------|-----|
| Location Service | Go | Goroutines handle concurrent HTTP emitters efficiently. Stateless ingest benefits from thin, fast binary. |
| Push Server | Elixir/Phoenix | BEAM processes use ~2KB each — ideal for 10k+ long-lived WebSocket connections. Phoenix.PubSub handles fan-out natively. Broadway provides backpressure-aware Kafka consumption. |
| Message Queue | Redpanda | Decouples location ingest from WebSocket delivery. Kafka's ordered, partitioned topics ensure each customer's messages stay in sequence. Enables horizontal scaling — multiple push server replicas can consume independently via partition assignment. Backpressure handled by consumer lag. |
| State / PubSub | Redis | Dual role: (1) stores driver-customer assignments as fast lookup cache, (2) Phoenix.PubSub adapter enables cross-replica pub/sub so any push-server can publish to any subscriber. |
| Load Balancer | Nginx | Least_conn routing for location-service, ip_hash sticky sessions for push-servers (convenience optimization — PubSub ensures correctness). |
| Frontend | Static HTML + Leaflet.js | No API keys required — uses OpenStreetMap tiles. |
| Observability | Prometheus + Grafana | Unified metrics from all services. |
| Stress Testing | k6 | Pushes metrics via remote write to same Prometheus — critical for correlating load with service behavior. |

## Quick Setup

```bash
docker compose up
```

- Frontend: `http://localhost/track/{customer_id}` — single driver view
- Frontend: `http://localhost/overview` — dispatcher view
- Grafana: `http://localhost:3000` (admin/admin)

## Architecture

```
Simulator → Nginx → Location Service → Kafka → Push Server → WebSocket → Browser
                Redis (state + PubSub)
```

## Environment

Key variables in `.env`:

- `DRIVER_COUNT` — number of simulated drivers
- `EMIT_INTERVAL_MS` — GPS update frequency
- `LOCATION_SERVICE_REPLICAS` — horizontal scaling
- `PUSH_SERVER_REPLICAS` — horizontal scaling
- `PARTITION_MULTIPLIER` — tunes Broadway parallelism per replica

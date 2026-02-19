# Grafana + Prometheus for AnyCable-Go

Local monitoring setup with Prometheus, Grafana, Redis, and Redis Exporter.

## Prerequisites

- Docker & Docker Compose
- [Dip](https://github.com/bibendi/dip)

## Usage

Start all services:

```sh
cd etc/grafana
dip up grafana
```

Open Grafana at http://localhost:3100 (no login required).

Run AnyCable-Go on the host with metrics enabled:

```sh
anycable-go --metrics_http="/metrics"
```

## Services

| Service         | Port  | Description                              |
|-----------------|-------|------------------------------------------|
| Grafana         | 3100  | Dashboards UI                            |
| Prometheus      | 9090  | Metrics storage, scrapes AnyCable+Redis  |
| Redis           | 6379  | Redis server                             |
| Redis Exporter  | 9121  | Exports Redis metrics for Prometheus     |

## Dashboards

Pre-provisioned dashboards are stored in `dashboards/`. Edits made in Grafana UI persist back to this directory.

- **AnyCable** — clients, messages, broadcasts, errors, goroutines
- **Redis** — memory, clients, ops/sec, hit rate, keys

## Prometheus targets

- `anycable` — scrapes `host.docker.internal:8080` (AnyCable on the host)
- `redis` — scrapes `redis-exporter:9121` (Redis exporter in Docker)

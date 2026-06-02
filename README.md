# notification-service

[![Security](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)
[![Reliability](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)
[![Maintainability](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)
[![Duplications](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=duplicated_lines_density)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=coverage)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)

> **Purpose:** Async notification dispatcher — consumes domain events and sends driver SMS messages.
> **Author:** Farid Triwicaksono

## Architecture Overview

![Architecture](docs/PintarParkir.architecture.svg)

## E2E Flow

![Flow Diagram](docs/flow.diagram.svg)

## Sequence Diagrams

- [Notification Flow](docs/sequence-diagrams/06-notification-flow.md)

## Tech Stack

- Go 1.25 + Gin (HTTP) + gRPC
- PostgreSQL (pgcrypto for PII encryption)
- Redis (caching + distributed locks)
- RabbitMQ (async event-driven via outbox pattern)
- Cloud Run (GCP) with auto-scaling
- OpenTelemetry (traces + metrics)

**Service-specific:** Pure event consumer (no REST API except /health), SMS via Telkomsel/stub, DLQ handling

## API

See [OpenAPI Specification](docs/api-specifications/openapi-spec.yaml) and [AsyncAPI Specification](docs/api-specifications/asyncapi-spec.yaml).

## Running Locally

```bash
cp configs/.env.example configs/.env
make run
```

## Testing

```bash
make test          # unit tests
make test-coverage # with coverage report
```

## Deployment

CD via GitHub Actions → GCP Cloud Run (asia-southeast1).
Triggers on push to `main`.

Cloud Run URL: `https://notification-service-725nddkmwq-as.a.run.app`

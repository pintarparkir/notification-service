# notification-service

> **Purpose:** Async notification dispatcher — consumes domain events and sends driver SMS messages.  
> **Author:** Farid Triwicaksono · **Last Updated:** 2026-05-21

## Project Overview

**ParkirPintar** is a backend mini-app for smart parking within a super-app. It handles:
- Availability queries (spots per floor, per vehicle type)
- Reservation creation (system-assigned or user-selected spots)
- Reservation state transitions (confirm, cancel, check-in, check-out)
- Geofence validation (GPS-based check-in)
- No-show expiration (automatic after 1 hour hold)
- Event publishing (outbox pattern → RabbitMQ)

Five services: **user**, **reservation**, **billing**, **payment**, **notification** (this service).

## Service Scope

**Owns:**
- Event-driven SMS dispatch
- Message template rendering
- Driver MSISDN resolution via user-service gRPC fallback
- Dead-letter queue (DLQ) inspection and replay tooling
- Consumer error classification (transient NACK vs permanent DLQ)

**Does NOT own:**
- Driver profile storage (user-service owns)
- Reservation state (reservation-service owns)
- Billing/payment state (billing/payment services own)
- Public REST/gRPC API (worker-only service)

**Key invariants:**
- Notification failure must not block reservation/billing/payment flows
- Consumers are idempotent and replay-safe
- Permanent poison messages go to DLQ
- MSISDN from event payload preferred; user-service lookup used as fallback

## At a Glance

| Aspect | Details |
|--------|---------|
| **REST Port** | N/A (consumer-only) |
| **gRPC Port** | N/A (client-only) |
| **Database** | N/A |
| **Cache** | N/A |
| **Message Queue** | RabbitMQ 3.13 (consumer + DLQ) |
| **External APIs** | user-service gRPC, SMS gateway/stub |

## Tech Stack

- **Language:** Go 1.22
- **Worker Runtime:** RabbitMQ consumer
- **Message Queue:** RabbitMQ 3.13 (amqp091-go)
- **External Client:** user-service gRPC, SMS gateway/stub
- **Logging:** Zap + Lumberjack
- **Observability:** OpenTelemetry (OTLP/gRPC)
- **Testing:** testify/mock, table-driven tests
- **CLI:** DLQ tool (`cmd/dlq`)

## Architecture

### High-Level Design
See [`../docs/architecture/high-level-design/05-notification-service.md`](../docs/architecture/high-level-design/05-notification-service.md) for:
- Service responsibilities and boundaries
- Async-only event consumption model
- Failure modes and DLQ strategy

### Low-Level Design
See [`../docs/architecture/low-level-design/05-notification-service-lld.md`](../docs/architecture/low-level-design/05-notification-service-lld.md) for:
- Consumer → usecase → template → SMS client flow
- Error classification rules
- MSISDN resolution behavior

### Entity Relationship Diagram
Notification-service owns no database tables. See [`../docs/architecture/erd/00-overview.md`](../docs/architecture/erd/00-overview.md) for cross-service data ownership.

![ParkirPintar ERD](../user-service/ERD.jpg)

## API Reference

### Public API
Notification-service has no public REST or gRPC API. It runs as an event consumer.

### RabbitMQ Events (consumed)

| Event | Trigger | SMS Message |
|-------|---------|-------------|
| `reservation.created.v1` | Reservation created | Reservation hold opened |
| `reservation.confirmed.v1` | Reservation confirmed | Spot confirmed + check-in instruction |
| `reservation.cancelled.v1` | Reservation cancelled | Cancellation confirmation |
| `reservation.expired.v1` | No-show expiration | Reservation expired/no-show notice |
| `reservation.checked_out.v1` | Check-out succeeds | Session completed notice |
| `invoice.closed.v1` | Invoice closed | Invoice total + payment instruction |
| `payment.paid.v1` | Payment settled | Payment receipt |
| `payment.failed.v1` | Payment failed/expired | Payment failure notice |

### DLQ CLI

| Command | Purpose |
|---------|---------|
| `make dlq-list` | Inspect first 20 DLQ messages |
| `make dlq-replay` | Replay up to 50 DLQ messages back to main exchange |
| `make dlq -- list --limit 5` | Pass custom args to DLQ CLI |

## Sample Environment

```bash
# ── App ─────────────────────────────────────────────────────────────────────
APP_NAME=notification-service
APP_ENV=local

# ── RabbitMQ (event consumer + DLQ) ──────────────────────────────────────────
RABBIT_URL=amqp://guest:guest@localhost:5672/
RABBIT_EXCHANGE=parkirpintar.events
RABBIT_QUEUE=notification.events
RABBIT_DLQ=notification.events.dlq

# ── user-service gRPC (MSISDN lookup fallback) ───────────────────────────────
USER_GRPC_ADDR=localhost:9094
USER_GRPC_TIMEOUT=2s

# ── Observability ────────────────────────────────────────────────────────────
OTLP_ENDPOINT=localhost:4317

# ── SMS gateway ──────────────────────────────────────────────────────────────
SMS_MODE=stub
SMS_SENDER_ID=ParkirPintar
```

See `configs/.env.example` for full reference.

## Getting Started

### Prerequisites
- Docker 24+ & Docker Compose v2
- Go 1.22+ (for local development)
- RabbitMQ running locally
- user-service running if event payload does not include MSISDN

### Local Development

```bash
# 1. Clone and setup
git clone <repo> && cd <repo>
cd notification-service
cp configs/.env.example configs/.env

# 2. Start shared infra (see https://github.com/pintarparkir/infra)
cd ../infra && podman compose up -d

# 3. Start user-service for MSISDN fallback
cd ../user-service && make run

# 4. Run notification consumer
cd ../notification-service
make run
```

## Testing

### Unit Tests (no infra)
```bash
make test-unit
```
Covers: template rendering, event parsing, MSISDN fallback behavior, error classification.

### All Tests
```bash
make test
```

### Coverage
```bash
go test -coverprofile=cov.out ./...
go tool cover -html=cov.out
```
Target: usecase ≥80%.

## Debugging

### Logs
```bash
LOG_LEVEL=debug make run
```
Logs are JSON-formatted with trace_id, span_id, request_id, event_type.

### RabbitMQ
- **Management UI:** http://localhost:15672 (guest/guest)
- **View exchange:** parkirpintar.events
- **View main queue:** notification.events
- **View DLQ:** notification.events.dlq

### DLQ Inspection
```bash
# List first 20 failed messages
make dlq-list

# Replay up to 50 failed messages
make dlq-replay

# Custom limit
make dlq -- list --limit 5
```

### SMS Stub Mode
```bash
# Stub mode logs SMS payload instead of sending real SMS
SMS_MODE=stub make run
```

### user-service Lookup
```bash
# Verify user-service is reachable
grpcurl -plaintext localhost:9094 grpc.health.v1.Health/Check
```

## Operations

### Health Checks
Notification-service has no public HTTP/gRPC health endpoint. Operational health is inferred from:
- RabbitMQ consumer connection status
- Queue depth (`notification.events`)
- DLQ depth (`notification.events.dlq`)
- Logs with successful `message ack` events

### Queue Monitoring
```bash
# Use RabbitMQ UI, or inspect queue depth via rabbitmqctl
rabbitmqctl list_queues name messages consumers
```

### Replay Policy
Replay DLQ messages only after fixing root cause. Poison messages with invalid schema should remain in DLQ for audit.

## Security Notes

- **Secrets:** Never commit `.env` files. Use Secret Manager in production.
- **PII:** MSISDN is PII; redact from logs or log only masked form.
- **Consumer safety:** Permanent parsing/validation errors go to DLQ, transient downstream errors are retried.
- **SMS provider:** Real SMS gateway credentials must come from Secret Manager.

## Related Documentation

- **Architecture Overview:** [`../docs/README.md`](../docs/README.md)
- **Shared Infra Docs:** [`infra`](https://github.com/pintarparkir/infra)
- **API Documentation:** [`../docs/api-documentation/00-overview.md`](../docs/api-documentation/00-overview.md)
- **Implementation Backlog:** [`../docs/implementation-todo/00-backlog.md`](../docs/implementation-todo/00-backlog.md)

---

_For questions or issues, refer to the troubleshooting section in the main README or open an issue on the repo._

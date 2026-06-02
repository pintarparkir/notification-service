# notification-service

[![Security](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)
[![Reliability](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)
[![Maintainability](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)
[![Duplications](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=duplicated_lines_density)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_notification-service&metric=coverage)](https://sonarcloud.io/summary/new_code?id=pintarparkir_notification-service)

**Cloud Run:** `https://notification-service-725nddkmwq-as.a.run.app`

## Architecture Overview

![Architecture](docs/PintarParkir.architecture.svg)

## E2E Flow

![Flow Diagram](docs/flow.diagram.svg)

## Sequence Diagrams

### Notification Flow

```mermaid
sequenceDiagram
    autonumber
    participant 🔧 as 🔧 RabbitMQ
    participant 💾 as 💾 Notification Service
    participant 🧠 as 🧠 MSISDN Cache
    participant 💾_User as 💾 User Service
    participant 📱 as 📱 SMS Gateway
    participant 👤 as 👤 Driver
    
    Note over 🔧,👤: ──────────────────────────────────────────────
    FLOW: reservation.confirmed.v1 → SMS Dispatch
    Note over 🔧,👤: ──────────────────────────────────────────────
    
    %% Event arrives from outbox publisher
    🔧->>💾: DELIVER reservation.confirmed.v1<br/>{reservation_id, driver_id, spot_id, hold_end}
    activate 💾
    
    %% Step 1: Parse event payload
    💾->>💾: Parse JSON body<br/>alt Parse error (invalid JSON)
        💾->>🔧: ACK + Move to DLQ
        Note right of 💾: Permanent error, no retry
        deactivate 💾
        return
    end
    
    %% Step 2: Extract driver_id
    alt Missing required field (driver_id)
        💾->>🔧: ACK + Move to DLQ
        Note right of 💾: Data integrity issue
        deactivate 💾
        return
    end
    
    %% Step 3: MSISDN resolution
    💾->>🧠: GET cache:user:{driver_id}
    
    alt Cache hit (phone found)
        🧠-->>💾: phone_e164
        Note right of 💾: Cache TTL = 5min
    else Cache miss
        %% Step 3b: Fallback to user-service
        💾->>💾_User: gRPC GetUserById(driver_id)
        activate 💾_User
        
        alt User found
            💾_User-->>💾: phone_e164 (decrypted via pgcrypto)
            💾->>🧠: SET cache:user:{driver_id} = phone_e164 TTL=5min
        else User NOT_FOUND
            💾_User-->>💾: error=NOT_FOUND
            💾->>🔧: ACK + Move to DLQ
            Note right of 💾: Permanent error (missing driver)
            deactivate 💾_User
            deactivate 💾
            return
        end
        
        deactivate 💾_User
    end
    
    %% Step 4: Render SMS template
    💾->>💾: Render template for event_type='reservation.confirmed.v1'
    Note right of 💾: Template: "Reservasi spot {spot_id} dikonfirmasi. Silakan check-in sebelum {hold_end}."
    💾->>💾: message = "Reservasi spot F2-C-014 dikonfirmasi. Silakan check-in sebelum 09:10."
    
    %% Step 5: Send SMS via gateway
    💾->>📱: POST /send_sms {to: "+628123456789", message: "...", sender_id: "ParkirPintar"}
    activate 📱
    
    alt SMS 2xx Success
        📱-->>💾: 200 OK {message_id: "msg-123"}
        💾->>🔧: ACK
        Note right of 💾: Message processed successfully
        deactivate 📱
    else SMS 4xx Client Error
        %% Invalid phone, blocked number, etc.
        📱-->>💾: 400 Bad Request {error: "invalid destination"}
        💾->>🔧: ACK + Move to DLQ
        Note right of 💾: Permanent error (won't recover on retry)
        deactivate 📱
    else SMS 5xx Server Error
        %% Telkomsel gateway down, rate limit, etc.
        📱-->>💾: 503 Service Unavailable
        💾->>🔧: NACK + REQUEUE
        Note right of 💾: Transient error, will retry
        deactivate 📱
    end
    
    deactivate 💾
    
    Note over 🔧,👤: ──────────────────────────────────────────────
    Summary: ✅ SMS sent successfully
    Note over 🔧,👤: Total latency: < 30s (SLO target)
    Note over 🔧,👤: Driver receives: "Reservasi spot F2-C-014 dikonfirmasi..."
    Note over 🔧,👤: ──────────────────────────────────────────────
```
```mermaid
sequenceDiagram
    autonumber
    participant 🔧 as 🔧 RabbitMQ
    participant 💾 as 💾 Notification Service
    participant 🧠 as 🧠 MSISDN Cache
    participant 💾_User as 💾 User Service
    participant 🗑️ as 🗑️ DLQ (notification.events.dlq)
    participant 👤 as 👤 Driver
    
    Note over 🔧,👤: ──────────────────────────────────────────────
    SCENARIO: Permanent Errors → DLQ
    Note over 🔧,👤: ──────────────────────────────────────────────
    
    🔧->>💾: DELIVER reservation.confirmed.v1<br/>{driver_id: "invalid"}
    activate 💾
    
    %% Missing driver_id
    💾->>💾: Parse event
    alt Missing driver_id
        💾->>🔧: ACK + Move to DLQ
        Note right of 💾: Schema violation
        deactivate 💾
        return
    end
    
    %% Try cache
    💾->>🧠: GET cache:user:invalid
    🧠-->>💾: miss
    
    %% Fallback to user-service
    💾->>💾_User: gRPC GetUserById("invalid")
    activate 💾_User
    
    💾_User-->>💾: NOT_FOUND
    deactivate 💾_User
    
    %% Permanent error → DLQ
    💾->>🔧: ACK + Move to DLQ
    Note right of 💾: Will not recover with retry
    deactivate 💾
    
    %% DLQ inspection
    Note over 🗑️,👤: DLQ now contains this message for manual review
    participant 💻 as 💻 Operator
    
    💻->>🗑️: LIST DLQ messages
    🗑️-->>💻: [{event: reservation.confirmed.v1, reason: missing driver_id}]
    
    %% After investigation, operator replays
    alt Root cause fixed (driver created)
        💻->>🔧: REPLAY DLQ message to main exchange
        Note right of 💻: After driver onboarding
        🔧-->💾: DELIVER reservation.confirmed.v1 (replayed)
        %% Flow proceeds normally from here
    else Root cause not fixed
        💻->>🗑️: IGNORE (remain in DLQ for audit)
    end
```
```mermaid
sequenceDiagram
    autonumber
    participant 🔧 as 🔧 RabbitMQ
    participant 💾 as 💾 Notification Service
    participant 💾_User as 💾 User Service
    participant 📱 as 📱 SMS Gateway
    participant 👤 as 👤 Driver
    
    Note over 🔧,👤: ──────────────────────────────────────────────
    SCENARIO: Transient Error → Retry → Success
    Note over 🔧,👤: ──────────────────────────────────────────────
    
    loop Retry Loop (max 3 attempts)
        🔧->>💾: DELIVER reservation.confirmed.v1
        activate 💾
        
        %% Processing...
        💾->>🧠: Cache miss
        💾->>💾_User: gRPC GetUserById(driver_id)
        activate 💾_User
        
        %% Simulate timeout on 1st attempt
        alt Attempt == 1
            💾_User-->>💾: context deadline exceeded (timeout after 2s)
            deactivate 💾_User
            
            %% Transient error → NACK
            💾->>🔧: NACK + REQUEUE
            Note right of 💾: Will retry after backoff
            deactivate 💾
            
        else Attempt >= 2
            %% Success on retry
            💾_User-->>💾: phone_e164
            deactivate 💾_User
            
            %% Render and send SMS
            💾->>📱: POST /send_sms
            activate 📱
            📱-->>💾: 200 OK
            💾->>🔧: ACK
            Note right of 💾: Success on retry
            deactivate 📱
            deactivate 💾
            return
        end
    end
    
    %% Max retries exceeded → DLQ
    💾->>🔧: ACK + Move to DLQ
    Note right of 💾: Exhausted retry attempts
    deactivate 💾
```

<details>
<summary>More sequence diagrams</summary>

- [All Sequence Diagrams](docs/sequence-diagrams/)
</details>

---



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

## Business Flow Logic

### Notification Service: Event Consumer Flow

Notification-service adalah **consumer-only** service yang menerima domain events dari RabbitMQ dan mengirim SMS ke driver.

```mermaid
sequenceDiagram
    autonumber
    participant RMQ as RabbitMQ
    participant Notif as Notification Service
    participant Cache as MSISDN Cache<br/>(in-memory, 5min)
    participant UserSvc as User Service
    participant SMS as SMS Gateway<br/>(Telkomsel/Stub)
    participant DLQ as Dead Letter Queue
    
    Note over RMQ,DLQ: Consume Event: reservation.confirmed.v1
    
    RMQ-->Notif: DELIVER reservation.confirmed.v1<br/>{reservation_id, driver_id, spot_id}
    activate Notif
    
    Notif->>Notif: Parse event payload
    
    alt Parse error (bad JSON)
        Notif->>RMQ: ACK + Move to DLQ
        Note right of Notif: Permanent error, no retry
        deactivate Notif
        return
    end
    
    %% MSISDN resolution
    Notif->>Cache: GET driver_id
    
    alt Cache hit
        Cache-->>Notif: phone_e164
    else Cache miss
        Notif->>UserSvc: gRPC GetUserById(driver_id)
        activate UserSvc
        
        alt User found
            UserSvc-->>Notif: phone_e164
            Notif->>Cache: SET driver_id → phone_e164 TTL=5min
        else User not found
            UserSvc-->>Notif: NOT_FOUND
            Notif->>RMQ: ACK + Move to DLQ
            Note right of Notif: Permanent error
            deactivate UserSvc
            deactivate Notif
            return
        end
        
        deactivate UserSvc
    end
    
    %% Render template
    Notif->>Notif: Render template:<br/>"Reservasi spot {spot_id} dikonfirmasi<br/>sampai {hold_end}"
    
    %% Send SMS
    Notif->>SMS: POST /send {to: phone_e164, message: ...}
    activate SMS
    
    alt SMS 2xx success
        SMS-->>Notif: 200 OK {message_id}
        Notif->>RMQ: ACK
        Note right of Notif: Message processed successfully
    else SMS 4xx (client error)
        SMS-->>Notif: 400 Bad Request
        Notif->>RMQ: ACK + Move to DLQ
        Note right of Notif: Permanent error (invalid phone)
    else SMS 5xx (server error)
        SMS-->>Notif: 500 Internal Error
        Notif->>RMQ: NACK + REQUEUE
        Note right of Notif: Transient error, will retry
    end
    
    deactivate SMS
    deactivate Notif
    
    Note over RMQ,DLQ: DLQ Replay (Manual Tool)
    
    participant CLI as DLQ CLI Tool
    
    CLI->>RMQ: LIST messages in notification.events.dlq
    RMQ-->>CLI: [msg1, msg2, ...]
    
    CLI->>RMQ: REPLAY msg1 to notification.events
    Note right of CLI: After fixing root cause
```

### Event → SMS Template Mapping

| Event | SMS Template (Indonesian) |
|-------|---------------------------|
| `reservation.confirmed.v1` | "Reservasi spot {spot_id} dikonfirmasi. Check-in sebelum {hold_end}." |
| `reservation.cancelled.v1` | "Reservasi Anda telah dibatalkan." |
| `reservation.expired.v1` | "Reservasi expired (no-show). Fee Rp {fee} dikenakan." |
| `billing.invoice.closed.v1` | "Invoice #{id} total Rp {total}. Silakan bayar via mini-app." |
| `payment.paid.v1` | "Pembayaran berhasil Rp {amount}. Terima kasih!" |
| `payment.failed.v1` | "Pembayaran gagal: {reason}. Silakan coba lagi." |

### Error Classification

| Error Type | Action | Rationale |
|------------|--------|-----------|
| JSON parse fail | ACK → DLQ | Permanent, no remediation |
| Unknown routing key | ACK → drop | Not our message |
| Empty MSISDN | ACK → DLQ | Data issue, needs manual fix |
| UserSvc NOT_FOUND | ACK → DLQ | Missing driver, needs investigation |
| UserSvc timeout | NACK → requeue | Transient, will retry |
| SMS gateway 4xx | ACK → DLQ | Invalid phone, permanent |
| SMS gateway 5xx | NACK → requeue | Provider issue, transient |

---

## Related Documentation

- **Architecture Overview:** [`../docs/README.md`](../docs/README.md)
- **Shared Infra Docs:** [`infra`](https://github.com/pintarparkir/infra)
- **API Documentation:** [`../docs/api-documentation/00-overview.md`](../docs/api-documentation/00-overview.md)
- **Implementation Backlog:** [`../docs/implementation-todo/00-backlog.md`](../docs/implementation-todo/00-backlog.md)

---

_For questions or issues, refer to the troubleshooting section in the main README or open an issue on the repo._

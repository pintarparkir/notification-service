# notification-service

Async event consumer. Subscribes to RabbitMQ events and dispatches SMS to drivers
via the Telkomsel SMS API (stub in dev).

## Responsibilities
- Subscribe to `reservation.confirmed`, `reservation.expired`, `reservation.cancelled`,
  `billing.invoice.closed`, `payment.paid` events
- Resolve driver MSISDN by calling `user-service` gRPC (`GetUserById`)
- Send SMS via `pkg/sms` client (stub → real Telkomsel gateway in production)

## Service-to-service dependencies
| Dependency    | Protocol | Purpose                  |
|---------------|----------|--------------------------|
| user-service  | gRPC     | MSISDN lookup by driver_id |
| RabbitMQ      | AMQP     | Event consumption          |

## Event routing
| Routing key                  | SMS triggered                    |
|------------------------------|----------------------------------|
| `reservation.confirmed.v1`   | Spot confirmed + check-in window |
| `reservation.expired.v1`     | No-show notification             |
| `reservation.cancelled.v1`   | Cancellation confirmation        |
| `billing.invoice.closed.v1`  | Invoice + payment request        |
| `payment.paid.v1`            | Payment receipt                  |

## Run
```bash
cp configs/.env.example configs/.env
go run cmd/main.go
```

# High-Level Design — notification-service

Async-only. No public REST. Subscribes to RabbitMQ events and dispatches SMS to
drivers. Resolves MSISDN per-event via gRPC → user-service.

## Position

```
            RabbitMQ (parkirpintar.events)
                  │  (5 routing keys)
                  ▼
           notification-service
                  │
                  │ gRPC ──▶ user-service.GetUserById (resolve MSISDN)
                  │
                  ▼
            SMS gateway
            (stub in dev / Telkomsel in prod)
```

## Subscriptions

| Routing key                  | Template                                |
|------------------------------|-----------------------------------------|
| `reservation.confirmed.v1`   | "Spot {spot_id} confirmed. Check in within 1 hour."|
| `reservation.cancelled.v1`   | "Reservation {id} cancelled."           |
| `reservation.expired.v1`     | "You missed your check-in window."      |
| `billing.invoice.closed.v1`  | "Invoice {id}: total Rp{total_idr}. Pay via QRIS." |
| `payment.paid.v1`            | "Payment received. Thank you."          |

## Failure semantics

- RabbitMQ delivers at-least-once. Templates are pure strings → re-sending the
  same SMS is acceptable (driver may receive a duplicate; not catastrophic).
- gRPC call to user-service has a 2 s deadline + circuit breaker. If user-service
  is down, the consumer NACKs the message → RabbitMQ requeues with exponential
  backoff (3 retries, then DLQ).
- SMS gateway 4xx → log, DO NOT retry (likely bad MSISDN).
- SMS gateway 5xx → NACK + requeue.

## No outbound state

This service is *stateless* — no Postgres, no Redis. Just a RabbitMQ consumer
loop and a gRPC client. Cloud Run scale-to-zero is safe.

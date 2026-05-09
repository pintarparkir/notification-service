# Feature 01 — RabbitMQ consumer

**Status:** ✅ shipped
**Owner:** notification-service

## Scope

Subscribe to the topic exchange `parkirpintar.events` with 5 routing keys.
One queue per service replica (durable, named) so consumers can be load-balanced.

## Topology

```
exchange: parkirpintar.events (topic, durable)
queue:    notification.events (durable, prefetch=10)
bindings:
  - reservation.confirmed.v1
  - reservation.cancelled.v1
  - reservation.expired.v1
  - billing.invoice.closed.v1
  - payment.paid.v1
```

DLQ:
```
exchange: parkirpintar.events.dlx (direct)
queue:    notification.events.dlq (durable)
binding:  notification.events → dlx via x-dead-letter-exchange
```

## Tasks

- [ ] `pkg/rabbit` connection + channel pool
- [ ] Bootstrap topology (exchange + queue + bindings) on startup
- [ ] Consumer loop: parse routing key → dispatch to usecase method
- [ ] ACK on success, NACK with requeue=false on parse error → DLQ
- [ ] NACK with requeue=true on transient errors (gRPC timeout, SMS 5xx)
- [ ] Metric: `notif_handled_total{routing_key=…, outcome=ack|nack|dlq}`

## Acceptance criteria

- Killing the consumer mid-message: message is redelivered on restart.
- Malformed JSON payload → DLQ; metric increments.
- 100 events/min sustained with no message loss across consumer restarts.

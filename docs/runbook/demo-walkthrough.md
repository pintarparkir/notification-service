# Demo Walkthrough — notification-service

End-to-end: subscribe → resolve MSISDN → render Indonesian SMS body → SMS-stub
prints. Plus DLQ verification and the `cmd/dlq` tool.

## Setup

```bash
# 1. Shared infra
cd ../infra && podman compose up -d           # postgres, redis, otel
brew services start rabbitmq                  # rabbit on the host (see ../infra/README)

# 2. user-service must be up — notification calls it via gRPC for MSISDN
cd ../user-service && cp configs/.env.example configs/.env && make migrate-up && make run &

# 3. notification-service
cd ../notification-service
cp configs/.env.example configs/.env
make run
```

You should see:
```
consumer: subscribing  queue=notification.events  dlq=notification.events.dlq
                       routing_keys=[reservation.confirmed.v1 ... payment.paid.v1]
```

## Scenario 1 — Pure unit tests (offline)

```bash
make test
```

You'll see:
- `TestRender_AllSupportedRoutingKeys`  — every key produces non-empty Indonesian text containing the right substring
- `TestRender_UnknownKeyReturnsEmpty`   — engine drops events it doesn't recognise
- `TestFormatIDR`                        — `25000 → "25.000"`, `1234567 → "1.234.567"`

## Scenario 2 — Live event flow (~1 min)

We need a real driver_id from user-service first:

```bash
PAYLOAD=$(printf '{"sub":"notif-demo","phone":"+628999777666","exp":9999999999}' | base64 | tr -d '=' | tr '/+' '_-')
TOKEN="eyJhbGciOiJSUzI1NiJ9.${PAYLOAD}.x"
DRIVER_ID=$(curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/me | jq -r .id)
echo "driver_id=$DRIVER_ID"
```

Helper to publish into RabbitMQ via the management API (port 15672, guest/guest):

```bash
publish() {
  local key="$1" payload="$2"
  curl -s -u guest:guest -H "Content-Type: application/json" \
    -X POST "http://localhost:15672/api/exchanges/%2F/parkirpintar.events/publish" \
    -d "{\"properties\":{\"content_type\":\"application/json\",\"delivery_mode\":2},
         \"routing_key\":\"$key\",\"payload\":$(jq -cn --arg p "$payload" '$p'),
         \"payload_encoding\":\"string\"}"; echo
}

publish reservation.confirmed.v1   "{\"driver_id\":\"$DRIVER_ID\",\"spot_id\":\"F1-C-007\"}"
publish billing.invoice.closed.v1  "{\"driver_id\":\"$DRIVER_ID\",\"invoice_id\":\"inv-1\",\"total_idr\":25000}"
publish payment.paid.v1            "{\"driver_id\":\"$DRIVER_ID\"}"
```

In the notification-service log (or stdout) you'll see three lines like:
```
[SMS-STUB sender=ParkirPintar to=+628999777666] Reservasi Anda di slot F1-C-007 dikonfirmasi…
[SMS-STUB sender=ParkirPintar to=+628999777666] Tagihan Anda sebesar Rp25.000. Silakan bayar via QRIS…
[SMS-STUB sender=ParkirPintar to=+628999777666] Pembayaran diterima. Terima kasih…
```

**Talking points:**
- The driver lookup is cached for 5 minutes — three back-to-back events for the same driver hit user-service exactly once.
- Templates are in Indonesian and IDR amounts use thousand-separator dots (`Rp25.000`).
- The dispatcher is structured so each routing key maps cleanly to one template; adding `user.phone_changed.v1` later is one switch arm.

## Scenario 3 — Unknown routing key drops cleanly

```bash
publish user.phone_changed.v1 "{\"driver_id\":\"$DRIVER_ID\"}"
```

The management API returns `{"routed":false}` — no consumer is bound to that key,
so it's discarded by the broker. No SMS, no DLQ.

## Scenario 4 — Permanent failure → DLQ

Send an event without `driver_id`:

```bash
publish reservation.confirmed.v1 "{\"spot_id\":\"NOPE\"}"
```

In notification-service log:
```
consumer: missing driver_id  routing_key=reservation.confirmed.v1
```

Verify the DLQ caught it:

```bash
make dlq-list
# → [1] routing_key=reservation.confirmed.v1  body={"spot_id":"NOPE"}
```

The other failure modes route to DLQ the same way:
- gRPC `NOT_FOUND` from user-service (driver doesn't exist)
- Driver row exists but `phone_e164` is empty
- JSON parse error on the payload

Transient failures (gRPC timeout, connection refused, 5xx from SMS gateway)
NACK with requeue=true so RabbitMQ retries with backoff — they don't end up
in the DLQ.

## Scenario 5 — Replay from DLQ

After fixing the upstream issue (e.g. backfilling `driver_id` in the producer):

```bash
make dlq-replay
# → replayed N messages
```

Replayed messages are re-published to `parkirpintar.events` with their original
routing key (recovered from the broker's `x-death` header) and consumed normally.

## Scenario 6 — Purge

```bash
go run ./cmd/dlq purge --yes-i-know
# → purged N messages
```

Refuses without `--yes-i-know` — protects against accidental data loss.

## Cleanup

```bash
# Ctrl-C each running service, then:
cd ../infra && podman compose down
brew services stop rabbitmq
```

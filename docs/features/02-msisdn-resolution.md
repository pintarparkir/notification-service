# Feature 02 — MSISDN resolution

**Status:** ✅ shipped
**Owner:** notification-service

## Scope

For each event, resolve `driver_id → MSISDN` by gRPC call to
`user-service.GetUserById`. Phone is decrypted server-side (pgcrypto in user-service).

## Why per-event lookup vs caching

- MSISDN can change (super-app phone migration). Stale cache → SMS to old number.
- 5 events/sec at MVP — ~5 gRPC calls/sec. Negligible load on user-service.
- If volume grows, add a `pkg/redis` LRU cache here with 1 h TTL — invalidated by
  a hypothetical `user.phone_changed.v1` event.

## Tasks

- [ ] `pkg/grpcclient/user` — circuit breaker + 2 s timeout
- [ ] OTel propagation (event trace → MSISDN lookup → SMS dispatch)
- [ ] Test with mock user-service

## Acceptance criteria

- gRPC timeout → consumer NACKs with requeue=true.
- gRPC `NOT_FOUND` (driver row missing) → log + DLQ; do NOT retry.

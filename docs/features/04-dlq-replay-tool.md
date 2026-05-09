# Feature 04 — DLQ replay tool

**Status:** ✅ shipped
**Owner:** notification-service

## Scope

CLI subcommand to drain the dead-letter queue and re-publish messages back to
the main exchange after the underlying issue is fixed.

## Usage

```bash
notification-service dlq list                    # show messages with reason headers
notification-service dlq replay --limit 50       # republish up to 50 to main exchange
notification-service dlq purge                   # drop all (use with care)
```

## Tasks

- [ ] `cmd/notification dlq` cobra subcommand
- [ ] List shows: timestamp, routing_key, reason header, payload preview
- [ ] Replay re-publishes to `parkirpintar.events` with original routing_key
- [ ] Audit log to stdout for each replayed message

## Acceptance criteria

- A DLQ message replayed against a working SMS gateway is delivered exactly once
  (consumer dedup; or SMS retry).
- Purge requires `--yes-i-know` flag.

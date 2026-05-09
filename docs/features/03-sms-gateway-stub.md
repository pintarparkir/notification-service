# Feature 03 — SMS gateway abstraction

**Status:** ⏳ partial — stub client shipped (`pkg/sms/stub.go`); real Telkomsel client TBD

## Scope

`pkg/sms.Client` interface with one method: `Send(ctx, to, message) error`.
Two implementations:

- `stubClient` — logs to stdout. Used in dev + tests.
- `telkomselClient` — HTTP client to the Telkomsel SMS gateway. Used in prod.

The choice is made at boot from `APP_ENV`:

```go
if cfg.AppEnv == "production" {
    client = sms.NewTelkomsel(cfg.TelkomselURL, cfg.TelkomselAPIKey, cfg.SmsSenderID)
} else {
    client = sms.NewStub(cfg.SmsSenderID)
}
```

## Tasks

- [x] `Client` interface
- [x] `stubClient` (logs)
- [ ] `telkomselClient` (HTTP, retry-on-5xx, OTel-instrumented)
- [ ] Rate limit: 10 SMS/sec/sender (Telkomsel gateway constraint)

## Acceptance criteria

- Switching `APP_ENV=production` with valid Telkomsel creds sends a real SMS.
- Stub client never errors; real client surfaces 4xx as non-retryable, 5xx as retryable.

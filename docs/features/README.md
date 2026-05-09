# Features — notification-service

| File                              | Status | Summary                                       |
|-----------------------------------|--------|-----------------------------------------------|
| `01-rabbitmq-consumer.md`         | ✅     | 5 routing keys + DLQ binding + dispatcher     |
| `02-msisdn-resolution.md`         | ✅     | gRPC user-service.GetUserById, 5-min in-process cache |
| `03-sms-gateway-stub.md`          | ⏳     | Stub client shipped; real Telkomsel client TBD|
| `04-dlq-replay-tool.md`           | ✅     | `cmd/dlq` — list / replay / purge subcommands |

Plus: per-event-type Indonesian SMS templates with thousand-separator IDR formatting (5 cases, table-driven tests).

Legend: 📋 planned · ⏳ in progress · ✅ shipped · 🚫 deferred

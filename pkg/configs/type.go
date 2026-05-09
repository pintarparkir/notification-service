package configs

import "time"

// Config holds notification-service configuration.
// notification-service has no public REST/gRPC surface — it's purely an
// event consumer. So no AppPort / GrpcPort, and no Postgres (stateless).
type Config struct {
	AppName string `env:"APP_NAME" envDefault:"notification-service"`
	AppEnv  string `env:"APP_ENV" envDefault:"local"`

	// ── RabbitMQ (event consumer) ──
	RabbitURL      string `env:"RABBIT_URL" envDefault:"amqp://guest:guest@localhost:5672/"`
	RabbitExchange string `env:"RABBIT_EXCHANGE" envDefault:"parkirpintar.events"`
	RabbitQueue    string `env:"RABBIT_QUEUE" envDefault:"notification.events"`
	RabbitDLQ      string `env:"RABBIT_DLQ" envDefault:"notification.events.dlq"`

	// ── user-service gRPC (MSISDN lookup) ──
	UserGrpcAddr string `env:"USER_GRPC_ADDR" envDefault:"localhost:9094"`

	// gRPC dial timeout per call to user-service.
	UserGrpcTimeout time.Duration `env:"USER_GRPC_TIMEOUT" envDefault:"2s"`

	OTLPEndpoint string `env:"OTLP_ENDPOINT" envDefault:"localhost:4317"`

	// ── SMS gateway ──
	// SmsSenderID is the alphanumeric sender id shown on the recipient's handset.
	SmsSenderID string `env:"SMS_SENDER_ID" envDefault:"ParkirPintar"`

	// SmsMode picks the sms.Client implementation:
	//   "stub"     — logs only (default for local)
	//   "telkomsel" — real Telkomsel HTTP gateway (TBD, see roadmap)
	SmsMode string `env:"SMS_MODE" envDefault:"stub"`
}

// ConfigLoader controls the source of config (env file path, etc.).
type ConfigLoader struct {
	Env     string
	EnvFile string
}

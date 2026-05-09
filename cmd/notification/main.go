// notification-service entry point.
//
// Pure event consumer — no inbound REST or gRPC. Wires:
//   - configs → logger → otel
//   - gRPC client to user-service (for MSISDN lookup, with 5-min in-process cache)
//   - sms.Client (stub or real Telkomsel based on SMS_MODE)
//   - rabbit.Subscriber on parkirpintar.events with 5 routing keys + DLQ wiring
//   - dispatcher loop until ctx is cancelled
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/farid/notification-service/internal/notification/consumer"
	"github.com/farid/notification-service/internal/notification/model"

	"github.com/farid/notification-service/pkg/configs"
	"github.com/farid/notification-service/pkg/grpcclient"
	"github.com/farid/notification-service/pkg/logger"
	pkgOtel "github.com/farid/notification-service/pkg/otel"
	"github.com/farid/notification-service/pkg/rabbit"
	"github.com/farid/notification-service/pkg/sms"
)

func main() {
	cfg := configs.NewConfig(configs.ConfigLoader{Env: os.Getenv("PROJECT_ENV")})
	if err := logger.NewLogger(cfg.AppName, cfg.AppEnv); err != nil {
		panic(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	otel := pkgOtel.NewOpenTelemetry(cfg.OTLPEndpoint, "notification", cfg.AppEnv)
	defer func() { _ = otel.EndAPM() }()

	// ── user-service gRPC client ─────────────────────────────────────────────
	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	conn, err := grpcclient.Dial(dialCtx, cfg.UserGrpcAddr)
	dialCancel()
	if err != nil {
		logger.Fatal(ctx, "user-service grpc dial failed",
			map[string]interface{}{"addr": cfg.UserGrpcAddr, logger.ErrorKey: err.Error()})
	}
	defer conn.Close()
	users := grpcclient.NewUserClient(conn, cfg.UserGrpcTimeout)

	// ── SMS client ───────────────────────────────────────────────────────────
	var smsClient sms.Client
	switch cfg.SmsMode {
	case "telkomsel":
		// Real Telkomsel HTTP client — TBD per ROADMAP. Until then, fall back
		// to stub with a warning so the operator notices misconfiguration.
		logger.Warn(ctx, "SMS_MODE=telkomsel but real client not implemented yet; using stub", nil)
		smsClient = sms.NewStub(cfg.SmsSenderID)
	default:
		smsClient = sms.NewStub(cfg.SmsSenderID)
	}

	// ── RabbitMQ subscriber + DLQ ────────────────────────────────────────────
	sub, err := rabbit.NewSubscriber(
		cfg.RabbitURL, cfg.RabbitExchange, cfg.RabbitQueue,
		model.AllRoutingKeys(),
		rabbit.SubscriberOptions{DLQ: cfg.RabbitDLQ},
	)
	if err != nil {
		logger.Fatal(ctx, "rabbitmq subscriber init failed",
			map[string]interface{}{logger.ErrorKey: err.Error()})
	}
	defer sub.Close()

	disp := consumer.New(users, smsClient)

	logger.Info(ctx, "consumer: subscribing", map[string]interface{}{
		"queue":        cfg.RabbitQueue,
		"dlq":          cfg.RabbitDLQ,
		"routing_keys": model.AllRoutingKeys(),
	})

	go func() {
		if err := sub.Consume(ctx, disp.Handle); err != nil {
			logger.Error(ctx, "consumer: stopped",
				map[string]interface{}{logger.ErrorKey: err.Error()})
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────
	<-ctx.Done()
	logger.Info(context.Background(), "shutdown signal received", nil)
	_ = logger.Sync()
}

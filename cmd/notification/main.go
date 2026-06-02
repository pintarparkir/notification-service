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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/farid/notification-service/internal/notification/consumer"
	"github.com/farid/notification-service/internal/notification/model"
	"github.com/farid/notification-service/internal/notification/usecase"

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
	defer func() {
		if err := otel.EndAPM(); err != nil {
			fmt.Fprintln(os.Stderr, "otel shutdown:", err)
		}
	}()

	// ── HTTP health (Cloud Run requires a listening port) ────────────────
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		srv := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		if err := srv.ListenAndServe(); err != nil {
			logger.Error(ctx, "health http failed", map[string]interface{}{logger.ErrorKey: err.Error()})
		}
	}()

	// ── user-service gRPC client ─────────────────────────────────────────────
	// Dial is lazy (no WithBlock) — actual TCP connection happens on first call,
	// so notification-service can start regardless of user-service readiness.
	conn, err := grpcclient.Dial(cfg.UserGrpcAddr)
	if err != nil {
		logger.Fatal(ctx, "user-service grpc dial failed",
			map[string]interface{}{"addr": cfg.UserGrpcAddr, logger.ErrorKey: err.Error()})
	}
	defer func() { _ = conn.Close() }()
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

	uc := usecase.New(users, smsClient)
	disp := consumer.New(uc)

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
	if err := logger.Sync(); err != nil {
		fmt.Fprintln(os.Stderr, "logger sync:", err)
	}
}

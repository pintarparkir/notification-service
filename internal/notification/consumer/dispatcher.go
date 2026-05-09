// Package consumer wires RabbitMQ deliveries into the SMS pipeline.
//
// Pipeline per delivery:
//   1. Parse the JSON payload into model.Event.
//   2. Resolve MSISDN via grpcclient.UserClient (with 5-min cache).
//   3. Render the SMS body via template.Render() based on routing key.
//   4. Send via sms.Client (stub or real Telkomsel).
//
// Failure semantics (from feature 01):
//   - parse error      → ACK + DLQ (bad payload, retry won't help)
//   - unknown key      → ACK + drop (no work to do)
//   - empty MSISDN     → ACK + DLQ (driver row exists but has no phone)
//   - gRPC timeout     → NACK with requeue (transient)
//   - gRPC NOT_FOUND   → ACK + DLQ (driver doesn't exist; replay won't fix)
//   - SMS gateway 4xx  → ACK + DLQ (bad number, etc.)
//   - SMS gateway 5xx  → NACK with requeue (transient)
//
// For MVP we collapse the DLQ-vs-drop distinction: the rabbit Subscriber NACKs
// with requeue=false on returned error of type ErrPermanent so RabbitMQ routes
// to the bound DLQ; nil ack-s success; any other error requeues.
package consumer

import (
	"context"
	"encoding/json"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/farid/notification-service/internal/notification/model"
	"github.com/farid/notification-service/internal/notification/template"
	"github.com/farid/notification-service/pkg/grpcclient"
	"github.com/farid/notification-service/pkg/logger"
	"github.com/farid/notification-service/pkg/rabbit"
	"github.com/farid/notification-service/pkg/sms"
)

// ErrPermanent re-exports rabbit.ErrPermanent so dispatcher callers don't
// have to import pkg/rabbit just to construct a DLQ-bound error.
var ErrPermanent = rabbit.ErrPermanent

type Dispatcher struct {
	users grpcclient.UserClient
	sms   sms.Client
}

func New(users grpcclient.UserClient, smsClient sms.Client) *Dispatcher {
	return &Dispatcher{users: users, sms: smsClient}
}

// Handle processes one delivery. Return values:
//
//	nil          → ACK
//	ErrPermanent → NACK requeue=false (DLQ)
//	other err    → NACK requeue=true  (retry)
func (d *Dispatcher) Handle(ctx context.Context, routingKey string, body []byte) error {
	var ev model.Event
	if err := json.Unmarshal(body, &ev); err != nil {
		logger.Error(ctx, "consumer: bad payload",
			map[string]interface{}{"routing_key": routingKey, logger.ErrorKey: err.Error()})
		return ErrPermanent
	}

	body64 := template.Render(routingKey, ev)
	if body64 == "" {
		// Unknown routing key OR template returned empty (data too sparse to
		// say anything useful). Drop without DLQ — there's no remediation.
		logger.Info(ctx, "consumer: dropping unrecognised key",
			map[string]interface{}{"routing_key": routingKey})
		return nil
	}

	if strings.TrimSpace(ev.DriverID) == "" {
		// Producer didn't include driver_id and we don't yet have the chain
		// resolution path (reservation_id → reservation-service.GetReservation
		// → driver_id). Drop to DLQ for manual triage.
		logger.Error(ctx, "consumer: missing driver_id",
			map[string]interface{}{"routing_key": routingKey})
		return ErrPermanent
	}

	msisdn, err := d.users.GetMSISDN(ctx, ev.DriverID)
	if err != nil {
		// gRPC NOT_FOUND → permanent. Anything else → transient (timeout, conn).
		if status.Code(err) == codes.NotFound {
			logger.Error(ctx, "consumer: driver not found",
				map[string]interface{}{"driver_id": ev.DriverID, "routing_key": routingKey})
			return ErrPermanent
		}
		return err
	}
	if strings.TrimSpace(msisdn) == "" {
		logger.Error(ctx, "consumer: driver has no phone",
			map[string]interface{}{"driver_id": ev.DriverID, "routing_key": routingKey})
		return ErrPermanent
	}

	if err := d.sms.Send(ctx, msisdn, body64); err != nil {
		// Real Telkomsel client classifies 4xx vs 5xx. Stub never errors.
		// Default to retryable for anything we can't classify — better to
		// retry once than to dead-letter prematurely.
		return err
	}

	logger.Info(ctx, "sms sent", map[string]interface{}{
		"routing_key": routingKey,
		"to":          msisdn,
		"driver_id":   ev.DriverID,
	})
	return nil
}

// Package consumer wires RabbitMQ deliveries into the SMS pipeline.
//
// Pipeline per delivery:
//  1. Parse the JSON payload into model.Event.
//  2. Route to the appropriate NotificationUsecase method.
//
// Failure semantics (from feature 01):
//   - parse error    → ErrPermanent (DLQ)
//   - unknown key    → nil (drop)
//   - usecase errors → propagated as-is (ErrPermanent or transient)
package consumer

import (
	"context"
	"encoding/json"

	"github.com/farid/notification-service/internal/notification/model"
	"github.com/farid/notification-service/internal/notification/usecase"
	"github.com/farid/notification-service/pkg/logger"
	"github.com/farid/notification-service/pkg/rabbit"
)

// ErrPermanent re-exports rabbit.ErrPermanent so dispatcher callers don't
// have to import pkg/rabbit just to inspect the DLQ-bound sentinel.
var ErrPermanent = rabbit.ErrPermanent

type Dispatcher struct {
	uc usecase.NotificationUsecase
}

func New(uc usecase.NotificationUsecase) *Dispatcher {
	return &Dispatcher{uc: uc}
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

	switch routingKey {
	case model.EvtReservationConfirmed:
		return d.uc.HandleReservationConfirmed(ctx, ev)
	case model.EvtReservationCancelled:
		return d.uc.HandleReservationCancelled(ctx, ev)
	case model.EvtReservationExpired:
		return d.uc.HandleReservationExpired(ctx, ev)
	case model.EvtInvoiceClosed:
		return d.uc.HandleInvoiceClosed(ctx, ev)
	case model.EvtPaymentPaid:
		return d.uc.HandlePaymentPaid(ctx, ev)
	default:
		logger.Info(ctx, "consumer: dropping unrecognised key",
			map[string]interface{}{"routing_key": routingKey})
		return nil
	}
}

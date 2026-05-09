package usecase

import (
	"context"

	"github.com/farid/notification-service/internal/notification/model"
)

type NotificationUsecase interface {
	HandleReservationConfirmed(ctx context.Context, ev model.Event) error
	HandleReservationCancelled(ctx context.Context, ev model.Event) error
	HandleReservationExpired(ctx context.Context, ev model.Event) error
	HandleInvoiceClosed(ctx context.Context, ev model.Event) error
	HandlePaymentPaid(ctx context.Context, ev model.Event) error
}

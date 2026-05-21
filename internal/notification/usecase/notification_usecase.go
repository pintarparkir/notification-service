// Package usecase implements notification business logic.
package usecase

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/farid/notification-service/internal/notification/model"
	"github.com/farid/notification-service/internal/notification/template"
	"github.com/farid/notification-service/pkg/grpcclient"
	"github.com/farid/notification-service/pkg/rabbit"
	"github.com/farid/notification-service/pkg/sms"
)

type NotificationUsecaseImpl struct {
	users grpcclient.UserClient
	sms   sms.Client
}

func New(users grpcclient.UserClient, smsClient sms.Client) *NotificationUsecaseImpl {
	return &NotificationUsecaseImpl{users: users, sms: smsClient}
}

// notify is the shared pipeline: validate driver_id → lookup MSISDN → render → send.
func (u *NotificationUsecaseImpl) notify(ctx context.Context, ev model.Event, routingKey string) error {
	if strings.TrimSpace(ev.DriverID) == "" {
		return rabbit.ErrPermanent
	}

	msisdn := strings.TrimSpace(ev.MSISDN)
	if msisdn == "" {
		var err error
		msisdn, err = u.users.GetMSISDN(ctx, ev.DriverID)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return rabbit.ErrPermanent
			}
			return err
		}
	}
	if strings.TrimSpace(msisdn) == "" {
		return rabbit.ErrPermanent
	}

	body := template.Render(routingKey, ev)
	if body == "" {
		return rabbit.ErrPermanent
	}
	return u.sms.Send(ctx, msisdn, body)
}

func (u *NotificationUsecaseImpl) HandleReservationConfirmed(ctx context.Context, ev model.Event) error {
	return u.notify(ctx, ev, model.EvtReservationConfirmed)
}

func (u *NotificationUsecaseImpl) HandleReservationCancelled(ctx context.Context, ev model.Event) error {
	return u.notify(ctx, ev, model.EvtReservationCancelled)
}

func (u *NotificationUsecaseImpl) HandleReservationExpired(ctx context.Context, ev model.Event) error {
	return u.notify(ctx, ev, model.EvtReservationExpired)
}

func (u *NotificationUsecaseImpl) HandleInvoiceClosed(ctx context.Context, ev model.Event) error {
	return u.notify(ctx, ev, model.EvtInvoiceClosed)
}

func (u *NotificationUsecaseImpl) HandlePaymentPaid(ctx context.Context, ev model.Event) error {
	return u.notify(ctx, ev, model.EvtPaymentPaid)
}

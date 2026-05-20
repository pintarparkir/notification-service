package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/farid/notification-service/internal/notification/consumer"
	"github.com/farid/notification-service/internal/notification/model"
	mockusecase "github.com/farid/notification-service/internal/notification/usecase/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validBody marshals a minimal Event with the given driverID.
func validBody(t *testing.T, driverID string) []byte {
	t.Helper()
	b, err := json.Marshal(model.Event{DriverID: driverID})
	require.NoError(t, err)
	return b
}

func TestHandle_BadJSON(t *testing.T) {
	ctx := context.Background()
	uc := new(mockusecase.MockNotificationUsecase)
	d := consumer.New(uc)

	err := d.Handle(ctx, model.EvtReservationConfirmed, []byte("notjson{{"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, consumer.ErrPermanent))
	uc.AssertNotCalled(t, "HandleReservationConfirmed")
}

func TestHandle_PropagatesUsecaseErrPermanent(t *testing.T) {
	ctx := context.Background()
	uc := new(mockusecase.MockNotificationUsecase)
	ev := model.Event{DriverID: "drv-1"}
	uc.On("HandleReservationConfirmed", ctx, ev).Return(consumer.ErrPermanent)
	d := consumer.New(uc)

	err := d.Handle(ctx, model.EvtReservationConfirmed, validBody(t, "drv-1"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, consumer.ErrPermanent))
	uc.AssertExpectations(t)
}

func TestHandle_PropagatesUsecaseTransientError(t *testing.T) {
	ctx := context.Background()
	uc := new(mockusecase.MockNotificationUsecase)
	ev := model.Event{DriverID: "drv-1"}
	transientErr := errors.New("grpc timeout")
	uc.On("HandleReservationConfirmed", ctx, ev).Return(transientErr)
	d := consumer.New(uc)

	err := d.Handle(ctx, model.EvtReservationConfirmed, validBody(t, "drv-1"))

	require.Error(t, err)
	assert.False(t, errors.Is(err, consumer.ErrPermanent), "transient error must not be treated as permanent")
	assert.ErrorIs(t, err, transientErr)
	uc.AssertExpectations(t)
}

func TestHandle_UnknownKey(t *testing.T) {
	ctx := context.Background()
	uc := new(mockusecase.MockNotificationUsecase)
	d := consumer.New(uc)

	err := d.Handle(ctx, "unknown.event.v9", validBody(t, "drv-1"))

	require.NoError(t, err)
	uc.AssertNotCalled(t, "HandleReservationConfirmed")
	uc.AssertNotCalled(t, "HandleReservationCancelled")
	uc.AssertNotCalled(t, "HandleReservationExpired")
	uc.AssertNotCalled(t, "HandleInvoiceClosed")
	uc.AssertNotCalled(t, "HandlePaymentPaid")
}

func TestHandle_RouteReservationConfirmed(t *testing.T) {
	ctx := context.Background()
	uc := new(mockusecase.MockNotificationUsecase)
	ev := model.Event{DriverID: "drv-1"}
	uc.On("HandleReservationConfirmed", ctx, ev).Return(nil)
	d := consumer.New(uc)

	err := d.Handle(ctx, model.EvtReservationConfirmed, validBody(t, "drv-1"))

	require.NoError(t, err)
	uc.AssertExpectations(t)
}

func TestHandle_RouteReservationCancelled(t *testing.T) {
	ctx := context.Background()
	uc := new(mockusecase.MockNotificationUsecase)
	ev := model.Event{DriverID: "drv-1"}
	uc.On("HandleReservationCancelled", ctx, ev).Return(nil)
	d := consumer.New(uc)

	err := d.Handle(ctx, model.EvtReservationCancelled, validBody(t, "drv-1"))

	require.NoError(t, err)
	uc.AssertExpectations(t)
}

func TestHandle_RouteReservationExpired(t *testing.T) {
	ctx := context.Background()
	uc := new(mockusecase.MockNotificationUsecase)
	ev := model.Event{DriverID: "drv-1"}
	uc.On("HandleReservationExpired", ctx, ev).Return(nil)
	d := consumer.New(uc)

	err := d.Handle(ctx, model.EvtReservationExpired, validBody(t, "drv-1"))

	require.NoError(t, err)
	uc.AssertExpectations(t)
}

func TestHandle_RouteInvoiceClosed(t *testing.T) {
	ctx := context.Background()
	uc := new(mockusecase.MockNotificationUsecase)
	ev := model.Event{DriverID: "drv-1"}
	uc.On("HandleInvoiceClosed", ctx, ev).Return(nil)
	d := consumer.New(uc)

	err := d.Handle(ctx, model.EvtInvoiceClosed, validBody(t, "drv-1"))

	require.NoError(t, err)
	uc.AssertExpectations(t)
}

func TestHandle_RoutePaymentPaid(t *testing.T) {
	ctx := context.Background()
	uc := new(mockusecase.MockNotificationUsecase)
	ev := model.Event{DriverID: "drv-1"}
	uc.On("HandlePaymentPaid", ctx, ev).Return(nil)
	d := consumer.New(uc)

	err := d.Handle(ctx, model.EvtPaymentPaid, validBody(t, "drv-1"))

	require.NoError(t, err)
	uc.AssertExpectations(t)
}

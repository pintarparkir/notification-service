package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/farid/notification-service/internal/notification/model"
	"github.com/farid/notification-service/internal/notification/usecase"
	mockgrpc "github.com/farid/notification-service/pkg/grpcclient/mock"
	"github.com/farid/notification-service/pkg/rabbit"
	mocksms "github.com/farid/notification-service/pkg/sms/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newUC(users *mockgrpc.MockUserClient, smsClient *mocksms.MockSMSClient) usecase.NotificationUsecase {
	return usecase.New(users, smsClient)
}

// ── HandleReservationConfirmed — full failure path coverage ────────────────

func TestHandleReservationConfirmed_EmptyDriverID(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)

	uc := newUC(users, smsClient)
	err := uc.HandleReservationConfirmed(ctx, model.Event{DriverID: ""})

	require.Error(t, err)
	assert.True(t, errors.Is(err, rabbit.ErrPermanent))
	users.AssertNotCalled(t, "GetMSISDN")
}

func TestHandleReservationConfirmed_GRPCNotFound(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)
	users.On("GetMSISDN", ctx, "drv-1").
		Return("", status.Error(codes.NotFound, "driver not found"))

	uc := newUC(users, smsClient)
	err := uc.HandleReservationConfirmed(ctx, model.Event{DriverID: "drv-1", SpotID: "A1"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, rabbit.ErrPermanent))
	smsClient.AssertNotCalled(t, "Send")
	users.AssertExpectations(t)
}

func TestHandleReservationConfirmed_GRPCTransient(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)
	transientErr := errors.New("connection timeout")
	users.On("GetMSISDN", ctx, "drv-1").Return("", transientErr)

	uc := newUC(users, smsClient)
	err := uc.HandleReservationConfirmed(ctx, model.Event{DriverID: "drv-1"})

	require.Error(t, err)
	assert.False(t, errors.Is(err, rabbit.ErrPermanent), "transient error must not be wrapped as ErrPermanent")
	smsClient.AssertNotCalled(t, "Send")
	users.AssertExpectations(t)
}

func TestHandleReservationConfirmed_EmptyMSISDN(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)
	users.On("GetMSISDN", ctx, "drv-1").Return("", nil)

	uc := newUC(users, smsClient)
	err := uc.HandleReservationConfirmed(ctx, model.Event{DriverID: "drv-1"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, rabbit.ErrPermanent))
	smsClient.AssertNotCalled(t, "Send")
	users.AssertExpectations(t)
}

func TestHandleReservationConfirmed_SMSError(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)
	smsErr := errors.New("sms gateway 5xx")
	users.On("GetMSISDN", ctx, "drv-1").Return("08121111111", nil)
	smsClient.On("Send", ctx, "08121111111", mock.AnythingOfType("string")).Return(smsErr)

	uc := newUC(users, smsClient)
	err := uc.HandleReservationConfirmed(ctx, model.Event{DriverID: "drv-1", SpotID: "A1"})

	require.Error(t, err)
	assert.ErrorIs(t, err, smsErr)
	users.AssertExpectations(t)
	smsClient.AssertExpectations(t)
}

func TestHandleReservationConfirmed_HappyPath(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)
	users.On("GetMSISDN", ctx, "drv-1").Return("08121111111", nil)
	smsClient.On("Send", ctx, "08121111111",
		mock.MatchedBy(func(body string) bool {
			return strings.Contains(body, "A1") // SpotID rendered in SMS body
		}),
	).Return(nil)

	uc := newUC(users, smsClient)
	err := uc.HandleReservationConfirmed(ctx, model.Event{DriverID: "drv-1", SpotID: "A1"})

	require.NoError(t, err)
	users.AssertExpectations(t)
	smsClient.AssertExpectations(t)
}

// ── Happy-path smoke tests for remaining 4 methods ─────────────────────────

func TestHandleReservationCancelled_HappyPath(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)
	users.On("GetMSISDN", ctx, "drv-2").Return("08122222222", nil)
	smsClient.On("Send", ctx, "08122222222", mock.AnythingOfType("string")).Return(nil)

	uc := newUC(users, smsClient)
	err := uc.HandleReservationCancelled(ctx, model.Event{DriverID: "drv-2"})

	require.NoError(t, err)
	users.AssertExpectations(t)
	smsClient.AssertExpectations(t)
}

func TestHandleReservationExpired_HappyPath(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)
	users.On("GetMSISDN", ctx, "drv-3").Return("08123333333", nil)
	smsClient.On("Send", ctx, "08123333333", mock.AnythingOfType("string")).Return(nil)

	uc := newUC(users, smsClient)
	err := uc.HandleReservationExpired(ctx, model.Event{DriverID: "drv-3"})

	require.NoError(t, err)
	users.AssertExpectations(t)
	smsClient.AssertExpectations(t)
}

func TestHandleInvoiceClosed_HappyPath(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)
	users.On("GetMSISDN", ctx, "drv-4").Return("08124444444", nil)
	// formatIDR(15000) = "15.000"
	smsClient.On("Send", ctx, "08124444444",
		mock.MatchedBy(func(body string) bool {
			return strings.Contains(body, "15.000")
		}),
	).Return(nil)

	uc := newUC(users, smsClient)
	err := uc.HandleInvoiceClosed(ctx, model.Event{DriverID: "drv-4", TotalIDR: 15000})

	require.NoError(t, err)
	users.AssertExpectations(t)
	smsClient.AssertExpectations(t)
}

func TestHandlePaymentPaid_HappyPath(t *testing.T) {
	ctx := context.Background()
	users := new(mockgrpc.MockUserClient)
	smsClient := new(mocksms.MockSMSClient)
	users.On("GetMSISDN", ctx, "drv-5").Return("08125555555", nil)
	smsClient.On("Send", ctx, "08125555555", mock.AnythingOfType("string")).Return(nil)

	uc := newUC(users, smsClient)
	err := uc.HandlePaymentPaid(ctx, model.Event{DriverID: "drv-5"})

	require.NoError(t, err)
	users.AssertExpectations(t)
	smsClient.AssertExpectations(t)
}

package mockusecase

import (
	"context"

	"github.com/farid/notification-service/internal/notification/model"
	"github.com/farid/notification-service/internal/notification/usecase"
	"github.com/stretchr/testify/mock"
)

var _ usecase.NotificationUsecase = (*MockNotificationUsecase)(nil)

type MockNotificationUsecase struct {
	mock.Mock
}

func (m *MockNotificationUsecase) HandleReservationConfirmed(ctx context.Context, ev model.Event) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *MockNotificationUsecase) HandleReservationCancelled(ctx context.Context, ev model.Event) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *MockNotificationUsecase) HandleReservationExpired(ctx context.Context, ev model.Event) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *MockNotificationUsecase) HandleInvoiceClosed(ctx context.Context, ev model.Event) error {
	return m.Called(ctx, ev).Error(0)
}

func (m *MockNotificationUsecase) HandlePaymentPaid(ctx context.Context, ev model.Event) error {
	return m.Called(ctx, ev).Error(0)
}

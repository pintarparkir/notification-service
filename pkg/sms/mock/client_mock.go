package mocksms

import (
	"context"

	"github.com/farid/notification-service/pkg/sms"
	"github.com/stretchr/testify/mock"
)

var _ sms.Client = (*MockSMSClient)(nil)

type MockSMSClient struct {
	mock.Mock
}

func (m *MockSMSClient) Send(ctx context.Context, to, message string) error {
	args := m.Called(ctx, to, message)
	return args.Error(0)
}

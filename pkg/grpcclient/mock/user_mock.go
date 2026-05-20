package mockgrpc

import (
	"context"

	"github.com/farid/notification-service/pkg/grpcclient"
	"github.com/stretchr/testify/mock"
)

var _ grpcclient.UserClient = (*MockUserClient)(nil)

type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) GetMSISDN(ctx context.Context, driverID string) (string, error) {
	args := m.Called(ctx, driverID)
	return args.String(0), args.Error(1)
}

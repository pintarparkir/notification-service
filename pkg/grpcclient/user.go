package grpcclient

import (
	"context"
	"errors"
	"sync"
	"time"

	"google.golang.org/grpc"

	userv1 "github.com/farid/notification-service/api/proto/user/v1"
)

// UserClient is the slice of user-service we depend on. Just MSISDN lookup
// for now — extend if SMS templates start using more profile fields.
type UserClient interface {
	GetMSISDN(ctx context.Context, driverID string) (string, error)
}

// userClient wraps the generated UserServiceClient with a tiny per-process
// cache so a burst of events for the same driver doesn't hammer user-service.
//
// Cache shape: driverID → (msisdn, expiresAt). On hit + non-expired → return.
// On miss / expired → call gRPC, cache, return.
//
// 5-minute TTL is intentional: drivers rarely change phones mid-session.
// For a hard correctness story (driver phone migration) we'd subscribe to a
// hypothetical user.phone_changed.v1 event and invalidate; out of MVP scope.
type userClient struct {
	pb      userv1.UserServiceClient
	timeout time.Duration

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	msisdn    string
	expiresAt time.Time
}

// NewUserClient wraps a *grpc.ClientConn dialed via Dial().
func NewUserClient(conn *grpc.ClientConn, timeout time.Duration) UserClient {
	return &userClient{
		pb:      userv1.NewUserServiceClient(conn),
		timeout: timeout,
		cache:   make(map[string]cacheEntry, 64),
	}
}

func (c *userClient) GetMSISDN(ctx context.Context, driverID string) (string, error) {
	if driverID == "" {
		return "", errors.New("driver_id required")
	}

	// Cache lookup
	c.mu.RLock()
	if e, ok := c.cache[driverID]; ok && time.Now().Before(e.expiresAt) {
		c.mu.RUnlock()
		return e.msisdn, nil
	}
	c.mu.RUnlock()

	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.pb.GetUserById(cctx, &userv1.GetUserByIdRequest{Id: driverID})
	if err != nil {
		return "", err
	}
	msisdn := resp.GetPhoneE164()

	c.mu.Lock()
	c.cache[driverID] = cacheEntry{msisdn: msisdn, expiresAt: time.Now().Add(5 * time.Minute)}
	c.mu.Unlock()

	return msisdn, nil
}

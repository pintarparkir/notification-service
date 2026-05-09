// Package sms abstracts the SMS gateway. The stub implementation logs instead of
// hitting a real API; a Telkomsel HTTP client implementation lands when the
// gateway credentials are available.
package sms

import "context"

type Client interface {
	Send(ctx context.Context, to, message string) error
}

package sms

import (
	"context"
	"log"
)

type stubClient struct{ senderID string }

func NewStub(senderID string) Client { return &stubClient{senderID: senderID} }

func (c *stubClient) Send(ctx context.Context, to, message string) error {
	log.Printf("[SMS-STUB sender=%s to=%s] %s", c.senderID, to, message)
	return nil
}

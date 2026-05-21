package rabbit

import (
	"context"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ErrPermanent is the sentinel handlers return when a delivery should not be
// retried. Subscriber translates it to NACK requeue=false; combined with a
// queue-level dead-letter-exchange binding, the message lands in the DLQ.
var ErrPermanent = errors.New("permanent (DLQ)")

// Subscriber consumes from a durable queue bound to the topic exchange.
// One Subscriber owns one queue; the caller maps routing keys → handlers.
type Subscriber struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	queue string
}

// SubscriberOptions configures NewSubscriber.
//   - DLQ (optional): when set, the main queue is declared with
//     x-dead-letter-exchange = "<exchange>.dlx", and a fan-out DLX +
//     dead-letter queue named DLQ are declared/bound automatically.
//     Messages NACKed with requeue=false land there.
type SubscriberOptions struct {
	DLQ string // dead-letter queue name; "" disables DLX wiring
}

// NewSubscriber dials AMQP, declares topology (exchange, queue, optional DLX/DLQ,
// bindings), returns a subscriber ready to Consume.
func NewSubscriber(amqpURL, exchange, queue string, routingKeys []string, opt SubscriberOptions) (*Subscriber, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("amqp dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("amqp channel: %w", err)
	}

	// Main topic exchange.
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("exchange declare: %w", err)
	}

	// DLX/DLQ wiring (optional). DLX is a fan-out so any failure ends up in DLQ.
	var queueArgs amqp.Table
	if opt.DLQ != "" {
		dlxName := exchange + ".dlx"
		if err := ch.ExchangeDeclare(dlxName, "fanout", true, false, false, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("dlx declare: %w", err)
		}
		if _, err := ch.QueueDeclare(opt.DLQ, true, false, false, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("dlq declare: %w", err)
		}
		if err := ch.QueueBind(opt.DLQ, "", dlxName, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("dlq bind: %w", err)
		}
		queueArgs = amqp.Table{"x-dead-letter-exchange": dlxName}
	}

	if _, err := ch.QueueDeclare(queue, true, false, false, false, queueArgs); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("queue declare: %w", err)
	}
	for _, key := range routingKeys {
		if err := ch.QueueBind(queue, key, exchange, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("queue bind %s: %w", key, err)
		}
	}
	if err := ch.Qos(10, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("qos: %w", err)
	}
	return &Subscriber{conn: conn, ch: ch, queue: queue}, nil
}

// Handler processes one delivery.
//   - nil err            → ACK
//   - errors.Is ErrPermanent → NACK requeue=false (→ DLQ if wired)
//   - other err          → NACK requeue=true (transient retry)
type Handler func(ctx context.Context, routingKey string, body []byte) error

// Consume blocks until ctx is cancelled.
func (s *Subscriber) Consume(ctx context.Context, handler Handler) error {
	deliveries, err := s.ch.Consume(s.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			err := handler(ctx, d.RoutingKey, d.Body)
			switch {
			case err == nil:
				_ = d.Ack(false)
			case errors.Is(err, ErrPermanent):
				_ = d.Nack(false, false) // requeue=false → DLX → DLQ
			default:
				_ = d.Nack(false, true) // requeue=true → transient retry
			}
		}
	}
}

func (s *Subscriber) Close() {
	if s.ch != nil {
		_ = s.ch.Close()
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

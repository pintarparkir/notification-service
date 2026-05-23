package rabbit

import (
	"context"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var ErrPermanent = errors.New("permanent (DLQ)")

type Subscriber struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	queue string
}

type SubscriberOptions struct {
	DLQ string
}

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

	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("exchange declare: %w", err)
	}

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

type Handler func(ctx context.Context, routingKey string, body []byte) error

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
			msgCtx := otel.GetTextMapPropagator().Extract(ctx, &amqpHeaderCarrier{d.Headers})
			msgCtx, span := otel.Tracer("rabbit").Start(msgCtx, "consume "+d.RoutingKey,
				trace.WithSpanKind(trace.SpanKindConsumer),
			)

			err := handler(msgCtx, d.RoutingKey, d.Body)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			span.End()

			switch {
			case err == nil:
				_ = d.Ack(false)
			case errors.Is(err, ErrPermanent):
				_ = d.Nack(false, false)
			default:
				_ = d.Nack(false, true)
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

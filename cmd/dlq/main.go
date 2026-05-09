// notification-service DLQ tool.
//
// CLI for manually inspecting + replaying messages stuck in the DLQ.
// Usage:
//
//	dlq list [--limit N]            show messages in the DLQ (default N=20)
//	dlq replay [--limit N]          republish up to N messages back to the main exchange
//	dlq purge   --yes-i-know        delete all messages from the DLQ
//
// Reads RABBIT_URL, RABBIT_EXCHANGE, RABBIT_DLQ from the same .env as the
// service. Run from the service root: `go run ./cmd/dlq <subcommand>`.
package main

import (
	"flag"
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/farid/notification-service/pkg/configs"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	cfg := configs.NewConfig(configs.ConfigLoader{Env: os.Getenv("PROJECT_ENV")})

	conn, err := amqp.Dial(cfg.RabbitURL)
	must(err, "amqp dial")
	defer conn.Close()
	ch, err := conn.Channel()
	must(err, "channel")
	defer ch.Close()

	switch cmd {
	case "list":
		fs := flag.NewFlagSet("list", flag.ExitOnError)
		limit := fs.Int("limit", 20, "max messages to print")
		_ = fs.Parse(args)
		listDLQ(ch, cfg.RabbitDLQ, *limit)

	case "replay":
		fs := flag.NewFlagSet("replay", flag.ExitOnError)
		limit := fs.Int("limit", 50, "max messages to replay")
		_ = fs.Parse(args)
		replayDLQ(ch, cfg.RabbitExchange, cfg.RabbitDLQ, *limit)

	case "purge":
		fs := flag.NewFlagSet("purge", flag.ExitOnError)
		yes := fs.Bool("yes-i-know", false, "confirm deletion")
		_ = fs.Parse(args)
		if !*yes {
			fmt.Fprintln(os.Stderr, "refusing without --yes-i-know")
			os.Exit(2)
		}
		n, err := ch.QueuePurge(cfg.RabbitDLQ, false)
		must(err, "purge")
		fmt.Printf("purged %d messages\n", n)

	default:
		usage()
		os.Exit(2)
	}
}

// listDLQ pulls up to `limit` messages with autoAck=false then NACKs them
// back so the queue is unchanged. Read-only inspection.
func listDLQ(ch *amqp.Channel, dlq string, limit int) {
	for i := 0; i < limit; i++ {
		msg, ok, err := ch.Get(dlq, false)
		must(err, "get")
		if !ok {
			break
		}
		fmt.Printf("[%d] routing_key=%-40s body=%s\n", i+1, deathRoutingKey(msg), msg.Body)
		// requeue so we don't drain on inspection
		_ = msg.Nack(false, true)
	}
}

// replayDLQ pulls up to `limit` messages and republishes them to the main
// exchange with their original routing key (recovered from x-death header).
// On success we ACK the DLQ message; on publish failure we NACK requeue so
// nothing is lost.
func replayDLQ(ch *amqp.Channel, exchange, dlq string, limit int) {
	replayed := 0
	for i := 0; i < limit; i++ {
		msg, ok, err := ch.Get(dlq, false)
		must(err, "get")
		if !ok {
			break
		}
		key := deathRoutingKey(msg)
		if key == "" {
			fmt.Fprintf(os.Stderr, "skip: message has no x-death.routing-keys; original routing key unknown\n")
			_ = msg.Nack(false, true)
			continue
		}
		err = ch.Publish(exchange, key, false, false, amqp.Publishing{
			ContentType:  msg.ContentType,
			DeliveryMode: amqp.Persistent,
			Body:         msg.Body,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "publish failed: %v\n", err)
			_ = msg.Nack(false, true)
			continue
		}
		_ = msg.Ack(false)
		replayed++
		fmt.Printf("→ replayed key=%s body=%s\n", key, msg.Body)
	}
	fmt.Printf("\nreplayed %d messages\n", replayed)
}

// deathRoutingKey reads the original routing key from RabbitMQ's standard
// x-death header (added by the broker when dead-lettering).
func deathRoutingKey(msg amqp.Delivery) string {
	if msg.Headers == nil {
		return ""
	}
	xd, ok := msg.Headers["x-death"].([]interface{})
	if !ok || len(xd) == 0 {
		return ""
	}
	first, ok := xd[0].(amqp.Table)
	if !ok {
		return ""
	}
	keys, ok := first["routing-keys"].([]interface{})
	if !ok || len(keys) == 0 {
		return ""
	}
	if k, ok := keys[0].(string); ok {
		return k
	}
	return ""
}

func must(err error, where string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", where, err)
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  dlq list   [--limit N]   inspect messages currently in the DLQ
  dlq replay [--limit N]   republish DLQ messages back to the main exchange
  dlq purge  --yes-i-know  delete all messages from the DLQ`)
}

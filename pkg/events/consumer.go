package events

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
)

// Handler processes one received event. Returning nil ACKs the message (done);
// returning an error NAKs it (redeliver). Because delivery is at-least-once, a
// handler MUST be idempotent — the same event can arrive more than once.
type Handler func(ctx context.Context, env Envelope) error

// Consume starts a DURABLE JetStream consumer on the stream and dispatches each
// message's envelope to handler. "Durable" means the consumer's acked position is
// remembered by the server across restarts — a restarted consumer resumes where it
// left off instead of re-reading everything. MaxDeliver caps redelivery so a
// permanently-failing ("poison") message eventually stops being retried.
func Consume(
	ctx context.Context,
	js jetstream.JetStream,
	stream, durable string,
	log *slog.Logger,
	handler Handler,
) (jetstream.ConsumeContext, error) {
	cons, err := js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:    durable,
		AckPolicy:  jetstream.AckExplicitPolicy,
		MaxDeliver: 5,
	})
	if err != nil {
		return nil, fmt.Errorf("events: create consumer %q: %w", durable, err)
	}

	return cons.Consume(func(msg jetstream.Msg) {
		env, err := Parse(msg.Data())
		if err != nil {
			// Unparseable payload: retrying can't fix it, so drop it (ack) rather
			// than redeliver forever.
			log.ErrorContext(ctx, "dropping unparseable event", "err", err)
			_ = msg.Ack()
			return
		}
		if err := handler(ctx, env); err != nil {
			log.ErrorContext(ctx, "event handler failed; will redeliver", "err", err, "event_id", env.EventID, "topic", env.Topic)
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
}

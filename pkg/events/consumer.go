package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Nak backoff bounds: redelivery is spaced out with capped exponential backoff so a
// transient downstream outage doesn't burn a handler's whole retry budget in a tight
// loop, and doesn't hammer a struggling dependency.
const (
	nakBackoffBase = 5 * time.Second
	nakBackoffMax  = 5 * time.Minute
)

// nakBackoff returns an exponentially increasing delay based on how many times this
// message has already been delivered, capped at nakBackoffMax.
func nakBackoff(msg jetstream.Msg) time.Duration {
	n := 1
	if md, err := msg.Metadata(); err == nil && md.NumDelivered > 0 {
		n = int(md.NumDelivered)
	}
	d := nakBackoffBase
	for i := 1; i < n && d < nakBackoffMax; i++ {
		d *= 2
	}
	if d > nakBackoffMax {
		d = nakBackoffMax
	}
	return d
}

// Handler processes one received event. Returning nil ACKs the message (done);
// returning an error NAKs it (redeliver). Because delivery is at-least-once, a
// handler MUST be idempotent — the same event can arrive more than once.
type Handler func(ctx context.Context, env Envelope) error

// consumeOptions holds tunables for Consume.
type consumeOptions struct {
	maxDeliver int
}

// ConsumeOption customizes a durable consumer.
type ConsumeOption func(*consumeOptions)

// WithMaxDeliver overrides how many times JetStream redelivers a message before
// giving up (default 5). Pass -1 for unlimited — appropriate when the HANDLER owns
// the terminal decision (e.g. an app-level dead-letter after N attempts), so
// JetStream must not drop a message the handler still considers retryable.
func WithMaxDeliver(n int) ConsumeOption {
	return func(o *consumeOptions) { o.maxDeliver = n }
}

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
	opts ...ConsumeOption,
) (jetstream.ConsumeContext, error) {
	cfg := consumeOptions{maxDeliver: 5}
	for _, o := range opts {
		o(&cfg)
	}
	cons, err := js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:    durable,
		AckPolicy:  jetstream.AckExplicitPolicy,
		MaxDeliver: cfg.maxDeliver,
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
			delay := nakBackoff(msg)
			log.ErrorContext(ctx, "event handler failed; will redeliver", "err", err, "event_id", env.EventID, "topic", env.Topic, "delay", delay)
			_ = msg.NakWithDelay(delay)
			return
		}
		_ = msg.Ack()
	})
}

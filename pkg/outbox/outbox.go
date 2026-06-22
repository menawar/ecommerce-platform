// Package outbox implements the transactional-outbox publisher: a background
// poller that reads rows a service wrote to its outbox table (in the same DB
// transaction as a state change) and publishes them to a broker, marking each
// published only AFTER a successful publish.
//
// Why this exists: you cannot atomically "commit to Postgres AND publish to NATS"
// — there's no shared transaction across two systems. If you publish then crash
// before commit, you've emitted an event for a state that never happened; if you
// commit then crash before publish, you've lost the event. The outbox sidesteps
// this: the event is written INSIDE the DB transaction (so it's atomic with the
// state change), and this poller delivers it afterwards. The cost is at-least-once
// delivery (a crash between publish and mark re-delivers), so consumers MUST be
// idempotent.
package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Message is one outbox row to deliver.
type Message struct {
	ID      string
	Topic   string
	Payload []byte
}

// Store is the persistence the poller drives. A service implements it over its
// own outbox table (so this package depends on no concrete DB).
type Store interface {
	FetchUnpublished(ctx context.Context, limit int) ([]Message, error)
	MarkPublished(ctx context.Context, id string) error
}

// Publisher delivers a message to the broker. The order of the two methods on the
// poller — publish, THEN mark — is the at-least-once guarantee.
type Publisher interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

// Poller drains the outbox on an interval.
type Poller struct {
	store    Store
	pub      Publisher
	log      *slog.Logger
	interval time.Duration
	batch    int
}

// Option configures a Poller (functional-options pattern: optional settings
// without a sprawling constructor or a config struct callers must fully populate).
type Option func(*Poller)

func WithInterval(d time.Duration) Option { return func(p *Poller) { p.interval = d } }
func WithBatchSize(n int) Option          { return func(p *Poller) { p.batch = n } }

func NewPoller(store Store, pub Publisher, log *slog.Logger, opts ...Option) *Poller {
	p := &Poller{store: store, pub: pub, log: log, interval: time.Second, batch: 100}
	for _, o := range opts {
		o(p)
	}
	return p
}

// RunOnce drains a single batch: for each unpublished message, publish then mark.
// If publish (or mark) fails, it returns the error WITHOUT marking the rest — those
// messages stay unpublished and are retried on the next drain. Never marks a
// message published unless its publish succeeded → no lost events.
func (p *Poller) RunOnce(ctx context.Context) error {
	msgs, err := p.store.FetchUnpublished(ctx, p.batch)
	if err != nil {
		return fmt.Errorf("outbox: fetch unpublished: %w", err)
	}
	for _, m := range msgs {
		if err := p.pub.Publish(ctx, m.Topic, m.Payload); err != nil {
			return fmt.Errorf("outbox: publish %s: %w", m.ID, err)
		}
		if err := p.store.MarkPublished(ctx, m.ID); err != nil {
			return fmt.Errorf("outbox: mark published %s: %w", m.ID, err)
		}
	}
	return nil
}

// Run drains immediately, then on every interval, until ctx is cancelled. A failed
// drain is logged and retried next tick — transient broker/DB errors must not kill
// the poller. Returns nil on cancellation (clean shutdown).
func (p *Poller) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		if err := p.RunOnce(ctx); err != nil {
			p.log.ErrorContext(ctx, "outbox drain failed; will retry next tick", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// LoggingPublisher is a Publisher that just logs — a placeholder until NATS is
// wired in Phase 5. It lets the whole outbox flow run and be observed now.
type LoggingPublisher struct {
	Log *slog.Logger
}

func (p LoggingPublisher) Publish(ctx context.Context, topic string, payload []byte) error {
	p.Log.InfoContext(ctx, "outbox publish", "topic", topic, "payload", string(payload))
	return nil
}

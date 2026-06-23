package events

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// The shared stream every service agrees on. Subjects use a single wildcard token
// so "order.paid", "order.confirmed", "user.registered" etc. are all captured.
const StreamName = "EVENTS"

// StreamSubjects returns the subjects the EVENTS stream captures.
func StreamSubjects() []string { return []string{"order.*", "user.*"} }

// Publisher delivers an event payload to a topic. NATSPublisher implements it (and
// outbox.Publisher, which has the same shape) so the order outbox poller can push
// to NATS, and the user service can publish directly.
type Publisher interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

// Connect opens a NATS connection and a JetStream context, and ensures a durable
// stream exists capturing our event subjects. JetStream (vs core NATS pub/sub)
// gives us PERSISTENCE and replay: messages survive a consumer being offline, and
// a new/recovering consumer can read from the start — essential for a notification
// pipeline that must not drop events.
func Connect(ctx context.Context, url, streamName string, subjects []string) (*nats.Conn, jetstream.JetStream, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, nil, fmt.Errorf("events: connect nats: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("events: jetstream: %w", err)
	}
	// CreateOrUpdateStream is idempotent — safe for every service to call on boot.
	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: subjects,
	}); err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("events: ensure stream %q: %w", streamName, err)
	}
	return nc, js, nil
}

// NATSPublisher publishes envelopes to a JetStream subject (= the event topic).
type NATSPublisher struct {
	js jetstream.JetStream
}

func NewNATSPublisher(js jetstream.JetStream) NATSPublisher {
	return NATSPublisher{js: js}
}

// Publish sends payload to subject=topic. If the payload is an envelope, its
// event_id is set as the JetStream Msg-Id header so JetStream itself dedupes
// duplicate publishes (within its dedup window) — a first line of defense before
// the consumer's own idempotency check. Belt and suspenders, because the outbox
// poller may re-publish a message it failed to mark.
func (p NATSPublisher) Publish(ctx context.Context, topic string, payload []byte) error {
	msg := &nats.Msg{Subject: topic, Data: payload}
	if env, err := Parse(payload); err == nil && env.EventID != "" {
		msg.Header = nats.Header{}
		msg.Header.Set(jetstream.MsgIDHeader, env.EventID)
	}
	if _, err := p.js.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("events: publish %s: %w", topic, err)
	}
	return nil
}

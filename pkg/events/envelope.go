// Package events defines the shared event envelope every service uses to publish
// and consume domain events, plus (later) NATS publish/consume helpers. A single
// envelope shape means any consumer can route, dedupe, and version-check any event
// without knowing its specific payload type.
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Envelope wraps every event with routing + idempotency metadata. The
// event-specific payload lives in Data as raw JSON, so the envelope is generic
// over payload types — a consumer can read event_id/topic without unmarshalling
// the body it may not care about.
//
//	event_id    — unique per event; the dedup key (delivery is at-least-once)
//	topic       — what happened, e.g. "order.paid"
//	occurred_at — when the producer created it
//	version     — payload schema version, so consumers can evolve safely
//	data        — the typed payload, as raw JSON
type Envelope struct {
	EventID    string          `json:"event_id"`
	Topic      string          `json:"topic"`
	OccurredAt time.Time       `json:"occurred_at"`
	Version    int             `json:"version"`
	Data       json.RawMessage `json:"data"`
}

// New builds an envelope around a typed payload, assigning a fresh event_id and
// timestamp. It is GENERIC ([T any]) so callers pass their domain struct directly
// — the compiler keeps producers type-safe, and the marshalling happens here once.
func New[T any](topic string, data T) (Envelope, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		EventID:    uuid.NewString(),
		Topic:      topic,
		OccurredAt: time.Now().UTC(),
		Version:    1,
		Data:       raw,
	}, nil
}

// Marshal returns the envelope's wire bytes (what goes into the outbox / onto NATS).
func (e Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Parse decodes envelope bytes received from the broker.
func Parse(b []byte) (Envelope, error) {
	var e Envelope
	err := json.Unmarshal(b, &e)
	return e, err
}

// DataAs decodes the envelope's payload into T. Generic so a consumer pulls its
// typed payload out in one call: data, err := events.DataAs[OrderPaid](env).
func DataAs[T any](e Envelope) (T, error) {
	var t T
	err := json.Unmarshal(e.Data, &t)
	return t, err
}

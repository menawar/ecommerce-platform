package events_test

import (
	"testing"

	"github.com/menawar/ecommerce-platform/pkg/events"
)

type orderPaid struct {
	OrderID    string `json:"order_id"`
	TotalCents int64  `json:"total_cents"`
}

func TestEnvelope_RoundTrip(t *testing.T) {
	env, err := events.New("order.paid", orderPaid{OrderID: "o-1", TotalCents: 5000})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if env.EventID == "" || env.Topic != "order.paid" || env.Version != 1 || env.OccurredAt.IsZero() {
		t.Fatalf("envelope metadata = %+v", env)
	}

	// Wire round-trip: marshal -> parse.
	wire, err := env.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parsed, err := events.Parse(wire)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed.EventID != env.EventID || parsed.Topic != env.Topic {
		t.Errorf("parsed = %+v, want %+v", parsed, env)
	}

	// Typed payload extraction.
	data, err := events.DataAs[orderPaid](parsed)
	if err != nil {
		t.Fatalf("DataAs: %v", err)
	}
	if data.OrderID != "o-1" || data.TotalCents != 5000 {
		t.Errorf("data = %+v", data)
	}
}

func TestEnvelope_UniqueEventIDs(t *testing.T) {
	a, _ := events.New("t", orderPaid{})
	b, _ := events.New("t", orderPaid{})
	if a.EventID == b.EventID {
		t.Error("event_ids must be unique per event")
	}
}

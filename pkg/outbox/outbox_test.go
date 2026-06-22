package outbox_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/menawar/ecommerce-platform/pkg/outbox"
)

// fakeStore is an in-memory outbox: messages start unpublished and leave the
// FetchUnpublished result once marked.
type fakeStore struct {
	mu        sync.Mutex
	pending   []outbox.Message
	published map[string]bool
}

func newFakeStore(msgs ...outbox.Message) *fakeStore {
	return &fakeStore{pending: msgs, published: map[string]bool{}}
}

func (f *fakeStore) FetchUnpublished(_ context.Context, limit int) ([]outbox.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []outbox.Message
	for _, m := range f.pending {
		if !f.published[m.ID] {
			out = append(out, m)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) MarkPublished(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published[id] = true
	return nil
}

func (f *fakeStore) isPublished(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.published[id]
}

type fakePublisher struct {
	mu        sync.Mutex
	delivered []string
	failTopic string // publishing this topic returns an error
}

func (p *fakePublisher) Publish(_ context.Context, topic string, _ []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if topic == p.failTopic {
		return errors.New("broker down")
	}
	p.delivered = append(p.delivered, topic)
	return nil
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRunOnce_PublishesThenMarks(t *testing.T) {
	store := newFakeStore(
		outbox.Message{ID: "1", Topic: "order.paid", Payload: []byte(`{}`)},
		outbox.Message{ID: "2", Topic: "order.confirmed", Payload: []byte(`{}`)},
	)
	pub := &fakePublisher{}
	p := outbox.NewPoller(store, pub, discard())

	if err := p.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.delivered) != 2 {
		t.Errorf("delivered %d, want 2", len(pub.delivered))
	}
	if !store.isPublished("1") || !store.isPublished("2") {
		t.Error("messages not marked published")
	}

	// Second drain has nothing left.
	_ = p.RunOnce(context.Background())
	if len(pub.delivered) != 2 {
		t.Errorf("re-drain delivered extra: %d", len(pub.delivered))
	}
}

// TestRunOnce_PublishFailureLeavesUnmarked is the at-least-once guarantee: a
// message whose publish FAILS must NOT be marked published, so it's retried.
func TestRunOnce_PublishFailureLeavesUnmarked(t *testing.T) {
	store := newFakeStore(outbox.Message{ID: "1", Topic: "flaky", Payload: []byte(`{}`)})
	pub := &fakePublisher{failTopic: "flaky"}
	p := outbox.NewPoller(store, pub, discard())

	if err := p.RunOnce(context.Background()); err == nil {
		t.Fatal("expected error from failed publish")
	}
	if store.isPublished("1") {
		t.Error("message marked published despite publish failure — would lose the event")
	}

	// Broker recovers -> next drain delivers it.
	pub.failTopic = ""
	if err := p.RunOnce(context.Background()); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if !store.isPublished("1") {
		t.Error("message not delivered after broker recovered")
	}
}

func TestRun_StopsOnCancel(t *testing.T) {
	p := outbox.NewPoller(newFakeStore(), &fakePublisher{}, discard(), outbox.WithInterval(10*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want nil on cancel", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop on cancel")
	}
}

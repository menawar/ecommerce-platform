// Package outboxstore adapts the order database to the generic pkg/outbox.Store
// interface, so the reusable poller can drive this service's outbox table.
package outboxstore

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/menawar/ecommerce-platform/pkg/outbox"
	"github.com/menawar/ecommerce-platform/services/order/internal/db"
)

type Store struct {
	q *db.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{q: db.New(pool)}
}

// Compile-time check that we satisfy the poller's Store interface.
var _ outbox.Store = (*Store)(nil)

func (s *Store) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Message, error) {
	rows, err := s.q.ListUnpublishedOutbox(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	msgs := make([]outbox.Message, 0, len(rows))
	for _, r := range rows {
		msgs = append(msgs, outbox.Message{
			ID:      uuid.UUID(r.ID.Bytes).String(),
			Topic:   r.Topic,
			Payload: r.Payload, // JSONB -> []byte
		})
	}
	return msgs, nil
}

func (s *Store) MarkPublished(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	return s.q.MarkOutboxPublished(ctx, pgtype.UUID{Bytes: uid, Valid: true})
}

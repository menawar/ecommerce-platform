// Package postgres provides a shared pgx connection-pool constructor. Every
// service that talks to Postgres builds its pool through here, so pool sizing and
// fail-fast behavior are consistent across the platform.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates and verifies a pgx connection pool.
//
// Why a pool: opening a Postgres connection is expensive (TCP + TLS + auth +
// backend process startup). Doing that per query would cap throughput at a
// handful of queries/sec. pgxpool keeps a set of warm connections and lends one
// out per query, returning it afterwards — so a gRPC server handling many
// concurrent requests shares a bounded set of connections instead of stampeding
// the database with one per goroutine.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	// MaxConns bounds load on the DB; without a cap a traffic spike could open
	// thousands of connections and exhaust Postgres' max_connections.
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour      // recycle connections so a long-lived one can't pin a stale server state
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}

	// NewWithConfig is lazy (no connection yet). Ping forces one real connection
	// so we fail fast at startup on a wrong URL/credentials/unreachable DB,
	// instead of discovering it on the first user request.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

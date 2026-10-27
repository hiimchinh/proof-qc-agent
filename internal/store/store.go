// Package store keeps every demo review in Postgres: what was asked, what the
// model answered and what it cost, so the daily spend cap reads real numbers.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
)

// Review is one billed model call. A call that failed has an empty decision but
// still costs money, so it is saved too.
type Review struct {
	ID        int64
	CreatedAt time.Time
	ClientIP  string
	Model     string
	Order     artwork.Order
	Result    agent.Result
	CostUSD   float64
}

// Store is a Postgres connection pool.
type Store struct{ pool *pgxpool.Pool }

// Open connects and brings the schema up to date before returning.
func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases every connection.
func (s *Store) Close() { s.pool.Close() }

// Ping checks that the database answers.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Save records a review and returns it with its id and time.
func (s *Store) Save(ctx context.Context, r Review) (Review, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO reviews (client_ip, model, "order", result, cost_usd)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		r.ClientIP, r.Model, r.Order, r.Result, r.CostUSD,
	).Scan(&r.ID, &r.CreatedAt)
	if err != nil {
		return Review{}, fmt.Errorf("save review: %w", err)
	}
	return r, nil
}

// Recent returns the newest answered reviews first; failed calls have nothing to show.
func (s *Store) Recent(ctx context.Context, limit int) ([]Review, error) {
	if limit < 1 {
		return nil, errors.New("recent reviews: limit must be at least 1")
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, created_at, client_ip, model, "order", result, cost_usd
		 FROM reviews WHERE result->>'decision' <> ''
		 ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent reviews: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Review, error) {
		var r Review
		err := row.Scan(&r.ID, &r.CreatedAt, &r.ClientIP, &r.Model, &r.Order, &r.Result, &r.CostUSD)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("recent reviews: %w", err)
	}
	return out, nil
}

// SpentSince sums the cost of every call made at or after t, failed ones included.
func (s *Store) SpentSince(ctx context.Context, t time.Time) (float64, error) {
	var usd float64
	err := s.pool.QueryRow(ctx, "SELECT COALESCE(SUM(cost_usd), 0) FROM reviews WHERE created_at >= $1", t).Scan(&usd)
	if err != nil {
		return 0, fmt.Errorf("spend since %s: %w", t.Format(time.RFC3339), err)
	}
	return usd, nil
}

package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
)

const orderColumns = `id,case_id,"order",expected,status,created_at,updated_at`

func scanOrder(row pgx.Row) (DeskOrder, error) {
	var o DeskOrder
	err := row.Scan(&o.ID, &o.CaseID, &o.Order, &o.Expected, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

func (s *Store) Enqueue(ctx context.Context, caseID string, order artwork.Order, expected agent.Decision) (DeskOrder, error) {
	return scanOrder(s.pool.QueryRow(ctx, `INSERT INTO desk_orders(case_id,"order",expected) VALUES($1,$2,$3) RETURNING `+orderColumns, caseID, order, expected))
}

func (s *Store) ClaimOrder(ctx context.Context) (*DeskOrder, error) {
	o, err := scanOrder(s.pool.QueryRow(ctx, `UPDATE desk_orders SET status='working',updated_at=now() WHERE id=(
 SELECT id FROM desk_orders WHERE status='queued' ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1
 ) RETURNING `+orderColumns))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.Runs, err = s.OrderRuns(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) FinishOrder(ctx context.Context, id int64, status string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE desk_orders SET status=$2,updated_at=now() WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("order %d: %w", id, pgx.ErrNoRows)
	}
	return nil
}

func (s *Store) RecentOrders(ctx context.Context, limit int) ([]DeskOrder, error) {
	if limit < 1 {
		return nil, errors.New("recent orders: limit must be positive")
	}
	rows, err := s.pool.Query(ctx, `SELECT `+orderColumns+` FROM desk_orders ORDER BY created_at DESC,id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	orders, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (DeskOrder, error) { return scanOrder(row) })
	if err != nil {
		return nil, err
	}
	for i := range orders {
		orders[i].Runs, err = s.OrderRuns(ctx, orders[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return orders, nil
}

func (s *Store) CountOrders(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM desk_orders`).Scan(&n)
	return n, err
}

// ObservedOrders returns the labelled stream actually seen by at least one model.
// A queued backlog must not change the baseline used to evaluate those runs.
func (s *Store) ObservedOrders(ctx context.Context) ([]DeskOrder, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+orderColumns+` FROM desk_orders
 WHERE EXISTS (SELECT 1 FROM agent_runs WHERE order_id=desk_orders.id) ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (DeskOrder, error) { return scanOrder(row) })
}

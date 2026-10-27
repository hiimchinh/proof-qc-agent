package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *Store) InitFleet(ctx context.Context, champion, challenger string) (FleetState, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO fleet_state(champion,challenger) VALUES($1,$2) ON CONFLICT DO NOTHING`, champion, challenger)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		for _, model := range []string{champion, challenger} {
			if model != "" {
				if err := saveEvent(ctx, tx, FleetEvent{Kind: "added", Model: model, Reason: "initial fleet"}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return FleetState{}, err
	}
	return s.Fleet(ctx)
}
func (s *Store) Fleet(ctx context.Context) (FleetState, error) {
	var state FleetState
	err := s.pool.QueryRow(ctx, `SELECT champion,challenger FROM fleet_state WHERE singleton`).Scan(&state.Champion, &state.Challenger)
	return state, err
}
func saveEvent(ctx context.Context, tx pgx.Tx, event FleetEvent) error {
	if len(event.Numbers) == 0 {
		event.Numbers = []byte(`{}`)
	}
	_, err := tx.Exec(ctx, `INSERT INTO fleet_events(kind,model,reason,numbers) VALUES($1,$2,$3,$4)`, event.Kind, event.Model, event.Reason, event.Numbers)
	return err
}
func (s *Store) ApplyFleet(ctx context.Context, state FleetState, events []FleetEvent) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE fleet_state SET champion=$1,challenger=$2 WHERE singleton`, state.Champion, state.Challenger)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		for _, event := range events {
			if err := saveEvent(ctx, tx, event); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Store) FleetEvents(ctx context.Context, limit int) ([]FleetEvent, error) {
	if limit < 1 {
		return nil, errors.New("fleet events: limit must be positive")
	}
	rows, err := s.pool.Query(ctx, `SELECT id,kind,model,reason,numbers,created_at FROM fleet_events ORDER BY created_at DESC,id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (FleetEvent, error) {
		var e FleetEvent
		err := row.Scan(&e.ID, &e.Kind, &e.Model, &e.Reason, &e.Numbers, &e.CreatedAt)
		return e, err
	})
}

// FleetModels returns all membership history, including active seats for older
// databases. Do not limit this query to the events displayed by the web desk.
func (s *Store) FleetModels(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT model FROM (
  SELECT model FROM fleet_events
  UNION SELECT champion FROM fleet_state
  UNION SELECT challenger FROM fleet_state
 ) AS history WHERE model <> '' ORDER BY model`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

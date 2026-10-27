package store

import (
	"context"
	"errors"
	"time"
)

// tickLockClass and the schema name together key the tick lock, so deployments
// sharing one database in different schemas never block each other.
const tickLockClass = 731_025

// WithTickLock serializes tick and manual spending across instances. The callback
// runs only when this session owns the lock; contention returns false immediately.
func (s *Store) WithTickLock(ctx context.Context, fn func(context.Context) error) (held bool, err error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Release()
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1, hashtext(current_schema()))`, tickLockClass).Scan(&held); err != nil || !held {
		return held, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		unlockErr := conn.QueryRow(cleanup, `SELECT pg_advisory_unlock($1, hashtext(current_schema()))`, tickLockClass).Scan(&unlocked)
		if unlockErr != nil || !unlocked {
			// A connection with an unverified session lock must never return to the pool.
			_ = conn.Conn().Close(cleanup)
			if unlockErr == nil {
				unlockErr = errors.New("tick lock was not held")
			}
			err = errors.Join(err, unlockErr)
		}
	}()
	return true, fn(ctx)
}

// RecoverOrders requeues abandoned work. Call while holding WithTickLock.
func (s *Store) RecoverOrders(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE desk_orders SET status='queued',updated_at=now() WHERE status='working' AND updated_at < now()-interval '2 minutes'`)
	return int(tag.RowsAffected()), err
}

// NextCaseIndex persists its cursor independently of queue cleanup and restarts.
func (s *Store) NextCaseIndex(ctx context.Context, size int) (int, error) {
	if size < 1 {
		return 0, errors.New("case cursor: size must be positive")
	}
	var index int
	err := s.pool.QueryRow(ctx, `SELECT nextval('desk_case_cursor') % $1`, size).Scan(&index)
	return index, err
}

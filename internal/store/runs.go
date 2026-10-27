package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
)

const runColumns = `id,order_id,role,model,result,expected,correct,notes_pass,cost_usd,latency_ns,trace,overridden_by_rules,error,created_at`

func scanRun(row pgx.CollectableRow) (AgentRun, error) {
	var r AgentRun
	err := row.Scan(&r.ID, &r.OrderID, &r.Role, &r.Model, &r.Result, &r.Expected, &r.Correct, &r.NotesPass, &r.CostUSD, &r.Latency, &r.Trace, &r.OverriddenByRules, &r.Error, &r.CreatedAt)
	return r, err
}
func (s *Store) SaveRun(ctx context.Context, r AgentRun) (AgentRun, error) {
	if len(r.Trace) == 0 {
		r.Trace = []byte(`[]`)
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO agent_runs(order_id,role,model,result,expected,correct,notes_pass,cost_usd,latency_ns,trace,overridden_by_rules,error)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id,created_at`, r.OrderID, r.Role, r.Model, r.Result, r.Expected, r.Correct, r.NotesPass, r.CostUSD, int64(r.Latency), r.Trace, r.OverriddenByRules, r.Error).Scan(&r.ID, &r.CreatedAt)
	return r, err
}
func (s *Store) Runs(ctx context.Context) ([]AgentRun, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+runColumns+` FROM agent_runs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanRun)
}
func (s *Store) OrderRuns(ctx context.Context, id int64) ([]AgentRun, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+runColumns+` FROM agent_runs WHERE order_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanRun)
}
func (s *Store) CustomerHistory(ctx context.Context, customer string) ([]agent.HistoryEntry, error) {
	rows, err := s.pool.Query(ctx, `SELECT o."order",r.result->>'decision' FROM desk_orders o JOIN agent_runs r ON r.order_id=o.id
 WHERE o.status='done' AND o."order"->>'customer'=$1 AND r.role='champion' AND r.error=''
 ORDER BY o.created_at DESC,o.id DESC LIMIT 10`, customer)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (agent.HistoryEntry, error) {
		var h agent.HistoryEntry
		err := row.Scan(&h.Order, &h.Decision)
		return h, err
	})
}

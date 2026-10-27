package desk

import (
	"context"
	"errors"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
)

// evaluate is called only while Tick owns the schema-scoped advisory lock.
func (s *Service) evaluate(ctx context.Context) error {
	runs, err := s.cfg.Store.Runs(ctx)
	if err != nil {
		return err
	}
	state, err := s.cfg.Store.Fleet(ctx)
	if err != nil {
		return err
	}
	// Compare with rules on the same observed orders, including failed model runs.
	baseline := make([]eval.Outcome, 0, len(runs))
	// Each model sees the same stream; build its baseline from the processed queue.
	recent, err := s.cfg.Store.ObservedOrders(ctx)
	if err != nil {
		return err
	}
	for _, o := range recent {
		c, ok := s.byID[o.CaseID]
		if !ok {
			return errors.New("desk: unknown queued case")
		}
		result, ok := s.baseline[c.ID]
		if !ok {
			img, err := s.image(c)
			if err != nil {
				return err
			}
			result = agent.Baseline(img, o.Order)
			s.baseline[c.ID] = result
		}
		baseline = append(baseline, eval.Outcome{Case: c, Result: result})
	}
	history, err := s.cfg.Store.FleetModels(ctx)
	if err != nil {
		return err
	}
	next, events := fleet.Evaluate(state, runs, eval.Score("baseline-rules", baseline))
	next, additions := fleet.SeatNext(next, history, s.cfg.ChallengerQueue)
	events = append(events, additions...)
	if len(events) == 0 {
		return nil
	}
	return s.cfg.Store.ApplyFleet(ctx, next, events)
}

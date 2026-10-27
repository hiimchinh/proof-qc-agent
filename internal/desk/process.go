package desk

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

var ErrCapped = errors.New("daily model budget spent; order remains queued")

func (s *Service) budget(ctx context.Context, pending float64) error {
	spent, err := s.cfg.Store.SpentSince(ctx, midnight(time.Now()))
	if err != nil {
		return err
	}
	if spent+pending >= s.cfg.DailyCapUSD {
		return ErrCapped
	}
	return nil
}

type budgetClient struct {
	service *Service
	pending float64
}

func (b *budgetClient) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	if err := b.service.budget(ctx, b.pending); err != nil {
		return llm.Response{}, err
	}
	res, err := b.service.cfg.LLM.Complete(ctx, req)
	b.pending += res.CostUSD
	return res, err
}

func (s *Service) process(ctx context.Context, o store.DeskOrder, state store.FleetState) (bool, error) {
	finish := func(status string) error {
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return s.cfg.Store.FinishOrder(saveCtx, o.ID, status)
	}
	c, ok := s.byID[o.CaseID]
	if !ok {
		return false, errors.Join(errors.New("desk: unknown queued case"), finish("failed"))
	}
	img, err := s.image(c)
	if err != nil {
		return false, errors.Join(err, finish("failed"))
	}
	failed := false
	for _, member := range []struct{ role, model string }{{"champion", state.Champion}, {"challenger", state.Challenger}} {
		if member.model == "" {
			continue
		}
		found := false
		for _, run := range o.Runs {
			if run.Role == member.role {
				found = true
				failed = failed || run.Error != ""
			}
		}
		if found {
			continue
		}
		if err := s.budget(ctx, 0); err != nil {
			if errors.Is(err, ErrCapped) {
				return true, finish("queued")
			}
			return false, errors.Join(err, finish("queued"))
		}
		start := time.Now()
		reviewer := agent.Desk{Model: member.model, LLM: &budgetClient{service: s}, History: s.cfg.Store.CustomerHistory}
		res, callErr := reviewer.Handle(ctx, img, o.Order)
		// A call failure remains a scored miss and its billed usage remains in the ledger.
		trace, err := json.Marshal(res.Trace)
		if err != nil {
			return false, errors.Join(err, finish("failed"))
		}
		run := store.AgentRun{OrderID: o.ID, Role: member.role, Model: member.model, Result: res, Expected: o.Expected,
			Correct: callErr == nil && res.Decision == o.Expected, NotesPass: callErr == nil && len(res.Lint) == 0,
			CostUSD: res.CostUSD, Latency: time.Since(start), Trace: trace, OverriddenByRules: res.OverriddenByRules}
		if callErr != nil {
			run.Error = callErr.Error()
			failed = true
		}
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		_, err = s.cfg.Store.SaveRun(saveCtx, run)
		cancel()
		if err != nil {
			return false, errors.Join(err, finish("failed"))
		}
		if errors.Is(callErr, ErrCapped) {
			return true, finish("queued")
		}
		if ctx.Err() != nil {
			return false, errors.Join(ctx.Err(), finish("queued"))
		}
	}
	status := "done"
	if failed {
		status = "failed"
	}
	return false, finish(status)
}

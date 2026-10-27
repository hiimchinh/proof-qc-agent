// Package desk processes a simulated order stream and manages its model fleet.
package desk

import (
	"context"
	"errors"
	"image"
	"io/fs"
	"math/rand/v2"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

const (
	DefaultChampion   = "openai/gpt-6.1-terra"
	DefaultChallenger = "anthropic/claude-haiku-5.5"
	ordersPerTick     = 2
)

type Config struct {
	Store                *store.Store
	LLM                  agent.Completer
	Cases                []eval.Case
	Artwork              fs.FS
	DailyCapUSD          float64
	Champion, Challenger string
}

type Service struct {
	cfg      Config
	shuffled []eval.Case
	byID     map[string]eval.Case
	baseline map[string]agent.Result
}

type TickResult struct {
	Enqueued  int64 `json:"enqueued"`
	Processed int   `json:"processed"`
	Capped    bool  `json:"capped"`
	Busy      bool  `json:"busy"`
	Paused    bool  `json:"paused"`
}

func New(cfg Config) (*Service, error) {
	if cfg.Store == nil || cfg.LLM == nil || cfg.Artwork == nil || len(cfg.Cases) == 0 {
		return nil, errors.New("desk: store, LLM, artwork and cases required")
	}
	if cfg.Champion == "" {
		cfg.Champion = DefaultChampion
	}
	if cfg.Challenger == "" {
		cfg.Challenger = DefaultChallenger
	}
	if cfg.Champion == cfg.Challenger {
		return nil, errors.New("desk: champion and challenger must differ")
	}
	if _, err := cfg.Store.InitFleet(context.Background(), cfg.Champion, cfg.Challenger); err != nil {
		return nil, err
	}
	s := &Service{cfg: cfg, byID: make(map[string]eval.Case, len(cfg.Cases)), baseline: map[string]agent.Result{}}
	indices := rand.New(rand.NewPCG(42, 54)).Perm(len(cfg.Cases))
	for _, i := range indices {
		s.shuffled = append(s.shuffled, cfg.Cases[i])
		s.byID[cfg.Cases[i].ID] = cfg.Cases[i]
	}
	return s, nil
}

func (s *Service) Tick(ctx context.Context) (TickResult, error) {
	var out TickResult
	locked, err := s.cfg.Store.WithTickLock(ctx, func(ctx context.Context) error {
		if _, err := s.cfg.Store.RecoverOrders(ctx); err != nil {
			return err
		}
		i, err := s.cfg.Store.NextCaseIndex(ctx, len(s.shuffled))
		if err != nil {
			return err
		}
		c := s.shuffled[i]
		order, err := s.cfg.Store.Enqueue(ctx, c.ID, c.Order, c.Want)
		if err != nil {
			return err
		}
		out.Enqueued = order.ID
		for range ordersPerTick {
			if err := s.budget(ctx, 0); err != nil {
				if errors.Is(err, ErrCapped) {
					out.Capped = true
					return nil
				}
				return err
			}
			state, err := s.cfg.Store.Fleet(ctx)
			if err != nil {
				return err
			}
			if state.Champion == "" && state.Challenger == "" {
				out.Paused = true
				return nil
			}
			next, err := s.cfg.Store.ClaimOrder(ctx)
			if err != nil || next == nil {
				return err
			}
			capped, err := s.process(ctx, *next, state)
			if capped {
				out.Capped = true
				return err
			}
			if err != nil {
				return err
			}
			out.Processed++
			if err := s.evaluate(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	out.Busy = !locked
	return out, err
}

func (s *Service) image(c eval.Case) (image.Image, error) {
	f, err := s.cfg.Artwork.Open(c.File)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

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
	next, events := fleet.Evaluate(state, runs, eval.Score("baseline-rules", baseline))
	if len(events) == 0 {
		return nil
	}
	return s.cfg.Store.ApplyFleet(ctx, next, events)
}

func midnight(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

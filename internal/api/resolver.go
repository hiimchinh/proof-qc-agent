package api

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

const (
	reviewsPerHour = 20
	maxRecent      = 50
)

type resolver struct {
	challengerQueue []string
	store           *store.Store
	llm             agent.Completer
	cases           map[string]eval.Case
	art             fs.FS
	samples         []sampleView
	board           []scoreView
	capUSD          float64
	limiter         *limiter
	now             func() time.Time
	log             *slog.Logger
}

func newResolver(cfg Config) *resolver {
	r := &resolver{
		store: cfg.Store, llm: cfg.LLM, art: cfg.Artwork, capUSD: cfg.DailyCapUSD,
		challengerQueue: cfg.ChallengerQueue,
		cases:           make(map[string]eval.Case, len(cfg.Cases)),
		samples:         make([]sampleView, 0, len(cfg.Cases)),
		board:           make([]scoreView, 0, len(cfg.Leaderboard)),
		limiter:         newLimiter(reviewsPerHour, time.Hour, time.Now),
		now:             time.Now,
		log:             cfg.Logger,
	}
	for _, c := range cfg.Cases {
		r.cases[c.ID] = c
		r.samples = append(r.samples, newSampleView(c))
	}
	for _, s := range cfg.Leaderboard {
		r.board = append(r.board, newScoreView(s))
	}
	return r
}

func (r *resolver) Leaderboard() []scoreView { return r.board }

func (r *resolver) Samples() []sampleView { return r.samples }

func (r *resolver) SpentToday(ctx context.Context) (float64, error) {
	usd, err := r.store.SpentSince(ctx, utcMidnight(r.now()))
	if err != nil {
		r.log.ErrorContext(ctx, "spent today", "err", err)
		return 0, errInternal
	}
	return usd, nil
}

// RecentReviews takes the limit as a value: the schema's default fills it in.
func (r *resolver) RecentReviews(ctx context.Context, args struct{ Limit int32 }) ([]reviewView, error) {
	limit := args.Limit
	if limit < 1 || limit > maxRecent {
		return nil, &apiError{codeBadInput, fmt.Sprintf("limit must be between 1 and %d", maxRecent)}
	}
	reviews, err := r.store.Recent(ctx, int(limit))
	if err != nil {
		r.log.ErrorContext(ctx, "recent reviews", "err", err)
		return nil, errInternal
	}
	out := make([]reviewView, 0, len(reviews))
	for _, rv := range reviews {
		out = append(out, newReviewView(rv))
	}
	return out, nil
}

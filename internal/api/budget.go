package api

import (
	"context"

	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

// reviewBudget includes the current review's unpersisted completions, so a note
// repair cannot start after the first answer spends the day's remaining budget.
type reviewBudget struct {
	resolver *resolver
	pending  float64
}

func (b *reviewBudget) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	spent, err := b.resolver.store.SpentSince(ctx, utcMidnight(b.resolver.now()))
	if err != nil {
		return llm.Response{}, err
	}
	if spent+b.pending >= b.resolver.capUSD {
		return llm.Response{}, &apiError{codeDailyCap, "today's model budget is spent; note repair was not started"}
	}
	res, err := b.resolver.llm.Complete(ctx, req)
	b.pending += res.CostUSD
	return res, err
}

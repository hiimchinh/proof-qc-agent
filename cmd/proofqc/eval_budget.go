package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

// toolCallBudget meters each completion, while eval.Run aggregates whole orders.
// In-flight calls may finish after the cap; no further completion starts then.
type toolCallBudget struct {
	mu         sync.Mutex
	max, spent float64
}

func (b *toolCallBudget) beforeCall(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.spent >= b.max {
		return fmt.Errorf("eval budget exhausted")
	}
	return nil
}

type budgetedCompleter struct {
	agent.Completer
	budget *toolCallBudget
}

func (c budgetedCompleter) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	res, err := c.Completer.Complete(ctx, req)
	c.budget.mu.Lock()
	c.budget.spent += res.CostUSD
	c.budget.mu.Unlock()
	return res, err
}

package api

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

// fakeLLM stands in for OpenRouter, the one external boundary; everything else is real.
type fakeLLM struct {
	mu     sync.Mutex
	calls  int
	models []string
	cost   float64
	err    error
}

func (f *fakeLLM) Complete(_ context.Context, r llm.Request) (llm.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.models = append(f.models, r.Model)
	if f.err != nil {
		return llm.Response{CostUSD: f.cost}, f.err
	}
	answer, _ := json.Marshal(map[string]any{
		"decision": "ready_for_proof", "reasons": []string{"300 dpi, clean"},
		"designer_note": "Proof as is.", "customer_note": "Thanks Brian. Your proof is on the way.",
		"requested": map[string]any{"width_in": nil, "height_in": nil, "shape": "none", "material": ""},
	})
	return llm.Response{JSON: answer, CostUSD: f.cost, Latency: 1200 * time.Millisecond}, nil
}

func (f *fakeLLM) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

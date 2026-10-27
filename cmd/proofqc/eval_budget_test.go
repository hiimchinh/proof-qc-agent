package main

import (
	"context"
	"image"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

type budgetTestLLM struct{ calls int }

func (m *budgetTestLLM) Complete(context.Context, llm.Request) (llm.Response, error) {
	m.calls++
	return llm.Response{FinishReason: "tool_calls", CostUSD: 0.02, ToolCalls: []llm.ToolCall{{ID: "measure", Type: "function", Function: llm.FunctionCall{Name: "measure_artwork", Arguments: "{}"}}}}, nil
}

func TestToolEvalChecksBudgetBetweenCallsOfSameOrder(t *testing.T) {
	meter := &toolCallBudget{max: 0.01}
	m := &budgetTestLLM{}
	img := image.NewNRGBA(image.Rect(0, 0, 30, 30))
	o := artwork.Order{Customer: "Brian", Product: artwork.DieCut, WidthIn: 3, HeightIn: 3}
	res, err := (agent.Desk{LLM: budgetedCompleter{Completer: m, budget: meter}, BeforeCall: meter.beforeCall}).Handle(context.Background(), img, o)
	if err == nil || !strings.Contains(err.Error(), "budget") || res.Attempts != 1 || m.calls != 1 || res.CostUSD != 0.02 {
		t.Fatalf("result=%+v calls=%d err=%v", res, m.calls, err)
	}
}

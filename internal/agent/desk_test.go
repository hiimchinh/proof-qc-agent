package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

type toolScript struct {
	replies  []llm.Response
	requests []llm.Request
	err      error
}

func (s *toolScript) Complete(_ context.Context, r llm.Request) (llm.Response, error) {
	s.requests = append(s.requests, r)
	if len(s.replies) == 0 {
		return llm.Response{}, errors.New("unexpected call")
	}
	res := s.replies[0]
	s.replies = s.replies[1:]
	return res, s.err
}
func toolResponse(name, args string) llm.Response {
	return llm.Response{FinishReason: "tool_calls", CostUSD: 0.001, ToolCalls: []llm.ToolCall{{ID: name, Type: "function", Function: llm.FunctionCall{Name: name, Arguments: args}}}}
}
func routeArgs(decision Decision, note string) string {
	var ans map[string]any
	_ = json.Unmarshal([]byte(cannedAnswer(string(decision), note)), &ans)
	ans["requested"] = map[string]any{"width_in": nil, "height_in": nil, "shape": "none", "material": ""}
	raw, _ := json.Marshal(ans)
	return string(raw)
}
func TestDeskMeasuresLooksUpHistoryAndRoutes(t *testing.T) {
	m := &toolScript{replies: []llm.Response{toolResponse("measure_artwork", "{}"), toolResponse("customer_history", "{}"), toolResponse("route_order", routeArgs(ReadyForProof, "Thanks Brian. Your proof is on the way."))}}
	before, history := 0, 0
	d := Desk{Model: "offline", LLM: m, BeforeCall: func(context.Context) error { before++; return nil }, History: func(_ context.Context, customer string) ([]HistoryEntry, error) {
		history++
		if customer != "Brian Smith" {
			t.Fatal(customer)
		}
		return []HistoryEntry{{Order: order, Decision: AskCustomer}}, nil
	}}
	res, err := d.Handle(context.Background(), artworkImage(150), order)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != ReadyForProof || res.Attempts != 3 || res.CostUSD != 0.003 || before != 3 || history != 1 || len(res.Trace) != 3 || res.Preflight.EffectiveDPI != 150 {
		t.Fatalf("result=%+v before=%d history=%d", res, before, history)
	}
	if len(m.requests[0].Tools) != 3 || !strings.Contains(m.requests[1].Messages[1].Content.(string), "effective_dpi") || m.requests[1].Messages[1].ToolCallID != "measure_artwork" {
		t.Fatalf("requests=%+v", m.requests)
	}
	if !strings.Contains(m.requests[2].Messages[3].Content.(string), "ask_customer") {
		t.Fatal("history not returned to the model")
	}
}
func TestDeskRejectsRouteBeforeMeasurement(t *testing.T) {
	m := &toolScript{replies: []llm.Response{toolResponse("route_order", routeArgs(ReadyForProof, "Thanks Brian."))}}
	res, err := (Desk{LLM: m}).Handle(context.Background(), artworkImage(150), order)
	if err == nil || !strings.Contains(err.Error(), "measure_artwork") || res.Attempts != 1 || len(res.Trace) != 1 {
		t.Fatalf("result=%+v err=%v", res, err)
	}
}

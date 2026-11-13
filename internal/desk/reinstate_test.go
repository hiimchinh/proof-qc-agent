package desk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

// A desk paused because an older build retired its champion over provider
// credit errors resumes on the next tick, with no manual database change.
func TestTickReinstatesAChampionRetiredForCreditErrors(t *testing.T) {
	const champion = "google/gemma-4-31b-it"
	ctx := t.Context()
	cases, art, err := proofqc.Cases()
	if err != nil {
		t.Fatal(err)
	}
	var fixture eval.Case
	for _, c := range cases {
		if c.ID == "gen-026-contradict" {
			fixture = c
		}
	}
	if fixture.ID == "" {
		t.Fatal("fixture missing")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Messages []struct{ Role string } }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		tool, args := "measure_artwork", `{}`
		if req.Messages[len(req.Messages)-1].Role == "tool" {
			tool, args = "route_order", `{"decision":"ask_customer","reasons":["Contradictory background instructions"],"designer_note":"Wait for clarification.","customer_note":"Thanks Fatima. Should we remove the background or keep the white box?","requested":{"width_in":null,"height_in":null,"shape":"none","material":""}}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "tool_calls", "message": map[string]any{"tool_calls": []any{map[string]any{"id": tool, "type": "function", "function": map[string]string{"name": tool, "arguments": args}}}}}}, "usage": map[string]any{"cost": 0.0003}})
	}))
	defer upstream.Close()

	st := storetest.Open(t)
	svc, err := New(Config{Store: st, LLM: &llm.Client{BaseURL: upstream.URL}, Cases: []eval.Case{fixture}, Artwork: art, DailyCapUSD: 1, Champion: champion, Challenger: "anthropic/claude-haiku-5.5"})
	if err != nil {
		t.Fatal(err)
	}
	// 30 answered orders, then the five credit errors seen on 2026-11-06.
	for i := range 35 {
		o, err := st.Enqueue(ctx, fixture.ID, fixture.Order, agent.AskCustomer)
		if err != nil {
			t.Fatal(err)
		}
		run := store.AgentRun{OrderID: o.ID, Role: "champion", Model: champion, Expected: agent.AskCustomer, Correct: true, NotesPass: true, CostUSD: 0.0003, Latency: time.Second, Result: agent.Result{Decision: agent.AskCustomer}}
		status := "done"
		if i >= 30 {
			run = store.AgentRun{OrderID: o.ID, Role: "champion", Model: champion, Expected: agent.AskCustomer,
				Error: champion + ": This request requires more credits, or fewer max_tokens. You requested up to 2000 tokens, but can only afford 1380. To increase, visit ht..."}
			status = "failed"
		}
		if _, err := st.SaveRun(ctx, run); err != nil {
			t.Fatal(err)
		}
		if err := st.FinishOrder(ctx, o.ID, status); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.ApplyFleet(ctx, store.FleetState{}, []store.FleetEvent{
		{Kind: "retired", Model: "anthropic/claude-haiku-5.5", Reason: "80% accuracy (need 90%)"},
		{Kind: "retired", Model: champion, Reason: "5 consecutive failed calls; last: " + champion + ": This request requires more credits"},
	}); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Tick(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state, err := st.Fleet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Paused || res.Processed == 0 || state != (store.FleetState{Champion: champion}) {
		t.Fatalf("tick=%+v state=%+v; want the champion back and an order processed", res, state)
	}
	events, err := st.FleetEvents(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, e := range events {
		if e.Model == champion {
			kinds[e.Kind]++
		}
	}
	if kinds["reinstated"] != 1 || kinds["retired"] != 1 {
		t.Fatalf("champion events %v; want one reinstated and no second retirement", kinds)
	}
}

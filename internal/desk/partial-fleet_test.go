package desk

import (
	"testing"
	"time"

	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestTickResumesPartialOrderBeforeRetiringItsChampion(t *testing.T) {
	svc, st, _ := toolService(t, 1, false)
	cases, _, err := proofqc.Cases()
	if err != nil {
		t.Fatal(err)
	}
	cfg := svc.cfg
	cfg.Cases = cases
	cfg.ChallengerQueue = []string{"google/gemma-3-27b-it"}
	svc, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	// Thirty completed orders: each model has 27 correct, notes all pass;
	// rules get 15 correct. The cheaper champion keeps its seat at 90%.
	for i := range 30 {
		c := cases[0] // clean-diecut: rules match the ready_for_proof label.
		if i%2 == 1 {
			c = cases[11]
		} // size-change: rules miss ask_customer.
		if c.ID != "clean-diecut" && c.ID != "size-change" {
			t.Fatalf("fixture changed: %s", c.ID)
		}
		o, err := st.Enqueue(ctx, c.ID, c.Order, c.Want)
		if err != nil {
			t.Fatal(err)
		}
		for _, member := range []struct {
			role, model string
			cost        float64
		}{{"champion", DefaultChampion, 0.001}, {"challenger", DefaultChallenger, 0.002}} {
			_, err = st.SaveRun(ctx, store.AgentRun{OrderID: o.ID, Role: member.role, Model: member.model, Expected: c.Want,
				Correct: i < 27, NotesPass: true, CostUSD: member.cost, Latency: time.Second, Result: agent.Result{Decision: c.Want}})
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := st.FinishOrder(ctx, o.ID, "done"); err != nil {
			t.Fatal(err)
		}
	}
	c := cases[0]
	partial, err := st.Enqueue(ctx, c.ID, c.Order, c.Want)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := st.SaveRun(ctx, store.AgentRun{OrderID: partial.ID, Role: "champion", Model: DefaultChampion, Expected: c.Want,
		Correct: false, NotesPass: true, CostUSD: 0.001, Latency: time.Second, Result: agent.Result{Decision: agent.AskCustomer}})
	if err != nil {
		t.Fatal(err)
	}
	// The existing runs cost $0.091; the shadow's two tool calls cost $0.002.
	// Stop at $0.093 so this tick cannot process a second, unrelated order.
	svc.cfg.DailyCapUSD = 0.093
	// The champion is now below 90%, but the missing shadow still belongs to B.
	if _, err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	runs, err := st.OrderRuns(ctx, partial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].ID != saved.ID || runs[1].Role != "challenger" || runs[1].Model != DefaultChallenger {
		models := make([]string, 0, len(runs))
		for _, run := range runs {
			models = append(models, run.Role+"="+run.Model)
		}
		t.Fatalf("partial order reassigned before resume: %v", models)
	}
	state, err := st.Fleet(ctx)
	if err != nil || state.Champion != DefaultChallenger || state.Challenger != "google/gemma-3-27b-it" {
		t.Fatalf("fleet after resume=%+v err=%v", state, err)
	}
}

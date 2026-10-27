package eval

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
)

func cases(n int) []Case {
	cs := make([]Case, n)
	for i := range cs {
		cs[i] = Case{ID: string(rune('a' + i)), Want: agent.ReadyForProof}
	}
	return cs
}

func TestRunStopsCallingOnceTheBudgetIsSpent(t *testing.T) {
	var calls atomic.Int32
	review := func(Case) (agent.Result, error) {
		calls.Add(1)
		return agent.Result{Decision: agent.ReadyForProof, CostUSD: 1}, nil
	}
	b := &Budget{Max: 3}
	outs, err := Run(cases(10), review, 1, b)
	if err != nil {
		t.Fatal(err)
	}
	skipped := 0
	for _, o := range outs {
		if o.Skipped {
			skipped++
		}
	}
	if calls.Load() != 3 || skipped != 7 || b.Spent() != 3 {
		t.Fatalf("calls = %d skipped = %d spent = %v", calls.Load(), skipped, b.Spent())
	}
}

func TestRunRecordsErrorsAndKeepsOrder(t *testing.T) {
	review := func(c Case) (agent.Result, error) {
		if c.ID == "b" {
			return agent.Result{CostUSD: 0.5}, errors.New("502")
		}
		return agent.Result{Decision: agent.ReadyForProof}, nil
	}
	outs, err := Run(cases(3), review, 4, &Budget{Max: 10})
	if err != nil {
		t.Fatal(err)
	}
	if outs[1].Case.ID != "b" || outs[1].Err != "502" || outs[1].Result.CostUSD != 0.5 || outs[2].Err != "" {
		t.Fatalf("outcomes = %+v", outs)
	}
}

func TestRunRejectsNoWorkers(t *testing.T) {
	if _, err := Run(cases(1), nil, 0, &Budget{Max: 1}); err == nil {
		t.Fatal("want an error for zero workers")
	}
}

func TestLoadCasesValidatesOrders(t *testing.T) {
	path := t.TempDir() + "/cases.json"
	writeFile(t, path, `[{"id":"x","file":"x.png","want":"ready_for_proof","order":{"customer":"A","product":"diecut","width_in":3,"height_in":3}}]`)
	if _, err := LoadCases(path); err == nil {
		t.Fatal("want an error for an unknown product")
	}
}

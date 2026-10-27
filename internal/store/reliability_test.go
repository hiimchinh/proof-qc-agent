package store_test

import (
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func TestOrderedRunsRetireUnreliableChallengerAndSeatNext(t *testing.T) {
	st := storetest.Open(t)
	ctx := t.Context()
	initial, err := st.InitFleet(ctx, "good", "bad")
	if err != nil {
		t.Fatal(err)
	}
	save := func(run store.AgentRun) {
		t.Helper()
		o, err := st.Enqueue(ctx, "literal-case", artwork.Order{Customer: "Fatima Khan", Product: artwork.DieCut, WidthIn: 2, HeightIn: 2}, "ask_customer")
		if err != nil {
			t.Fatal(err)
		}
		run.OrderID = o.ID
		run.Role = "challenger"
		if _, err := st.SaveRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}
	// A success breaks the old streak. Other models' calls do not break it.
	history := []store.AgentRun{
		{Model: "bad", Error: "old failure"}, {Model: "bad", Error: "old failure"}, {Model: "bad"},
		{Model: "bad", Error: "first failure"}, {Model: "good"}, {Model: "bad", Error: "second failure"},
		{Model: "bad", Error: "third failure"}, {Model: "good"}, {Model: "bad", Error: "fourth failure"},
	}
	for _, run := range history {
		save(run)
	}
	runs, err := st.Runs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before, events := fleet.Evaluate(initial, runs, eval.Scorecard{})
	if before != initial || len(events) != 0 {
		t.Fatalf("streak did not reset: %+v %+v", before, events)
	}
	save(store.AgentRun{Model: "bad", Error: "timeout"})
	runs, err = st.Runs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(runs); i++ {
		if runs[i].ID <= runs[i-1].ID {
			t.Fatal("runs must be oldest first")
		}
	}
	next, events := fleet.Evaluate(initial, runs, eval.Scorecard{})
	if next.Challenger != "" || len(events) != 1 || events[0].Reason != "5 consecutive failed calls; last: timeout" {
		t.Fatalf("retirement=%+v %+v", next, events)
	}
	models, err := st.FleetModels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	next, added := fleet.SeatNext(next, models, []string{"bad", "next", "later"})
	if err := st.ApplyFleet(ctx, next, append(events, added...)); err != nil {
		t.Fatal(err)
	}
	saved, err := st.Fleet(ctx)
	if err != nil || saved != (store.FleetState{Champion: "good", Challenger: "next"}) {
		t.Fatalf("fleet=%+v err=%v", saved, err)
	}
	persisted, err := st.FleetEvents(ctx, 10)
	if err != nil || len(persisted) != 4 || persisted[0].Model != "next" || persisted[0].Kind != "added" || persisted[1].Kind != "retired" {
		t.Fatalf("events=%+v err=%v", persisted, err)
	}
}

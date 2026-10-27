package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func TestRunsRoundTripResumeHistoryAndSpend(t *testing.T) {
	s, ctx := storetest.Open(t), context.Background()
	o, e := s.Enqueue(ctx, "case", review(agent.ReadyForProof, 0).Order, agent.ReadyForProof)
	if e != nil {
		t.Fatal(e)
	}
	r := store.AgentRun{OrderID: o.ID, Role: "champion", Model: "a", Expected: agent.ReadyForProof, Result: review(agent.ReadyForProof, 0).Result, Correct: true, NotesPass: true, CostUSD: 0.25, Latency: 1500 * time.Millisecond, Trace: json.RawMessage(`[{"tool":"route_order"}]`), OverriddenByRules: true}
	saved, e := s.SaveRun(ctx, r)
	if e != nil || saved.ID == 0 || saved.CreatedAt.IsZero() {
		t.Fatalf("save %+v %v", saved, e)
	}
	if _, e = s.SaveRun(ctx, r); e == nil {
		t.Fatal("duplicate role accepted")
	}
	r.Role = "challenger"
	r.Error = "timeout"
	r.CostUSD = 0.5
	if _, e = s.SaveRun(ctx, r); e != nil {
		t.Fatal(e)
	}
	all, e := s.Runs(ctx)
	if e != nil || len(all) != 2 || all[0].Latency != 1500*time.Millisecond || !all[0].OverriddenByRules || string(all[0].Trace) == "" {
		t.Fatalf("runs %+v %v", all, e)
	}
	runs, e := s.OrderRuns(ctx, o.ID)
	if e != nil || len(runs) != 2 {
		t.Fatalf("order runs %+v %v", runs, e)
	}
	claimed, e := s.ClaimOrder(ctx)
	if e != nil || len(claimed.Runs) != 2 {
		t.Fatalf("resume %+v %v", claimed, e)
	}
	history, e := s.CustomerHistory(ctx, o.Order.Customer)
	if e != nil || len(history) != 0 {
		t.Fatalf("unfinished history %+v %v", history, e)
	}
	if e = s.FinishOrder(ctx, o.ID, "done"); e != nil {
		t.Fatal(e)
	}
	history, e = s.CustomerHistory(ctx, o.Order.Customer)
	if e != nil || len(history) != 1 || history[0].Decision != agent.ReadyForProof || history[0].Order.Customer != o.Order.Customer {
		t.Fatalf("history %+v %v", history, e)
	}
	orders, e := s.RecentOrders(ctx, 1)
	if e != nil || len(orders[0].Runs) != 2 {
		t.Fatalf("recent %+v %v", orders, e)
	}
	if _, e = s.Save(ctx, review("", 0.125)); e != nil {
		t.Fatal(e)
	}
	cost, e := s.SpentSince(ctx, time.Time{})
	if e != nil || cost != 0.875 {
		t.Fatalf("cost %f %v", cost, e)
	}
}

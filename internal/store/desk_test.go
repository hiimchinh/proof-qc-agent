package store_test

import (
	"context"
	"sync"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func TestDeskQueueClaimsOldestExactlyOnce(t *testing.T) {
	s, ctx := storetest.Open(t), context.Background()
	first, err := s.Enqueue(ctx, "one", review(agent.ReadyForProof, 0).Order, agent.ReadyForProof)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Enqueue(ctx, "two", first.Order, agent.AskCustomer)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ClaimOrder(ctx)
	if err != nil || got == nil || got.ID != first.ID || got.Status != "working" {
		t.Fatalf("first claim: %+v %v", got, err)
	}
	var wg sync.WaitGroup
	ids := make(chan int64, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o, e := s.ClaimOrder(ctx)
			if e != nil {
				t.Error(e)
			}
			if o != nil {
				ids <- o.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	var claimed []int64
	for id := range ids {
		claimed = append(claimed, id)
	}
	if len(claimed) != 1 || claimed[0] != second.ID {
		t.Fatalf("claims %v, want only %d", claimed, second.ID)
	}
	if err := s.FinishOrder(ctx, first.ID, "done"); err != nil {
		t.Fatal(err)
	}
	orders, err := s.RecentOrders(ctx, 2)
	if err != nil || len(orders) != 2 || orders[1].Status != "done" {
		t.Fatalf("orders %+v %v", orders, err)
	}
	if n, e := s.CountOrders(ctx); e != nil || n != 2 {
		t.Fatalf("count %d %v", n, e)
	}
	_ = store.FleetState{}
}

func TestObservedOrdersExcludeQueuedBacklog(t *testing.T) {
	s := storetest.Open(t)
	ctx := context.Background()
	o, err := s.Enqueue(ctx, "observed", artwork.Order{Customer: "Brian", Product: artwork.DieCut, WidthIn: 3, HeightIn: 3}, agent.ReadyForProof)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveRun(ctx, store.AgentRun{OrderID: o.ID, Role: "champion", Model: "first", Expected: agent.ReadyForProof, Result: agent.Result{Decision: agent.ReadyForProof}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enqueue(ctx, "queued", o.Order, agent.ReadyForProof); err != nil {
		t.Fatal(err)
	}
	orders, err := s.ObservedOrders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 1 || orders[0].CaseID != "observed" {
		t.Fatalf("observed=%+v", orders)
	}
}

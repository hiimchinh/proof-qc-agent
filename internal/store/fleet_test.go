package store_test

import (
	"context"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
	"testing"
)

func TestFleetInitializesOnceAndAppliesEventsAtomically(t *testing.T) {
	s, ctx := storetest.Open(t), context.Background()
	initial := store.FleetState{Champion: "a", Challenger: "b"}
	got, e := s.InitFleet(ctx, "a", "b")
	if e != nil || got != initial {
		t.Fatalf("init %+v %v", got, e)
	}
	got, e = s.InitFleet(ctx, "x", "y")
	if e != nil || got != initial {
		t.Fatalf("reinit %+v %v", got, e)
	}
	events, e := s.FleetEvents(ctx, 10)
	if e != nil || len(events) != 2 || events[0].Kind != "added" {
		t.Fatalf("added %+v %v", events, e)
	}
	next := store.FleetState{Champion: "b"}
	if e = s.ApplyFleet(ctx, next, []store.FleetEvent{{Kind: "promoted", Model: "b", Reason: "better", Numbers: []byte(`{"n":30}`)}}); e != nil {
		t.Fatal(e)
	}
	got, e = s.Fleet(ctx)
	if e != nil || got != next {
		t.Fatalf("state %+v %v", got, e)
	}
	if e = s.ApplyFleet(ctx, initial, []store.FleetEvent{{Kind: "bad", Numbers: []byte(`invalid`)}}); e == nil {
		t.Fatal("invalid event accepted")
	}
	got, e = s.Fleet(ctx)
	if e != nil || got != next {
		t.Fatalf("transaction leaked %+v %v", got, e)
	}
	events, e = s.FleetEvents(ctx, 1)
	if e != nil || len(events) != 1 || events[0].Kind != "promoted" || events[0].CreatedAt.IsZero() {
		t.Fatalf("events %+v %v", events, e)
	}
}

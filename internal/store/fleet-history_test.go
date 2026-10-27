package store_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func TestFleetQueuePersistsHistoryAcrossRestartAndRollsBackTogether(t *testing.T) {
	ctx := context.Background()
	url := storetest.URL(t)
	st, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { st.Close() }()
	if _, err := st.InitFleet(ctx, "a", "b"); err != nil {
		t.Fatal(err)
	}
	locked, err := st.WithTickLock(ctx, func(ctx context.Context) error {
		history, err := st.FleetModels(ctx)
		if err != nil {
			return err
		}
		// Promotion and admission must be one transaction; b cannot return later.
		next, additions := fleet.SeatNext(store.FleetState{Champion: "b"}, history, []string{"a", "b", "c", "d"})
		events := append([]store.FleetEvent{{Kind: "retired", Model: "a", Reason: "failed"}, {Kind: "promoted", Model: "b", Reason: "qualified"}}, additions...)
		if err := st.ApplyFleet(ctx, next, events); err != nil {
			return err
		}
		// An invalid event rolls back both the state and an earlier added event.
		if err := st.ApplyFleet(ctx, store.FleetState{Champion: "b", Challenger: "d"}, []store.FleetEvent{
			{Kind: "added", Model: "d", Reason: "next candidate from the queue"}, {Kind: "bad", Model: "d", Numbers: []byte(`invalid`)},
		}); err == nil {
			return fmt.Errorf("invalid batch accepted")
		}
		return nil
	})
	if err != nil || !locked {
		t.Fatalf("lock=%v err=%v", locked, err)
	}
	st.Close()
	st, err = store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InitFleet(ctx, "reset-a", "reset-b"); err != nil {
		t.Fatal(err)
	}
	state, err := st.Fleet(ctx)
	if err != nil || state != (store.FleetState{Champion: "b", Challenger: "c"}) {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	// Put a beyond-UI-history event in the ledger, then add enough later events
	// to push it outside the desk's 30-event window.
	for i := 0; i < 35; i++ {
		model := fmt.Sprintf("old-%02d", i)
		if err := st.ApplyFleet(ctx, state, []store.FleetEvent{{Kind: "added", Model: model, Reason: "historical"}}); err != nil {
			t.Fatal(err)
		}
	}
	history, err := st.FleetModels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 38 {
		t.Fatalf("history lost models or rollback leaked: %v", history)
	}
	queue := []string{"old-00", "a", "b", "c", "d"}
	if got := fleet.RemainingQueue(state, history, queue); !reflect.DeepEqual(got, []string{"d"}) {
		t.Fatalf("queue after restart=%v", got)
	}
	events, err := st.FleetEvents(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	added := 0
	for _, e := range events {
		if e.Kind == "added" && e.Model == "c" {
			added++
			if e.Reason != "next candidate from the queue" {
				t.Fatalf("reason=%q", e.Reason)
			}
		}
	}
	if added != 1 {
		t.Fatalf("c added %d times", added)
	}
}

package desk

import (
	"context"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestTickSeatsQueueUnderLockEvenWhenBudgetIsSpent(t *testing.T) {
	svc, st, calls := toolService(t, 0, false)
	ctx := context.Background()
	svc.cfg.ChallengerQueue = []string{DefaultChampion, DefaultChallenger, "google/gemma-3-27b-it", "x-ai/grok-4.7"}
	if err := st.ApplyFleet(ctx, store.FleetState{}, []store.FleetEvent{
		{Kind: "retired", Model: DefaultChampion, Reason: "failed"}, {Kind: "retired", Model: DefaultChallenger, Reason: "failed"},
	}); err != nil {
		t.Fatal(err)
	}
	locked, err := st.WithTickLock(ctx, func(ctx context.Context) error {
		out, err := svc.Tick(ctx)
		if err != nil {
			return err
		}
		state, err := st.Fleet(ctx)
		if !out.Busy || state.Challenger != "" {
			t.Fatalf("busy tick mutated fleet: %+v %+v", out, state)
		}
		return err
	})
	if err != nil || !locked {
		t.Fatalf("lock=%v err=%v", locked, err)
	}
	for _, want := range []string{"google/gemma-3-27b-it", "x-ai/grok-4.7", ""} {
		restarted, err := New(svc.cfg)
		if err != nil {
			t.Fatal(err)
		}
		out, err := restarted.Tick(ctx)
		if err != nil || !out.Capped || calls.Load() != 0 {
			t.Fatalf("out=%+v calls=%d err=%v", out, calls.Load(), err)
		}
		state, err := st.Fleet(ctx)
		if err != nil || state.Champion != "" || state.Challenger != want {
			t.Fatalf("state=%+v want challenger=%q err=%v", state, want, err)
		}
		// Repeated ticks and restarts leave an occupied seat alone.
		if _, err := restarted.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		if want != "" {
			if err := st.ApplyFleet(ctx, store.FleetState{}, []store.FleetEvent{{Kind: "retired", Model: want, Reason: "failed"}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	events, err := st.FleetEvents(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	added := 0
	for _, e := range events {
		if e.Kind == "added" {
			added++
		}
	}
	if added != 4 {
		t.Fatalf("added %d times; want two initial and two queued models", added)
	}
}

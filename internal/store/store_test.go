package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func review(decision agent.Decision, cost float64) store.Review {
	o := artwork.Order{Customer: "Brian Smith", Product: artwork.DieCut, WidthIn: 3, HeightIn: 3, Comment: "Remove the white box"}
	return store.Review{
		ClientIP: "203.0.113.7",
		Model:    "anthropic/claude-haiku-5.5",
		Order:    o,
		Result: agent.Result{
			Decision:     decision,
			CustomerNote: "Thanks Brian. Your proof is on the way.",
			OrderChanges: []string{"holographic instead of standard white vinyl"},
			Preflight:    artwork.Report{WidthPx: 900, HeightPx: 900, EffectiveDPI: 300},
			CostUSD:      cost,
			Latency:      1500 * time.Millisecond,
		},
		CostUSD: cost,
	}
}

func TestOpenMigratesOnceUnderConcurrentStarts(t *testing.T) {
	url := storetest.URL(t)
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := store.Open(context.Background(), url)
			if err != nil {
				errs <- err
				return
			}
			s.Close()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent open: %v", err)
	}
}

func TestSaveAndRecentNewestFirst(t *testing.T) {
	s, ctx := storetest.Open(t), context.Background()
	for _, d := range []agent.Decision{agent.ReadyForProof, agent.AskCustomer, agent.RecreateArtwork} {
		saved, err := s.Save(ctx, review(d, 0.0004569))
		if err != nil {
			t.Fatal(err)
		}
		if saved.ID == 0 || saved.CreatedAt.IsZero() {
			t.Fatalf("Save did not return id and time: %+v", saved)
		}
	}
	got, err := s.Recent(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Result.Decision != agent.RecreateArtwork || got[1].Result.Decision != agent.AskCustomer {
		t.Fatalf("Recent(2) = %+v, want the last two, newest first", got)
	}
	r := got[0]
	if r.Order.Comment != "Remove the white box" || r.Result.OrderChanges[0] != "holographic instead of standard white vinyl" ||
		r.Result.Preflight.EffectiveDPI != 300 || r.Result.Latency != 1500*time.Millisecond ||
		r.CostUSD != 0.0004569 || r.ClientIP != "203.0.113.7" || r.Model != "anthropic/claude-haiku-5.5" {
		t.Fatalf("round trip lost data: %+v", r)
	}
}

func TestRecentSkipsFailedCalls(t *testing.T) {
	s, ctx := storetest.Open(t), context.Background()
	if _, err := s.Save(ctx, review("", 0.002)); err != nil {
		t.Fatal(err)
	}
	got, err := s.Recent(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("Recent = %+v, a failed call has no answer to show", got)
	}
}

func TestRecentRejectsBadLimit(t *testing.T) {
	s := storetest.Open(t)
	if _, err := s.Recent(context.Background(), 0); err == nil {
		t.Fatal("Recent(0) succeeded, want an error")
	}
}

func TestSpentSinceCountsFailedCallsAfterTheCutoff(t *testing.T) {
	s, ctx := storetest.Open(t), context.Background()
	first, err := s.Save(ctx, review(agent.ReadyForProof, 0.5))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []store.Review{review(agent.AskCustomer, 0.0004569), review("", 0.0063)} {
		if _, err := s.Save(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.SpentSince(ctx, first.CreatedAt.Add(time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	if got != 0.0067569 {
		t.Fatalf("SpentSince = %v, want 0.0067569 (the two later calls, failed one included)", got)
	}
	if got, err := s.SpentSince(ctx, time.Now().Add(time.Hour)); err != nil || got != 0 {
		t.Fatalf("SpentSince(future) = %v, %v; want 0", got, err)
	}
}

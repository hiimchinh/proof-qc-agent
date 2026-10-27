package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
	"github.com/jackc/pgx/v5"
)

func TestTickLockExcludesInstancesAndReleasesAfterError(t *testing.T) {
	ctx := context.Background()
	url := storetest.URL(t)
	s, e := store.Open(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	other, e := store.Open(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	sentinel := errors.New("callback failed")
	ok, e := s.WithTickLock(ctx, func(ctx context.Context) error {
		acquired, err := other.WithTickLock(ctx, func(context.Context) error { t.Error("overlapping callback"); return nil })
		if acquired || err != nil {
			t.Errorf("contended lock %v %v", acquired, err)
		}
		return sentinel
	})
	if !ok || !errors.Is(e, sentinel) {
		t.Fatalf("lock %v %v", ok, e)
	}
	ok, e = other.WithTickLock(ctx, func(context.Context) error { return nil })
	if !ok || e != nil {
		t.Fatalf("lock leaked %v %v", ok, e)
	}
}

func TestPersistentCursorAndStaleOrderRecovery(t *testing.T) {
	ctx := context.Background()
	url := storetest.URL(t)
	s, e := store.Open(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, want := range []int{0, 1, 2, 0} {
		got, e := s.NextCaseIndex(ctx, 3)
		if e != nil || got != want {
			t.Fatalf("index %d want %d: %v", got, want, e)
		}
	}
	other, e := store.Open(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	if got, e := other.NextCaseIndex(ctx, 3); e != nil || got != 1 {
		t.Fatalf("cursor restart %d %v", got, e)
	}
	if _, e := s.NextCaseIndex(ctx, 0); e == nil {
		t.Fatal("zero size accepted")
	}
	first, e := s.Enqueue(ctx, "first", review(agent.ReadyForProof, 0).Order, agent.ReadyForProof)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ClaimOrder(ctx); e != nil {
		t.Fatal(e)
	}
	conn, e := pgx.Connect(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close(ctx)
	if _, e = conn.Exec(ctx, `UPDATE desk_orders SET updated_at=now()-interval '3 minutes' WHERE id=$1`, first.ID); e != nil {
		t.Fatal(e)
	}
	second, e := s.Enqueue(ctx, "second", first.Order, first.Expected)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ClaimOrder(ctx); e != nil {
		t.Fatal(e)
	}
	if n, e := s.RecoverOrders(ctx); e != nil || n != 1 {
		t.Fatalf("recover %d %v", n, e)
	}
	claim, e := s.ClaimOrder(ctx)
	if e != nil || claim == nil || claim.ID != first.ID {
		t.Fatalf("recovered %+v %v", claim, e)
	}
	if claim, e = s.ClaimOrder(ctx); e != nil || claim != nil {
		t.Fatalf("fresh working %d reclaimed %+v %v", second.ID, claim, e)
	}
}

func TestTickLocksInSeparateSchemasDoNotContend(t *testing.T) {
	ctx := context.Background()
	a, b := storetest.Open(t), storetest.Open(t)
	ok, e := a.WithTickLock(ctx, func(ctx context.Context) error {
		inner, err := b.WithTickLock(ctx, func(context.Context) error { return nil })
		if !inner || err != nil {
			t.Errorf("a deployment in another schema was blocked: %v %v", inner, err)
		}
		return nil
	})
	if !ok || e != nil {
		t.Fatalf("lock %v %v", ok, e)
	}
}

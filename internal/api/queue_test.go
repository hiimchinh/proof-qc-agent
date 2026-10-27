package api

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestDeskQueueExcludesPersistedHistoryWithoutModelCalls(t *testing.T) {
	s := newTestServer(t, 3)
	ctx := context.Background()
	if _, err := s.store.InitFleet(ctx, "a", "b"); err != nil {
		t.Fatal(err)
	}
	cases, art, err := proofqc.Cases()
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(Config{Store: s.store, LLM: s.llm, Cases: cases, Artwork: art, DailyCapUSD: 3,
		ChallengerQueue: []string{"a", "b", "old", "google/gemma-3-27b-it", "x-ai/grok-4.7"}})
	if err != nil {
		t.Fatal(err)
	}
	s.Handler = h
	for _, tc := range []struct {
		history []store.FleetEvent
		want    []string
	}{
		{[]store.FleetEvent{{Kind: "retired", Model: "old", Reason: "failed"}}, []string{"google/gemma-3-27b-it", "x-ai/grok-4.7"}},
		{[]store.FleetEvent{{Kind: "added", Model: "google/gemma-3-27b-it", Reason: "next candidate from the queue"}}, []string{"x-ai/grok-4.7"}},
		{[]store.FleetEvent{{Kind: "retired", Model: "x-ai/grok-4.7", Reason: "failed"}}, []string{}},
	} {
		if err := s.store.ApplyFleet(ctx, store.FleetState{Champion: "b"}, tc.history); err != nil {
			t.Fatal(err)
		}
		resp := s.do(t, "198.51.100.1", `{ desk { challengerQueue champion challenger events { reason } } }`, nil)
		if len(resp.Errors) > 0 {
			t.Fatalf("errors=%+v", resp.Errors)
		}
		var data struct {
			Desk struct{ ChallengerQueue []string }
		}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(data.Desk.ChallengerQueue, tc.want) {
			t.Fatalf("data=%s; want queue=%v", resp.Data, tc.want)
		}
	}
	if s.llm.callCount() != 0 {
		t.Fatal("desk query called model")
	}
}

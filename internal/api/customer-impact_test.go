package api

import (
	"encoding/json"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestDeskQuerySummarizesWhatCustomersGot(t *testing.T) {
	s := newTestServer(t, 3)
	ctx := t.Context()
	if _, err := s.store.InitFleet(ctx, "champion", "challenger"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct {
		decided, want agent.Decision
		err           string
	}{
		{agent.ReadyForProof, agent.ReadyForProof, ""},
		{agent.AskCustomer, agent.AskCustomer, ""},
		{agent.ReadyForProof, agent.AskCustomer, ""},
		{"", agent.ReadyForProof, "timeout"},
	} {
		o, err := s.store.Enqueue(ctx, "gen-026-contradict", artwork.Order{Customer: "Fatima Khan", Product: artwork.DieCut, WidthIn: 2, HeightIn: 2}, r.want)
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range []store.AgentRun{
			{OrderID: o.ID, Role: "champion", Model: "champion", Result: agent.Result{Decision: r.decided}, Expected: r.want, Error: r.err},
			{OrderID: o.ID, Role: "challenger", Model: "challenger", Result: agent.Result{Decision: agent.RecreateArtwork}, Expected: r.want},
		} {
			if _, err := s.store.SaveRun(ctx, run); err != nil {
				t.Fatal(err)
			}
		}
	}
	resp := s.do(t, "198.51.100.1", `{ desk { impact { orders straightToProof issuesCaught missedIssues detours handedToDesigner } } }`, nil)
	if len(resp.Errors) > 0 {
		t.Fatalf("errors=%+v", resp.Errors)
	}
	var data struct {
		Desk struct {
			Impact struct{ Orders, StraightToProof, IssuesCaught, MissedIssues, Detours, HandedToDesigner int }
		}
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatal(err)
	}
	got := data.Desk.Impact
	if got.Orders != 4 || got.StraightToProof != 1 || got.IssuesCaught != 1 || got.MissedIssues != 1 || got.Detours != 0 || got.HandedToDesigner != 1 {
		t.Fatalf("impact=%+v", got)
	}
}

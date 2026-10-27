package api

import (
	"encoding/json"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestDeskQueryKeepsShadowFailureSeparateFromCompletedOrder(t *testing.T) {
	s := newTestServer(t, 3)
	ctx := t.Context()
	if _, err := s.store.InitFleet(ctx, "champion", "challenger"); err != nil {
		t.Fatal(err)
	}
	o, err := s.store.Enqueue(ctx, "gen-026-contradict", artwork.Order{Customer: "Fatima Khan", Product: artwork.DieCut, WidthIn: 2, HeightIn: 2}, agent.AskCustomer)
	if err != nil {
		t.Fatal(err)
	}
	fullError := "google/gemma-3-27b-it: Provider returned error; provider: DeepInfra; upstream: temporarily rate-limited upstream"
	for _, run := range []store.AgentRun{
		{OrderID: o.ID, Role: "champion", Model: "champion", Result: agent.Result{Decision: agent.AskCustomer}, Correct: true, NotesPass: true},
		{OrderID: o.ID, Role: "challenger", Model: "challenger", Error: fullError},
	} {
		if _, err := s.store.SaveRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.FinishOrder(ctx, o.ID, "done"); err != nil {
		t.Fatal(err)
	}
	resp := s.do(t, "198.51.100.1", `{ desk { orders { status runs { role error errorCause } } } }`, nil)
	if len(resp.Errors) > 0 {
		t.Fatalf("errors=%+v", resp.Errors)
	}
	var data struct {
		Desk struct {
			Orders []struct {
				Status string
				Runs   []struct{ Role, Error, ErrorCause string }
			}
		}
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Desk.Orders) != 1 || data.Desk.Orders[0].Status != "done" {
		t.Fatalf("data=%s", resp.Data)
	}
	runs := data.Desk.Orders[0].Runs
	if len(runs) != 2 || runs[0].ErrorCause != "" || runs[1].Error != fullError || runs[1].ErrorCause != "provider DeepInfra rate-limited upstream" {
		t.Fatalf("data=%s", resp.Data)
	}
	if s.llm.callCount() != 0 {
		t.Fatal("read called a model")
	}
}

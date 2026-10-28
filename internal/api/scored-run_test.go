package api

import (
	"encoding/json"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestDeskQueryMarksProviderOutagesAsNotScored(t *testing.T) {
	s := newTestServer(t, 3)
	ctx := t.Context()
	if _, err := s.store.InitFleet(ctx, "champion", "challenger"); err != nil {
		t.Fatal(err)
	}
	runs := [][2]store.AgentRun{
		{{Role: "champion", Model: "champion", Result: agent.Result{Decision: agent.AskCustomer}, Correct: true, NotesPass: true},
			{Role: "challenger", Model: "challenger", Error: "challenger: Provider returned error; provider: Google AI Studio; upstream: temporarily rate-limited upstream"}},
		{{Role: "champion", Model: "champion", Error: "timeout"},
			{Role: "challenger", Model: "challenger", Error: "challenger: This request would exceed your available credits"}},
	}
	for _, pair := range runs {
		o, err := s.store.Enqueue(ctx, "gen-026-contradict", artwork.Order{Customer: "Fatima Khan", Product: artwork.DieCut, WidthIn: 2, HeightIn: 2}, agent.AskCustomer)
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range pair {
			run.OrderID = o.ID
			if _, err := s.store.SaveRun(ctx, run); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.store.FinishOrder(ctx, o.ID, "done"); err != nil {
			t.Fatal(err)
		}
	}
	resp := s.do(t, "198.51.100.1", `{ desk { orders { runs { role scored } } } }`, nil)
	if len(resp.Errors) > 0 {
		t.Fatalf("errors=%+v", resp.Errors)
	}
	var data struct {
		Desk struct {
			Orders []struct {
				Runs []struct {
					Role   string
					Scored bool
				}
			}
		}
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatal(err)
	}
	// Newest order first: a model timeout counts against the model, a credit error does not.
	want := [][]bool{{true, false}, {true, false}}
	if len(data.Desk.Orders) != 2 {
		t.Fatalf("data=%s", resp.Data)
	}
	for i, o := range data.Desk.Orders {
		if len(o.Runs) != 2 || o.Runs[0].Scored != want[i][0] || o.Runs[1].Scored != want[i][1] {
			t.Fatalf("order %d runs=%+v want scored=%v", i, o.Runs, want[i])
		}
	}
}

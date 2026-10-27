package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	"encoding/json"
	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
	"strings"
	"testing"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestDeskQueryStartsEmptyAndKeepsExistingSchema(t *testing.T) {
	s := newTestServer(t, 3)
	if _, err := s.store.InitFleet(context.Background(), "champion", "challenger"); err != nil {
		t.Fatal(err)
	}
	resp := s.do(t, "198.51.100.1", `{ desk { simulated spentToday dailyCapUsd champion challenger orders { id status runs { role trace { tool } } } scoreboard { model n } events { kind reason } } samples { id } }`, nil)
	if len(resp.Errors) > 0 {
		t.Fatalf("errors: %+v", resp.Errors)
	}
	var d struct {
		Desk struct {
			Simulated                  bool
			SpentToday, DailyCapUsd    float64
			Orders, Scoreboard, Events []json.RawMessage
		}
		Samples []json.RawMessage
	}
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		t.Fatal(err)
	}
	if !d.Desk.Simulated || d.Desk.SpentToday != 0 || d.Desk.DailyCapUsd != 3 || len(d.Samples) != 106 || d.Desk.Orders == nil || d.Desk.Scoreboard == nil || d.Desk.Events == nil {
		t.Fatalf("data = %s", resp.Data)
	}
}

func TestDeskQueryRejectsUnboundedOrders(t *testing.T) {
	s := newTestServer(t, 3)
	if _, err := s.store.InitFleet(context.Background(), "champion", "challenger"); err != nil {
		t.Fatal(err)
	}
	resp := s.do(t, "198.51.100.1", `{ desk(limit: 51) { spentToday } }`, nil)
	if resp.code() != "BAD_INPUT" || !strings.Contains(resp.Errors[0].Message, "50") {
		t.Fatalf("errors = %+v", resp.Errors)
	}
}

func TestDeskQueryReturnsPersistedRunTraceAndFleetEvidence(t *testing.T) {
	s := newTestServer(t, 3)
	ctx := context.Background()
	if _, err := s.store.InitFleet(ctx, "champion", "challenger"); err != nil {
		t.Fatal(err)
	}
	order, err := s.store.Enqueue(ctx, "literal-case", artwork.Order{Customer: "Brian Smith", Product: artwork.DieCut, WidthIn: 3, HeightIn: 3}, agent.ReadyForProof)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.store.SaveRun(ctx, store.AgentRun{OrderID: order.ID, Role: "champion", Model: "champion", Expected: agent.ReadyForProof, Correct: true, NotesPass: true, CostUSD: 0.004, Latency: 2 * time.Second, Result: agent.Result{Decision: agent.ReadyForProof, CustomerNote: "Thanks Brian. Your proof is on the way."}, Trace: json.RawMessage(`[{"tool":"measure_artwork","args":{},"result":"300 dpi","overridden":false},{"tool":"route_order","args":{},"result":"ready_for_proof","overridden":true}]`), OverriddenByRules: true})
	if err != nil {
		t.Fatal(err)
	}
	resp := s.do(t, "198.51.100.1", `{ desk { champion challenger spentToday orders { caseId expected runs { role correct costUsd latencyMs overriddenByRules trace { tool args result overridden } } } scoreboard { model n accuracy notesPassRate costPerOrder p50Ms } events { kind model numbers } } }`, nil)
	if len(resp.Errors) > 0 {
		t.Fatalf("errors=%+v", resp.Errors)
	}
	var data struct {
		Desk struct {
			Champion, Challenger string
			SpentToday           float64
			Orders               []struct {
				CaseID, Expected string
				Runs             []struct {
					Role                       string
					Correct, OverriddenByRules bool
					CostUSD                    float64
					LatencyMs                  int
					Trace                      []struct {
						Tool, Args, Result string
						Overridden         bool
					}
				}
			}
			Scoreboard []struct {
				Model                                 string
				N                                     int
				Accuracy, NotesPassRate, CostPerOrder float64
				P50Ms                                 int
			}
			Events []struct{ Kind, Model, Numbers string }
		}
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatal(err)
	}
	d := data.Desk
	if d.Champion != "champion" || d.Challenger != "challenger" || d.SpentToday != 0.004 || len(d.Orders) != 1 || len(d.Scoreboard) != 1 || len(d.Events) != 2 {
		t.Fatalf("data=%s", resp.Data)
	}
	r := d.Orders[0].Runs[0]
	score := d.Scoreboard[0]
	if !r.Correct || !r.OverriddenByRules || r.CostUSD != 0.004 || r.LatencyMs != 2000 || len(r.Trace) != 2 || r.Trace[0].Tool != "measure_artwork" || r.Trace[1].Result != "ready_for_proof" || !r.Trace[1].Overridden || score.N != 1 || score.Accuracy != 1 || score.NotesPassRate != 1 || score.CostPerOrder != 0.004 || score.P50Ms != 2000 {
		t.Fatalf("data=%s", resp.Data)
	}
	if s.llm.callCount() != 0 {
		t.Fatal("read query called external LLM")
	}
}

func TestDailyCapStopsManualNoteRepair(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": `{"decision":"ready_for_proof","reasons":[],"designer_note":"Proof as is.","customer_note":"Sorry!","requested":{"width_in":null,"height_in":null,"shape":"none","material":""}}`}}}, "usage": map[string]any{"cost": 0.002}})
	}))
	defer upstream.Close()
	cases, art, err := proofqc.Cases()
	if err != nil {
		t.Fatal(err)
	}
	st := storetest.Open(t)
	h, err := New(Config{Store: st, LLM: &llm.Client{BaseURL: upstream.URL}, Cases: cases[:1], Artwork: art, DailyCapUSD: 0.001})
	if err != nil {
		t.Fatal(err)
	}
	s := testServer{Handler: h, store: st}
	resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": sampleInput()})
	if len(resp.Errors) > 0 {
		t.Fatalf("errors=%+v", resp.Errors)
	}
	spent, err := st.SpentSince(context.Background(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || spent != 0.002 {
		t.Fatalf("calls=%d spend=%v; repair must stop at daily cap", calls.Load(), spent)
	}
}

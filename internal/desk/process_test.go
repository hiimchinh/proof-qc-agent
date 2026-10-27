package desk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func toolService(t *testing.T, capUSD float64, truncated bool) (*Service, *store.Store, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req struct{ Messages []struct{ Role string } }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		name, args := "measure_artwork", `{}`
		if len(req.Messages) > 0 && req.Messages[len(req.Messages)-1].Role == "tool" {
			name, args = "route_order", `{"decision":"ready_for_proof","reasons":["300 dpi"],"designer_note":"Proof as is.","customer_note":"Thanks Brian. Your proof is on the way.","requested":{"width_in":null,"height_in":null,"shape":"none","material":""}}`
		}
		reason := "tool_calls"
		if truncated {
			reason = "length"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": reason, "message": map[string]any{"tool_calls": []any{map[string]any{"id": name, "type": "function", "function": map[string]string{"name": name, "arguments": args}}}}}}, "usage": map[string]any{"cost": 0.001, "prompt_tokens": 10, "completion_tokens": 20}})
	}))
	t.Cleanup(upstream.Close)
	st := storetest.Open(t)
	cases, art, err := proofqc.Cases()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := New(Config{Store: st, LLM: &llm.Client{BaseURL: upstream.URL}, Cases: cases[:1], Artwork: art, DailyCapUSD: capUSD})
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, &calls
}

func TestTickRunsChampionAndShadowWithStoredEvidence(t *testing.T) {
	svc, st, calls := toolService(t, 1, false)
	out, err := svc.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	orders, err := st.RecentOrders(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if out.Processed != 1 || out.Capped || calls.Load() != 4 || len(orders) != 1 || orders[0].Status != "done" || len(orders[0].Runs) != 2 {
		t.Fatalf("tick=%+v calls=%d orders=%+v", out, calls.Load(), orders)
	}
	for i, r := range orders[0].Runs {
		var trace []struct{ Tool string }
		if err := json.Unmarshal(r.Trace, &trace); err != nil {
			t.Fatal(err)
		}
		role := "champion"
		if i == 1 {
			role = "challenger"
		}
		if r.Role != role || !r.Correct || !r.NotesPass || r.CostUSD != 0.002 || len(trace) != 2 || trace[0].Tool != "measure_artwork" || trace[1].Tool != "route_order" {
			t.Fatalf("run=%+v trace=%+v", r, trace)
		}
	}
	spent, err := st.SpentSince(context.Background(), midnight(orders[0].CreatedAt))
	if err != nil || spent != 0.004 {
		t.Fatalf("spent=%v err=%v", spent, err)
	}
}

func TestTickResumesShadowWithoutRepeatingChampion(t *testing.T) {
	svc, st, calls := toolService(t, 0.002, false)
	out, err := svc.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	orders, _ := st.RecentOrders(context.Background(), 10)
	if !out.Capped || calls.Load() != 2 || len(orders) != 1 || orders[0].Status != "queued" || len(orders[0].Runs) != 1 {
		t.Fatalf("out=%+v calls=%d orders=%+v", out, calls.Load(), orders)
	}
	championID := orders[0].Runs[0].ID
	cfg := svc.cfg
	cfg.DailyCapUSD = 1
	restarted, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, err = restarted.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	orders, _ = st.RecentOrders(context.Background(), 10)
	if out.Processed != 2 || calls.Load() != 8 || len(orders) != 2 || orders[1].Status != "done" || len(orders[1].Runs) != 2 || orders[1].Runs[0].ID != championID {
		t.Fatalf("out=%+v calls=%d orders=%+v", out, calls.Load(), orders)
	}
}

func TestTickRecordsFailedCallsAndStopsInsideToolBudget(t *testing.T) {
	t.Run("billed failure", func(t *testing.T) {
		svc, st, calls := toolService(t, 1, true)
		out, err := svc.Tick(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		orders, _ := st.RecentOrders(context.Background(), 10)
		if out.Processed != 1 || calls.Load() != 2 || orders[0].Status != "failed" {
			t.Fatalf("out=%+v orders=%+v", out, orders)
		}
		for _, run := range orders[0].Runs {
			if run.Error == "" || run.Correct || run.NotesPass || run.CostUSD != 0.001 {
				t.Fatalf("run=%+v", run)
			}
		}
	})
	t.Run("mid-loop cap", func(t *testing.T) {
		svc, st, calls := toolService(t, 0.0005, false)
		out, err := svc.Tick(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		orders, _ := st.RecentOrders(context.Background(), 10)
		if !out.Capped || calls.Load() != 1 || len(orders) != 1 || orders[0].Status != "queued" || len(orders[0].Runs) != 1 || orders[0].Runs[0].CostUSD != 0.001 {
			t.Fatalf("out=%+v calls=%d orders=%+v", out, calls.Load(), orders)
		}
	})
}

package desk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/internal/store/storetest"
)

func TestOrderStatusFollowsChampionDespiteShadowFailure(t *testing.T) {
	for _, failedRole := range []string{"challenger", "champion"} {
		for _, resumed := range []bool{false, true} {
			name := failedRole + "/fresh"
			if resumed {
				name = failedRole + "/resumed"
			}
			t.Run(name, func(t *testing.T) {
				ctx := t.Context()
				cases, art, err := proofqc.Cases()
				if err != nil {
					t.Fatal(err)
				}
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req struct {
						Model    string
						Messages []struct{ Role string }
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
						return
					}
					failingModel := "google/gemma-3-27b-it"
					if failedRole == "champion" {
						failingModel = "anthropic/claude-haiku-5.5"
					}
					if req.Model == failingModel {
						w.WriteHeader(http.StatusTooManyRequests)
						_, _ = w.Write([]byte(`{"error":{"message":"Provider returned error","metadata":{"provider_name":"DeepInfra","raw":"temporarily rate-limited upstream"}},"usage":{"cost":0.001}}`))
						return
					}
					tool, args := "measure_artwork", `{}`
					if req.Messages[len(req.Messages)-1].Role == "tool" {
						tool, args = "route_order", `{"decision":"ask_customer","reasons":["Contradictory background instructions"],"designer_note":"Wait for clarification.","customer_note":"Thanks Fatima. Should we remove the background or keep the white box?","requested":{"width_in":null,"height_in":null,"shape":"none","material":""}}`
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "tool_calls", "message": map[string]any{"tool_calls": []any{map[string]any{"id": tool, "type": "function", "function": map[string]string{"name": tool, "arguments": args}}}}}}, "usage": map[string]any{"cost": 0.001}})
				}))
				defer upstream.Close()
				st := storetest.Open(t)
				var fixture eval.Case
				for _, c := range cases {
					if c.ID == "gen-026-contradict" {
						fixture = c
					}
				}
				if fixture.ID == "" {
					t.Fatal("regression fixture missing")
				}
				svc, err := New(Config{Store: st, LLM: &llm.Client{BaseURL: upstream.URL}, Cases: []eval.Case{fixture}, Artwork: art, DailyCapUSD: 1, Champion: "anthropic/claude-haiku-5.5", Challenger: "google/gemma-3-27b-it"})
				if err != nil {
					t.Fatal(err)
				}
				if resumed {
					o, err := st.Enqueue(ctx, fixture.ID, fixture.Order, agent.AskCustomer)
					if err != nil {
						t.Fatal(err)
					}
					run := store.AgentRun{OrderID: o.ID, Role: "champion", Model: svc.cfg.Champion, Expected: agent.AskCustomer, Correct: true, NotesPass: true, Result: agent.Result{Decision: agent.AskCustomer}}
					if failedRole == "champion" {
						run.Error = "provider unavailable"
						run.Correct = false
						run.NotesPass = false
					}
					if _, err := st.SaveRun(ctx, run); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := svc.Tick(ctx); err != nil {
					t.Fatal(err)
				}
				orders, err := st.RecentOrders(ctx, 10)
				if err != nil {
					t.Fatal(err)
				}
				want := "done"
				if failedRole == "champion" {
					want = "failed"
				}
				for _, o := range orders {
					if o.Status != want {
						t.Errorf("case=%s status=%s; want %s", o.CaseID, o.Status, want)
					}
					if len(o.Runs) != 2 {
						t.Fatalf("runs=%+v", o.Runs)
					}
					for _, run := range o.Runs {
						if run.Role == failedRole {
							if run.Error == "" || run.Correct || run.NotesPass {
								t.Fatalf("failure lost: %+v", run)
							}
							if run.Role == "challenger" && !strings.Contains(run.Error, "rate-limited upstream") {
								t.Fatal("upstream cause lost")
							}
						} else if run.Error != "" || run.Result.Decision != agent.AskCustomer || !run.Correct {
							t.Fatalf("decision lost: %+v", run)
						}
					}
				}
				runs, err := st.Runs(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for _, score := range fleet.Scoreboard(runs) {
					if (failedRole == "challenger" && score.Model == svc.cfg.Challenger || failedRole == "champion" && score.Model == svc.cfg.Champion) && (score.Accuracy != 0 || score.NotesPassRate != 0 || score.N != len(orders)) {
						t.Fatalf("failed calls excluded: %+v", score)
					}
				}
			})
		}
	}
}

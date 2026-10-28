package fleet_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

const creditError = "x-ai/grok-4.7: This request would exceed your available credits given your current in-flight requests"

func TestCreditErrorsDoNotRetireOrScoreModels(t *testing.T) {
	testProviderErrorsDoNotRetireOrScoreModels(t, creditError)
}

func TestRateLimitsDoNotRetireOrScoreModels(t *testing.T) {
	for _, text := range []string{"status 429 rate limited", "HTTP 429", "Rate limit exceeded: free-models-per-day", "temporarily rate-limited upstream"} {
		t.Run(text, func(t *testing.T) { testProviderErrorsDoNotRetireOrScoreModels(t, text) })
	}
}

func testProviderErrorsDoNotRetireOrScoreModels(t *testing.T, providerError string) {
	state := store.FleetState{Champion: "a", Challenger: "b"}
	runs := append(modelRuns("a", 30, 30, 0.001), modelRuns("b", 30, 30, 0.002)...)

	for range 6 {
		for _, model := range []string{"a", "b"} {
			runs = append(runs, store.AgentRun{Model: model, Error: providerError, CostUSD: 1})
		}
	}
	got, events := fleet.Evaluate(state, runs, eval.Scorecard{Cases: 30, Correct: 15})
	if got != state || len(events) != 0 {
		t.Errorf("state=%+v events=%+v", got, events)
	}
	scores := fleet.Scoreboard(runs)
	if len(scores) != 2 {
		t.Fatalf("scores=%+v", scores)
	}
	for i, wantCost := range []float64{0.001, 0.002} {
		got := scores[i]
		if got.N != 30 || got.Accuracy != 1 || got.NotesPassRate != 1 || math.Abs(got.CostPerOrder-wantCost) > 1e-12 || got.P50 != time.Second {
			t.Errorf("score=%+v want 30 perfect cases, cost=%v, p50=1s", got, wantCost)
		}
	}
	if scores := fleet.Scoreboard([]store.AgentRun{{Model: "unanswered", Error: providerError}}); len(scores) != 0 {
		t.Errorf("credit-only scores=%+v", scores)
	}
}

func TestCreditErrorsDoNotResetModelFailureStreak(t *testing.T) {
	for _, failure := range []string{`expected tool_calls, got finish_reason "stop"`, "status 429 rate limited"} {
		t.Run(failure, func(t *testing.T) {
			runs := modelRuns("a", 1, 1, 0.001)
			for range 5 {
				runs = append(runs, store.AgentRun{Model: "a", Error: failure}, store.AgentRun{Model: "a", Error: creditError}, store.AgentRun{Model: "a", Error: "HTTP 429"})
			}
			got, events := fleet.Evaluate(store.FleetState{Champion: "a"}, runs, eval.Scorecard{})
			if failure == "status 429 rate limited" {
				if got.Champion != "a" || len(events) != 0 {
					t.Fatalf("state=%+v events=%+v", got, events)
				}
				scores := fleet.Scoreboard(runs)
				if len(scores) != 1 || scores[0].N != 1 || scores[0].Accuracy != 1 || scores[0].NotesPassRate != 1 {
					t.Fatalf("scores=%+v", scores)
				}
				return
			}
			if got.Champion != "" || len(events) != 1 || events[0].Kind != "retired" || !strings.HasPrefix(events[0].Reason, "5 consecutive failed calls;") {
				t.Fatalf("state=%+v events=%+v", got, events)
			}
			scores := fleet.Scoreboard(runs)
			if len(scores) != 1 || scores[0].N != 6 || scores[0].Accuracy != 1.0/6 || scores[0].NotesPassRate != 1.0/6 {
				t.Fatalf("scores=%+v", scores)
			}
		})
	}
}

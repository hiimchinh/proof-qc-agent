package fleet_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func modelRuns(model string, n, correct int, cost float64) []store.AgentRun {
	var runs []store.AgentRun
	for i := range n {
		runs = append(runs, store.AgentRun{Model: model, Correct: i < correct, NotesPass: true, Expected: agent.ReadyForProof, Result: agent.Result{Decision: agent.ReadyForProof}, CostUSD: cost, Latency: time.Second})
	}
	return runs
}
func TestScoreboardCountsFailedCallsAsWrongAndUnclean(t *testing.T) {
	runs := modelRuns("b", 2, 2, 0.003)
	runs[0].Latency = 3 * time.Second
	runs = append(runs, store.AgentRun{Model: "b", Correct: true, NotesPass: true, Error: "timeout", CostUSD: 0.006, Latency: time.Minute})
	runs = append(runs, modelRuns("a", 1, 1, 0.001)...)
	scores := fleet.Scoreboard(runs)
	if len(scores) != 2 || scores[0].Model != "a" {
		t.Fatalf("scores %+v", scores)
	}
	b := scores[1]
	if b.N != 3 || b.Accuracy != 2.0/3 || b.NotesPassRate != 2.0/3 || b.CostPerOrder != 0.004 || b.P50 != time.Second {
		t.Fatalf("failed calls score %+v", b)
	}
}

func TestPolicyWaitsForMinimumAndUsesMeasuredBaseline(t *testing.T) {
	initial := store.FleetState{Champion: "a", Challenger: "b"}
	baseline := eval.Scorecard{Cases: 30, Correct: 15}
	runs := append(modelRuns("a", 29, 1, 0.002), modelRuns("b", 29, 29, 0.001)...)
	got, events := fleet.Evaluate(initial, runs, baseline)
	if got != initial || len(events) != 0 {
		t.Fatalf("premature %+v %+v", got, events)
	}
	runs = append(modelRuns("a", 30, 30, 0.002), modelRuns("b", 30, 30, 0.001)...)
	// Perfect rules on this stream means neither model clears the +10 point bar.
	got, events = fleet.Evaluate(initial, runs, eval.Scorecard{Cases: 30, Correct: 30})
	if got.Champion != "" || got.Challenger != "" || len(events) != 2 || events[0].Kind != "retired" {
		t.Fatalf("baseline ignored %+v %+v", got, events)
	}
}

func TestPromotionPrefersAccuracyThenCost(t *testing.T) {
	for _, tc := range []struct {
		name               string
		correctA, correctB int
		costA, costB       float64
		promote            bool
	}{
		{"higher accuracy", 28, 30, 0.001, 0.004, true},
		{"equal accuracy cheaper", 30, 30, 0.004, 0.001, true},
		{"equal accuracy dearer", 30, 30, 0.001, 0.004, false},
		{"lower accuracy cheaper", 30, 28, 0.004, 0.001, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := store.FleetState{Champion: "a", Challenger: "b"}
			runs := append(modelRuns("a", 30, tc.correctA, tc.costA), modelRuns("b", 30, tc.correctB, tc.costB)...)
			got, events := fleet.Evaluate(state, runs, eval.Scorecard{Cases: 30, Correct: 15})
			if tc.promote {
				if got.Champion != "b" || got.Challenger != "" || len(events) != 1 || events[0].Kind != "promoted" {
					t.Fatalf("promotion %+v %+v", got, events)
				}
			} else if got != state || len(events) != 0 {
				t.Fatalf("unexpected promotion %+v %+v", got, events)
			}
		})
	}
}

func TestRetirementLeavesSafeChampionState(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		correctA, correctB, nB int
		want                   store.FleetState
		kinds                  []string
	}{
		{"champion fails successor passes", 10, 30, 30, store.FleetState{Champion: "b"}, []string{"retired", "promoted"}},
		{"both fail", 10, 10, 30, store.FleetState{}, []string{"retired", "retired"}},
		{"challenger fails", 30, 10, 30, store.FleetState{Champion: "a"}, []string{"retired"}},
		{"successor too young", 10, 29, 29, store.FleetState{Challenger: "b"}, []string{"retired"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runs := append(modelRuns("a", 30, tc.correctA, 0.001), modelRuns("b", tc.nB, tc.correctB, 0.001)...)
			state, events := fleet.Evaluate(store.FleetState{Champion: "a", Challenger: "b"}, runs, eval.Scorecard{Cases: 30, Correct: 15})
			if state != tc.want || len(events) != len(tc.kinds) {
				t.Fatalf("state %+v events %+v", state, events)
			}
			for i, kind := range tc.kinds {
				if events[i].Kind != kind || len(events[i].Numbers) == 0 {
					t.Fatalf("event %+v want %s", events[i], kind)
				}
			}
		})
	}
}

func TestErrorsAndNotesFailPolicy(t *testing.T) {
	for _, field := range []string{"error", "notes"} {
		t.Run(field, func(t *testing.T) {
			runs := modelRuns("a", 30, 30, 0.001)
			for i := range 2 {
				if field == "error" {
					runs[i].Error = "timeout"
				} else {
					runs[i].NotesPass = false
				}
			}
			state, events := fleet.Evaluate(store.FleetState{Champion: "a"}, runs, eval.Scorecard{Cases: 30, Correct: 15})
			if state.Champion != "" || len(events) != 1 || events[0].Kind != "retired" {
				t.Fatalf("failure ignored %+v %+v", state, events)
			}
		})
	}
}

func TestEventNumbersCarryComparisonAndPolicyEvidence(t *testing.T) {
	runs := append(modelRuns("a", 30, 27, 0.004), modelRuns("b", 30, 30, 0.001)...)
	for _, tc := range []struct {
		name            string
		baselineCorrect int
		kind            string
	}{
		{"promotion", 15, "promoted"}, {"retirement", 30, "retired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, events := fleet.Evaluate(store.FleetState{Champion: "a", Challenger: "b"}, runs, eval.Scorecard{Cases: 30, Correct: tc.baselineCorrect})
			if len(events) == 0 || events[0].Kind != tc.kind {
				t.Fatalf("events %+v", events)
			}
			var numbers struct {
				Champion   fleet.Score `json:"champion"`
				Challenger fleet.Score `json:"challenger"`
				Baseline   struct {
					N        int     `json:"n"`
					Accuracy float64 `json:"accuracy"`
				} `json:"baseline"`
				Policy struct {
					MinSamples  int           `json:"min_samples"`
					MinAccuracy float64       `json:"min_accuracy"`
					MinGain     float64       `json:"min_gain_over_baseline"`
					MinNotes    float64       `json:"min_notes_pass"`
					MaxCost     float64       `json:"max_cost_per_order"`
					MaxP50      time.Duration `json:"max_p50_ns"`
				} `json:"policy"`
			}
			if err := json.Unmarshal(events[0].Numbers, &numbers); err != nil {
				t.Fatal(err)
			}
			if numbers.Champion.Model != "a" || numbers.Champion.N != 30 || numbers.Champion.Accuracy != 0.9 || math.Abs(numbers.Champion.CostPerOrder-0.004) > 1e-12 || numbers.Challenger.Model != "b" || numbers.Challenger.Accuracy != 1 || math.Abs(numbers.Challenger.CostPerOrder-0.001) > 1e-12 {
				t.Fatalf("comparison missing %+v", numbers)
			}
			wantBaseline := 0.5
			if tc.name == "retirement" {
				wantBaseline = 1
			}
			if numbers.Baseline.N != 30 || numbers.Baseline.Accuracy != wantBaseline {
				t.Fatalf("baseline missing %+v", numbers.Baseline)
			}
			p := numbers.Policy
			if p.MinSamples != 30 || p.MinAccuracy != 0.9 || p.MinGain != 0.1 || p.MinNotes != 0.95 || p.MaxCost != 0.01 || p.MaxP50 != 30*time.Second {
				t.Fatalf("policy missing %+v", p)
			}
		})
	}
}

package fleet_test

import (
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestEvaluateRetiresConsecutiveFailuresBeforeMinimumSamples(t *testing.T) {
	// Literal call histories are oldest first, interleaving both models.
	failures := []store.AgentRun{
		{Model: "bad", Error: "timeout"},
		{Model: "good", Correct: true, NotesPass: true},
		{Model: "bad", Error: "timeout"},
		{Model: "bad", Error: "timeout"},
		{Model: "good", Correct: true, NotesPass: true},
		{Model: "bad", Error: "timeout"},
		{Model: "bad", Error: "expected tool_calls, got finish_reason stop"},
	}
	for _, role := range []string{"champion", "challenger"} {
		t.Run(role, func(t *testing.T) {
			initial, want := store.FleetState{Champion: "good", Challenger: "bad"}, store.FleetState{Champion: "good"}
			if role == "champion" {
				initial = store.FleetState{Champion: "bad", Challenger: "good"}
				want = store.FleetState{Challenger: "good"}
			}
			for _, tc := range []struct {
				name   string
				runs   []store.AgentRun
				retire bool
			}{
				{"four failures", failures[:6], false},
				{"fifth failure", failures, true},
				{"six failures", append(append([]store.AgentRun{}, failures...), store.AgentRun{Model: "bad", Error: "timeout"}), true},
				{"successful call resets streak", append(append([]store.AgentRun{}, failures...), store.AgentRun{Model: "bad"}), false},
				{"answered wrong resets streak", append(append([]store.AgentRun{}, failures[:6]...), store.AgentRun{Model: "bad"}, store.AgentRun{Model: "bad", Error: "timeout"}), false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					got, events := fleet.Evaluate(initial, tc.runs, eval.Scorecard{Cases: 30, Correct: 15})
					if !tc.retire {
						if got != initial || len(events) != 0 {
							t.Fatalf("unexpected retirement: %+v %+v", got, events)
						}
						return
					}
					if got != want || len(events) != 1 || events[0].Kind != "retired" || events[0].Model != "bad" {
						t.Fatalf("unreliable model kept: %+v %+v", got, events)
					}
					reason := "5 consecutive failed calls; last: expected tool_calls, got finish_reason stop"
					if tc.name == "six failures" {
						reason = "6 consecutive failed calls; last: timeout"
					}
					if events[0].Reason != reason {
						t.Fatalf("reason=%q; want %q", events[0].Reason, reason)
					}
					if !strings.Contains(string(events[0].Numbers), `"max_consecutive_failures":5`) {
						t.Fatalf("reliability threshold missing: %s", events[0].Numbers)
					}
				})
			}
		})
	}
}

func TestUnreliableChampionPromotesOnlyQualifiedChallenger(t *testing.T) {
	failures := []store.AgentRun{{Model: "a", Error: "timeout"}, {Model: "a", Error: "timeout"}, {Model: "a", Error: "timeout"}, {Model: "a", Error: "timeout"}, {Model: "a", Error: "timeout"}}
	for _, tc := range []struct {
		name      string
		successor []store.AgentRun
		want      store.FleetState
		events    int
	}{
		{"qualified", modelRuns("b", 30, 30, 0.001), store.FleetState{Champion: "b"}, 2},
		{"too young", modelRuns("b", 29, 29, 0.001), store.FleetState{Challenger: "b"}, 1},
		{"no successor", nil, store.FleetState{}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := store.FleetState{Champion: "a"}
			if len(tc.successor) > 0 {
				initial.Challenger = "b"
			}
			got, events := fleet.Evaluate(initial, append(append([]store.AgentRun{}, failures...), tc.successor...), eval.Scorecard{Cases: 30, Correct: 15})
			if got != tc.want || len(events) != tc.events {
				t.Fatalf("state=%+v events=%+v", got, events)
			}
			if tc.events == 2 && (events[1].Kind != "promoted" || !strings.Contains(events[1].Reason, "retired a")) {
				t.Fatalf("promotion=%+v", events[1])
			}
		})
	}
}

func TestRetirementReasonBoundsAndRedactsLastError(t *testing.T) {
	runs := []store.AgentRun{{Model: "bad", Error: "old"}, {Model: "bad", Error: "old"}, {Model: "bad", Error: "old"}, {Model: "bad", Error: "old"},
		{Model: "bad", Error: `failure Authorization: Bearer bearer-secret api_key=key-secret password="a,b secret-tail" ` + strings.Repeat("診", 500)},
	}
	_, events := fleet.Evaluate(store.FleetState{Challenger: "bad"}, runs, eval.Scorecard{})
	if len(events) != 1 {
		t.Fatalf("events=%+v", events)
	}
	reason := events[0].Reason
	if len([]rune(reason)) > 200 || !strings.HasSuffix(reason, "...") {
		t.Fatalf("unbounded cause: %q", reason)
	}
	for _, secret := range []string{"bearer-secret", "key-secret", "secret-tail", "\n"} {
		if strings.Contains(reason, secret) {
			t.Fatal("retirement leaked a secret or control character")
		}
	}
}

package fleet_test

import (
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

const googleRateLimit = "google/gemma-4-31b-it:free: Provider returned error; provider: Google AI Studio; upstream: google/gemma-4-31b-it:free is temporarily rate-limited upstream"

func rateLimited(model string, n int) []store.AgentRun {
	var runs []store.AgentRun
	for range n {
		runs = append(runs, store.AgentRun{Model: model, Error: googleRateLimit})
	}
	return runs
}

func TestEvaluateSkipsChallengerThatTheProviderKeepsRateLimiting(t *testing.T) {
	initial := store.FleetState{Champion: "a", Challenger: "b"}
	base := modelRuns("a", 30, 30, 0.001)
	for _, tc := range []struct {
		name string
		runs []store.AgentRun
		skip bool
	}{
		{"eleven rate limits", append(append([]store.AgentRun{}, base...), rateLimited("b", 11)...), false},
		{"twelve rate limits", append(append([]store.AgentRun{}, base...), rateLimited("b", 12)...), true},
		{"answered call resets the streak", append(append(append(append([]store.AgentRun{}, base...), rateLimited("b", 11)...), modelRuns("b", 1, 1, 0.002)...), rateLimited("b", 1)...), false},
		{"model failure resets the streak", append(append(append(append([]store.AgentRun{}, base...), rateLimited("b", 11)...), store.AgentRun{Model: "b", Error: "timeout"}), rateLimited("b", 1)...), false},
		{"credit errors do not reset the streak", append(append(append(append([]store.AgentRun{}, base...), rateLimited("b", 6)...), store.AgentRun{Model: "b", Error: creditError}), rateLimited("b", 6)...), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, events := fleet.Evaluate(initial, tc.runs, eval.Scorecard{Cases: 30, Correct: 15})
			if !tc.skip {
				if got != initial || len(events) != 0 {
					t.Fatalf("state=%+v events=%+v", got, events)
				}
				return
			}
			if got != (store.FleetState{Champion: "a"}) || len(events) != 1 {
				t.Fatalf("state=%+v events=%+v", got, events)
			}
			e := events[0]
			if e.Kind != "skipped" || e.Model != "b" || e.Reason != "12 consecutive rate-limited calls; last: provider Google AI Studio rate-limited upstream" {
				t.Fatalf("event=%+v", e)
			}
			if !strings.Contains(string(e.Numbers), `"max_consecutive_rate_limits":12`) {
				t.Fatalf("rate-limit threshold missing: %s", e.Numbers)
			}
		})
	}
}

func TestEvaluateKeepsChampionThatTheProviderRateLimits(t *testing.T) {
	initial := store.FleetState{Champion: "a", Challenger: "b"}
	runs := append(modelRuns("b", 1, 1, 0.002), rateLimited("a", 20)...)
	got, events := fleet.Evaluate(initial, runs, eval.Scorecard{Cases: 30, Correct: 15})
	if got != initial || len(events) != 0 {
		t.Fatalf("state=%+v events=%+v", got, events)
	}
}

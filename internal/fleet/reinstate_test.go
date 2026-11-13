package fleet_test

import (
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

// openRouterCreditError is the error OpenRouter returned on 2026-11-06 when the
// account ran out of credits; the fleet retired its champion over five of them.
const openRouterCreditError = "google/gemma-4-31b-it: This request requires more credits, or fewer max_tokens. You requested up to 2000 tokens, but can only afford 1380. To increase, visit ht..."

func TestOpenRouterCreditErrorDoesNotRetireOrScoreModels(t *testing.T) {
	testProviderErrorsDoNotRetireOrScoreModels(t, openRouterCreditError)
}

// creditRetiredChampion replays the incident: a passing champion, then five
// credit errors that an older build counted as model failures.
func creditRetiredChampion(model string) ([]store.AgentRun, store.FleetEvent) {
	runs := modelRuns(model, 30, 30, 0.001)
	for range 5 {
		runs = append(runs, store.AgentRun{Model: model, Error: openRouterCreditError})
	}
	for i := range runs {
		runs[i].Role = "champion"
	}
	retired := store.FleetEvent{Kind: "retired", Model: model, Reason: "5 consecutive failed calls; last: " + openRouterCreditError}
	return runs, retired
}

func TestReinstateReturnsAModelRetiredForProviderErrorsToItsSeat(t *testing.T) {
	runs, retired := creditRetiredChampion("gemma")
	got, events := fleet.Reinstate(store.FleetState{}, runs, []store.FleetEvent{retired})
	if got != (store.FleetState{Champion: "gemma"}) || len(events) != 1 || events[0].Kind != "reinstated" || events[0].Model != "gemma" || events[0].Reason == "" {
		t.Fatalf("state=%+v events=%+v", got, events)
	}
	// The reinstated champion passes the policy, so the next evaluation keeps it.
	kept, events := fleet.Evaluate(got, runs, eval.Scorecard{Cases: 30, Correct: 15})
	if kept != got || len(events) != 0 {
		t.Fatalf("state=%+v events=%+v", kept, events)
	}
}

func TestReinstateKeepsDeservedRetirements(t *testing.T) {
	creditRuns, creditRetired := creditRetiredChampion("gemma")
	toolRuns := modelRuns("gemma-3", 1, 1, 0.001)
	for range 5 {
		toolRuns = append(toolRuns, store.AgentRun{Model: "gemma-3", Role: "challenger", Error: `expected tool_calls, got finish_reason "stop"`})
	}
	toolRetired := store.FleetEvent{Kind: "retired", Model: "gemma-3", Reason: `5 consecutive failed calls; last: expected tool_calls, got finish_reason "stop"`}
	policyRuns := modelRuns("terra", 30, 24, 0.001)
	policyRetired := store.FleetEvent{Kind: "retired", Model: "terra", Reason: "80% accuracy (need 90%)"}
	for name, tc := range map[string]struct {
		state  store.FleetState
		runs   []store.AgentRun
		latest []store.FleetEvent
	}{
		"real model failures":           {store.FleetState{}, toolRuns, []store.FleetEvent{toolRetired}},
		"policy retirement":             {store.FleetState{}, policyRuns, []store.FleetEvent{policyRetired}},
		"seat already filled":           {store.FleetState{Champion: "haiku"}, creditRuns, []store.FleetEvent{creditRetired}},
		"latest event is not a retire":  {store.FleetState{}, creditRuns, []store.FleetEvent{{Kind: "added", Model: "gemma"}}},
		"model without runs in history": {store.FleetState{}, nil, []store.FleetEvent{creditRetired}},
	} {
		t.Run(name, func(t *testing.T) {
			got, events := fleet.Reinstate(tc.state, tc.runs, tc.latest)
			if got != tc.state || len(events) != 0 {
				t.Fatalf("state=%+v events=%+v", got, events)
			}
		})
	}
}

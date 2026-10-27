package fleet_test

import (
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestPromotionReasonNamesThePreviousSeat(t *testing.T) {
	for _, tc := range []struct{ name, champion, want string }{
		{"retired champion", "openai/gpt-6.1-terra", "qualified challenger replaces retired openai/gpt-6.1-terra by accuracy then cost"},
		{"empty champion", "", "qualified challenger fills the empty champion seat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runs := append(modelRuns("openai/gpt-6.1-terra", 30, 24, 0.004), modelRuns("anthropic/claude-haiku-5.5", 30, 30, 0.001)...)
			state, events := fleet.Evaluate(store.FleetState{Champion: tc.champion, Challenger: "anthropic/claude-haiku-5.5"}, runs, eval.Scorecard{Cases: 30, Correct: 15})
			if state.Champion != "anthropic/claude-haiku-5.5" || len(events) == 0 {
				t.Fatalf("state=%+v events=%+v", state, events)
			}
			if got := events[len(events)-1].Reason; got != tc.want {
				t.Fatalf("reason = %q; want %q", got, tc.want)
			}
		})
	}
}

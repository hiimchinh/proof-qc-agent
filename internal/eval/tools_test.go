package eval

import (
	"encoding/json"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
)

func trace(tool string, override bool) agent.TraceStep {
	return agent.TraceStep{Tool: tool, Args: json.RawMessage(`{}`), Result: "ok", Overridden: override}
}
func TestToolScoreChecksMeasurementOverridesAndCalls(t *testing.T) {
	valid := Outcome{Case: Case{ID: "ok", Want: agent.AskCustomer}, Result: agent.Result{Decision: agent.AskCustomer, Attempts: 3, OverriddenByRules: true, Trace: []agent.TraceStep{trace("measure_artwork", false), trace("customer_history", false), trace("route_order", true)}}}
	invalid := Outcome{Case: Case{ID: "bad"}, Result: agent.Result{Attempts: 7, OverriddenByRules: true, Trace: []agent.TraceStep{trace("route_order", false), trace("measure_artwork", false)}}}
	skipped := Outcome{Skipped: true}
	score := ScoreTools([]Outcome{valid, invalid, skipped}, 6)
	if score.Cases != 2 || score.MeasuredBeforeRoute != 1 || score.RuleOverrides != 2 || score.ConsistentOverrides != 1 || score.BoundedSteps != 1 || score.TotalCalls != 10 || len(score.Violations) != 1 {
		t.Fatalf("score=%+v", score)
	}
}
func TestToolScoreRejectsUnknownAndFailedTraces(t *testing.T) {
	for _, out := range []Outcome{
		{Case: Case{ID: "failed"}, Err: "upstream failed", Result: agent.Result{Attempts: 1, Trace: []agent.TraceStep{trace("measure_artwork", false)}}},
		{Case: Case{ID: "unknown"}, Result: agent.Result{Attempts: 2, Trace: []agent.TraceStep{trace("measure_artwork", false), trace("delete_order", false), trace("route_order", false)}}},
	} {
		s := ScoreTools([]Outcome{out}, 6)
		if len(s.Violations) != 1 {
			t.Fatalf("score=%+v", s)
		}
	}
}

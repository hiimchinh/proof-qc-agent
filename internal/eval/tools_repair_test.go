package eval

import (
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
)

func TestToolScoreAllowsOneRepairAfterTerminalRoute(t *testing.T) {
	out := Outcome{Case: Case{ID: "repaired"}, Result: agent.Result{Attempts: 3, Trace: []agent.TraceStep{trace("measure_artwork", false), trace("route_order", false), trace("repair_note", false)}}}
	s := ScoreTools([]Outcome{out}, 6)
	if s.MeasuredBeforeRoute != 1 || len(s.Violations) != 0 {
		t.Fatalf("score=%+v", s)
	}
	out.Result.Trace = append(out.Result.Trace, trace("repair_note", false))
	s = ScoreTools([]Outcome{out}, 6)
	if len(s.Violations) != 1 {
		t.Fatalf("duplicate repair accepted: %+v", s)
	}
}

package eval

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/notelint"
)

func outcome(want, got agent.Decision, cost float64, lat time.Duration, lint bool, err error) Outcome {
	o := Outcome{Case: Case{ID: "c", Want: want}, Result: agent.Result{Decision: got, CostUSD: cost, Latency: lat}}
	if lint {
		o.Result.Lint = []notelint.Violation{{Rule: notelint.Negative}}
	}
	if err != nil {
		o.Err = err.Error()
	}
	return o
}

func TestScoreCountsErrorsAsMisses(t *testing.T) {
	s := Score("m", []Outcome{
		outcome(agent.ReadyForProof, agent.ReadyForProof, 0.002, 2*time.Second, false, nil),
		outcome(agent.AskCustomer, agent.ReadyForProof, 0.002, 4*time.Second, true, nil),
		outcome(agent.AskCustomer, "", 0, 0, false, errors.New("timeout")),
		outcome(agent.RecreateArtwork, agent.RecreateArtwork, 0.002, 3*time.Second, false, nil),
	})
	if s.Cases != 4 || s.Correct != 2 || s.Errors != 1 {
		t.Fatalf("score = %+v", s)
	}
	if s.Accuracy() != 0.5 {
		t.Fatalf("accuracy = %v, want 0.5", s.Accuracy())
	}
	// Lint is judged on answered cases only: 2 of 3 notes are clean.
	if got := s.LintPassRate(); got < 0.666 || got > 0.667 {
		t.Fatalf("lint pass = %v, want 2/3", got)
	}
	if s.CostPerProof() != 0.0015 {
		t.Fatalf("cost per proof = %v, want 0.0015", s.CostPerProof())
	}
	if s.P50() != 3*time.Second {
		t.Fatalf("p50 = %v, want 3s", s.P50())
	}
	if len(s.Misses) != 2 {
		t.Fatalf("misses = %v, want 2", s.Misses)
	}
}

func card(acc float64, lint float64, cost float64, p50 time.Duration) Scorecard {
	n := 100
	s := Scorecard{Model: "m", Cases: n, Correct: int(acc * float64(n)), Answered: n, LintClean: int(lint * float64(n)), CostUSD: cost * float64(n)}
	for range n {
		s.latencies = append(s.latencies, p50)
	}
	return s
}

func TestVerdict(t *testing.T) {
	p := DefaultPolicy
	base := card(0.70, 1, 0, 0)
	cases := []struct {
		name string
		s    Scorecard
		keep bool
		why  string
	}{
		{"beats baseline cheaply", card(0.95, 1, 0.004, 5*time.Second), true, ""},
		{"no better than rules", card(0.72, 1, 0.004, 5*time.Second), false, "baseline"},
		{"breaks support principles", card(0.95, 0.80, 0.004, 5*time.Second), false, "support"},
		{"too expensive", card(0.95, 1, 0.05, 5*time.Second), false, "$"},
		{"too slow", card(0.95, 1, 0.004, 90*time.Second), false, "p50"},
	}
	for _, c := range cases {
		keep, reasons := p.Verdict(c.s, base)
		if keep != c.keep || (c.why != "" && !strings.Contains(strings.Join(reasons, " "), c.why)) {
			t.Errorf("%s: keep = %v reasons = %v", c.name, keep, reasons)
		}
	}
}

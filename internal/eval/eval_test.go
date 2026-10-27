package eval

import (
	"errors"
	"os"
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
		{"beats a weak baseline but is still wrong too often", card(0.85, 1, 0.004, 5*time.Second), false, "accuracy"},
	}
	for _, c := range cases {
		keep, reasons := p.Verdict(c.s, base)
		if keep != c.keep || (c.why != "" && !strings.Contains(strings.Join(reasons, " "), c.why)) {
			t.Errorf("%s: keep = %v reasons = %v", c.name, keep, reasons)
		}
	}
}

func TestSkippedCasesMakeTheRunIncomplete(t *testing.T) {
	outs := []Outcome{
		outcome(agent.ReadyForProof, agent.ReadyForProof, 0.004, time.Second, false, nil),
		{Case: Case{ID: "s", Want: agent.AskCustomer}, Skipped: true},
	}
	s := Score("m", outs)
	if s.Skipped != 1 || s.CostPerProof() != 0.004 {
		t.Fatalf("score = %+v, cost per proof = %v; skipped cases are not proofs", s, s.CostPerProof())
	}
	keep, reasons := DefaultPolicy.Verdict(s, card(0.5, 1, 0, 0))
	if keep || !strings.Contains(strings.Join(reasons, " "), "incomplete") {
		t.Fatalf("keep = %v reasons = %v, want incomplete", keep, reasons)
	}
}

func TestRelabelUsesTheCurrentCases(t *testing.T) {
	outs := []Outcome{{Case: Case{ID: "a", Want: agent.ReadyForProof}}}
	if err := Relabel(outs, []Case{{ID: "a", Want: agent.AskCustomer}}); err != nil || outs[0].Case.Want != agent.AskCustomer {
		t.Fatalf("err = %v want = %s", err, outs[0].Case.Want)
	}
	if err := Relabel(outs, []Case{{ID: "b"}}); err == nil {
		t.Fatal("want an error when saved outcomes and cases.json disagree")
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMergeRunsPoolsCountsAndKeepsTheRange(t *testing.T) {
	r1 := Score("m", []Outcome{
		outcome(agent.ReadyForProof, agent.ReadyForProof, 0.01, time.Second, false, nil),
		outcome(agent.AskCustomer, agent.ReadyForProof, 0.01, time.Second, false, nil),
	})
	r2 := Score("m", []Outcome{
		outcome(agent.ReadyForProof, agent.ReadyForProof, 0.01, 3*time.Second, false, nil),
		outcome(agent.AskCustomer, agent.AskCustomer, 0.01, 3*time.Second, false, nil),
	})
	m := Merge(r1, r2)
	if m.Cases != 4 || m.Correct != 3 || m.Accuracy() != 0.75 || m.CostPerProof() != 0.01 {
		t.Fatalf("merged = %+v", m)
	}
	if lo, hi := m.AccuracyRange(); lo != 0.5 || hi != 1 {
		t.Fatalf("range = %v..%v, want 0.5..1", lo, hi)
	}
	if !strings.Contains(Markdown(r1, []Scorecard{m}, DefaultPolicy), "75% (50–100)") {
		t.Fatalf("leaderboard does not show the run range:\n%s", Markdown(r1, []Scorecard{m}, DefaultPolicy))
	}
}

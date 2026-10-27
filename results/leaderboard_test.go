package results

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	proofqc "github.com/hiimchinh/proof-qc-agent"
	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
)

var twoCases = []eval.Case{
	{ID: "a", Want: agent.ReadyForProof},
	{ID: "b", Want: agent.AskCustomer},
}

// run saves one reviewer's answers to cases a and b. The saved labels are stale
// on purpose: scoring must use today's labels.
func run(t *testing.T, a, b agent.Decision) *fstest.MapFile {
	t.Helper()
	outs := []eval.Outcome{
		{Case: eval.Case{ID: "a", Want: agent.RecreateArtwork}, Result: agent.Result{Decision: a, CostUSD: 0.002, Latency: 3 * time.Second}},
		{Case: eval.Case{ID: "b", Want: agent.RecreateArtwork}, Result: agent.Result{Decision: b, CostUSD: 0.002, Latency: 5 * time.Second}},
	}
	raw, err := json.Marshal(outs)
	if err != nil {
		t.Fatal(err)
	}
	return &fstest.MapFile{Data: raw}
}

func TestLeaderboardMergesRunsPerModelAndVariant(t *testing.T) {
	ready, ask := agent.ReadyForProof, agent.AskCustomer
	fsys := fstest.MapFS{
		"v2/baseline-rules.json":               run(t, ready, ready),
		"v2/acme__model.run1.json":             run(t, ready, ask),
		"v2/acme__model.run2.json":             run(t, ready, ask),
		"v2/acme__flaky.run1.json":             run(t, ready, ask),
		"v2/acme__flaky.run2.json":             run(t, ready, ready),
		"v2/acme__model.run3.partial.json":     run(t, ask, ready),
		"v2/leaderboard.md":                    &fstest.MapFile{Data: []byte("# not a run")},
		"v2-prompt-only/acme__model.run1.json": run(t, ready, ready),
	}
	got, err := Leaderboard(fsys, twoCases)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d scores, want 4: %+v", len(got), got)
	}
	best := got[0]
	if best.Model != "acme/model" || best.Variant != RulesInCode || best.Runs != 2 || best.Cases != 4 ||
		best.Accuracy != 1 || best.CostPerProof != 0.002 || best.P50 != 3*time.Second || !best.Keep || best.Verdict != "keep" {
		t.Errorf("first = %+v, want acme/model with rules in code: 2 complete runs, 100%%, kept", best)
	}
	if flaky := got[1]; flaky.Model != "acme/flaky" || flaky.Runs != 2 || flaky.Accuracy != 0.75 ||
		flaky.AccuracyLow != 0.5 || flaky.AccuracyHigh != 1 || flaky.Keep || !strings.HasPrefix(flaky.Verdict, "retire: ") {
		t.Errorf("second = %+v, want acme/flaky at 75%% (50-100), retired", flaky)
	}
	if base := got[2]; base.Model != "baseline-rules" || base.Accuracy != 0.5 || base.Verdict != "reference" {
		t.Errorf("third = %+v, want the rules baseline at 50%% as reference", base)
	}
	if po := got[3]; po.Variant != PromptOnly || po.Keep || !strings.Contains(po.Verdict, "over the rules baseline") {
		t.Errorf("last = %+v, want prompt only retired for no gain over the baseline", po)
	}
}

func TestLeaderboardOfNoRunsIsEmpty(t *testing.T) {
	got, err := Leaderboard(fstest.MapFS{}, twoCases)
	if err != nil || len(got) != 0 {
		t.Fatalf("Leaderboard(empty) = %+v, %v; want no scores and no error", got, err)
	}
}

func TestEmbeddedRunsScoreAgainstTodaysCases(t *testing.T) {
	cases, _, err := proofqc.Cases()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Leaderboard(Embedded(), cases); err != nil {
		t.Fatal(err)
	}
}

package main

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
)

func TestToolEvalReusesIsolatedOutputsAndLimitsCases(t *testing.T) {
	dir := t.TempDir()
	art := filepath.Join(dir, "art.png")
	f, err := os.Create(art)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 30, 30))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	cases := []eval.Case{{ID: "a", File: "art.png", Order: artwork.Order{Customer: "Brian", Product: artwork.DieCut, WidthIn: 3, HeightIn: 3}, Want: agent.AskCustomer}}
	casesPath := filepath.Join(dir, "cases.json")
	if err := writeJSON(casesPath, cases); err != nil {
		t.Fatal(err)
	}
	trace := []agent.TraceStep{{Tool: "measure_artwork", Args: json.RawMessage(`{}`), Result: "empty artwork"}, {Tool: "route_order", Args: json.RawMessage(`{}`), Result: "ask_customer"}}
	outs := []eval.Outcome{{Case: cases[0], Result: agent.Result{Decision: agent.AskCustomer, Attempts: 2, Trace: trace}}}
	if err := writeJSON(filepath.Join(dir, "offline.tools.run1.json"), outs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"baseline-rules.json", "leaderboard.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("keep original"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := runEval([]string{"-cases", casesPath, "-out", dir, "-models", "offline", "-tools", "-reuse", "-limit", "1"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"baseline-rules.json", "leaderboard.md"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(b) != "keep original" {
			t.Fatalf("%s overwritten: %s %v", name, b, err)
		}
	}
	md, err := os.ReadFile(filepath.Join(dir, "leaderboard.tools.md"))
	if err != nil || !strings.Contains(string(md), "Measured before route") {
		t.Fatalf("leaderboard=%s err=%v", md, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "baseline-rules.tools.json")); err != nil {
		t.Fatal(err)
	}
}

func TestToolEvalWithoutModelsRequiresAClient(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	err := runEval([]string{"-tools", "-cases", "../../testdata/cases/cases.json", "-out", t.TempDir(), "-limit", "1", "-max-usd", "0"})
	if err == nil || !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
		t.Fatalf("tools must select a model, err=%v", err)
	}
}

func TestLimitedToolEvalPreservesFullReports(t *testing.T) {
	dir := t.TempDir()
	cases, err := eval.LoadCases("../../testdata/cases/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	outs := []eval.Outcome{{Case: cases[0], Result: agent.Result{Decision: agent.ReadyForProof}}, {Case: cases[1], Result: agent.Result{Decision: agent.ReadyForProof}}}
	if err := writeJSON(filepath.Join(dir, "offline.tools.run1.json"), outs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"baseline-rules.tools.json", "leaderboard.tools.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("full evaluation"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := runEval([]string{"-tools", "-models", "offline", "-reuse", "-cases", "../../testdata/cases/cases.json", "-out", dir, "-limit", "1"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"baseline-rules.tools.json", "leaderboard.tools.md"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(b) != "full evaluation" {
			t.Fatalf("%s overwritten: %q err=%v", name, b, err)
		}
	}
}

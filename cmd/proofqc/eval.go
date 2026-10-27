package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
)

func runEval(args []string) error {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	casesPath := fs.String("cases", "testdata/cases/cases.json", "labelled cases")
	models := fs.String("models", "", "comma-separated OpenRouter model ids")
	outDir := fs.String("out", "results", "where to write outcomes and the leaderboard")
	maxUSD := fs.Float64("max-usd", 5, "stop calling models once this much is spent")
	workers := fs.Int("workers", 4, "concurrent calls per model")
	reuse := fs.Bool("reuse", false, "score saved outcomes in -out instead of calling a model again")
	_ = fs.Parse(args)

	cases, err := eval.LoadCases(*casesPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}

	baseOuts := make([]eval.Outcome, len(cases))
	for i, c := range cases {
		img, err := loadImage(c.File)
		if err != nil {
			return err
		}
		baseOuts[i] = eval.Outcome{Case: c, Result: agent.Baseline(img, c.Order)}
	}
	baseline := eval.Score("baseline-rules", baseOuts)
	if err := writeJSON(filepath.Join(*outDir, "baseline-rules.json"), baseOuts); err != nil {
		return err
	}

	var cards []eval.Scorecard
	b := &eval.Budget{Max: *maxUSD}
	for _, model := range splitModels(*models) {
		path := filepath.Join(*outDir, strings.ReplaceAll(model, "/", "__")+".json")
		var outs []eval.Outcome
		if *reuse {
			if outs, err = readOutcomes(path); err != nil {
				return err
			}
			if err := eval.Relabel(outs, cases); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		} else {
			client, err := newClient()
			if err != nil {
				return err
			}
			a := agent.Agent{Model: model, LLM: client}
			review := func(c eval.Case) (agent.Result, error) {
				img, err := loadImage(c.File)
				if err != nil {
					return agent.Result{}, err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()
				return a.Review(ctx, img, c.Order)
			}
			if outs, err = eval.Run(cases, review, *workers, b); err != nil {
				return err
			}
		}
		s := eval.Score(model, outs)
		cards = append(cards, s)
		fmt.Fprintf(os.Stderr, "%-36s %2d/%d  $%.4f/proof  run spend $%.2f\n", model, s.Correct, s.Cases, s.CostPerProof(), b.Spent())
		if *reuse {
			continue
		}
		if s.Skipped > 0 {
			// Never replace a complete run with a partial one.
			path = strings.TrimSuffix(path, ".json") + ".partial.json"
		}
		if err := writeJSON(path, outs); err != nil {
			return err
		}
	}

	md := eval.Markdown(baseline, cards, eval.DefaultPolicy)
	fmt.Print(md)
	return os.WriteFile(filepath.Join(*outDir, "leaderboard.md"), []byte(md), 0o644)
}

func splitModels(s string) []string {
	var out []string
	for _, m := range strings.Split(s, ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func readOutcomes(path string) ([]eval.Outcome, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var outs []eval.Outcome
	if err := json.Unmarshal(raw, &outs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return outs, nil
}

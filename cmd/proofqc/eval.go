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
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	casesPath := fs.String("cases", "testdata/cases/cases.json", "labelled cases")
	models := fs.String("models", "", "comma-separated OpenRouter model ids")
	outDir := fs.String("out", "results", "where to write outcomes and the leaderboard")
	maxUSD := fs.Float64("max-usd", 5, "stop calling models once this much is spent")
	workers := fs.Int("workers", 4, "concurrent calls per model")
	reuse := fs.Bool("reuse", false, "score every saved run in -out instead of calling a model again")
	run := fs.Int("run", 1, "run number, so repeated runs of one model are kept side by side")
	promptOnly := fs.Bool("prompt-only", false, "leave order-change rules to the model (to measure what the rules add)")
	tools := fs.Bool("tools", false, "run the tool-calling desk on the labelled offline cases")
	limit := fs.Int("limit", 0, "limit cases for a smoke run (0 runs all cases)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *limit < 0 {
		return fmt.Errorf("limit must not be negative")
	}
	if *tools && *promptOnly {
		return fmt.Errorf("tools mode always applies order rules")
	}
	if *tools && strings.TrimSpace(*models) == "" {
		*models = "anthropic/claude-haiku-5.5"
	}
	suffix := ""
	if *tools {
		suffix = ".tools"
	}

	cases, err := eval.LoadCases(*casesPath)
	if err != nil {
		return err
	}
	fullSuffix := suffix
	if *limit > 0 && *limit < len(cases) {
		cases = cases[:*limit]
		suffix += fmt.Sprintf(".limit%d", *limit)
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
	if err := writeJSON(filepath.Join(*outDir, "baseline-rules"+suffix+".json"), baseOuts); err != nil {
		return err
	}

	var cards []eval.Scorecard
	toolReports := ""
	b := &eval.Budget{Max: *maxUSD}
	callBudget := &toolCallBudget{max: *maxUSD}
	for _, model := range splitModels(*models) {
		stem := filepath.Join(*outDir, strings.ReplaceAll(model, "/", "__")+suffix)
		if *reuse {
			paths, _ := filepath.Glob(stem + ".run*.json")
			if len(paths) == 0 && suffix != fullSuffix {
				fullStem := filepath.Join(*outDir, strings.ReplaceAll(model, "/", "__")+fullSuffix)
				paths, _ = filepath.Glob(fullStem + ".run*.json")
			}
			if len(paths) == 0 {
				return fmt.Errorf("no saved runs for %s in %s", model, *outDir)
			}
			var runs []eval.Scorecard
			var toolOuts []eval.Outcome
			for _, p := range paths {
				if strings.HasSuffix(p, ".partial.json") {
					continue
				}
				outs, err := readOutcomes(p)
				if err != nil {
					return err
				}
				if *limit > 0 {
					outs = limitOutcomes(outs, cases)
				}
				if err := eval.Relabel(outs, cases); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
				runs = append(runs, eval.Score(model, outs))
				toolOuts = append(toolOuts, outs...)
			}
			cards = append(cards, eval.Merge(runs...))
			if *tools {
				toolReports += eval.ToolsMarkdown(model, eval.ScoreTools(toolOuts, 6))
			}
			continue
		}

		client, err := newClient()
		if err != nil {
			return err
		}
		a := agent.Agent{Model: model, LLM: client, PromptOnly: *promptOnly}
		review := func(c eval.Case) (agent.Result, error) {
			img, err := loadImage(c.File)
			if err != nil {
				return agent.Result{}, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			if *tools {
				return (agent.Desk{Model: model, LLM: budgetedCompleter{Completer: client, budget: callBudget}, BeforeCall: callBudget.beforeCall}).Handle(ctx, img, c.Order)
			}
			return a.Review(ctx, img, c.Order)
		}
		outs, err := eval.Run(cases, review, *workers, b)
		if err != nil {
			return err
		}
		s := eval.Score(model, outs)
		cards = append(cards, s)
		if *tools {
			toolReports += eval.ToolsMarkdown(model, eval.ScoreTools(outs, 6))
		}
		fmt.Fprintf(os.Stderr, "%-36s %3d/%d  $%.4f/proof  run spend $%.2f\n", model, s.Correct, s.Cases, s.CostPerProof(), b.Spent())
		path := fmt.Sprintf("%s.run%d.json", stem, *run)
		if s.Skipped > 0 {
			// Never replace a complete run with a partial one.
			path = strings.TrimSuffix(path, ".json") + ".partial.json"
		}
		if err := writeJSON(path, outs); err != nil {
			return err
		}
	}

	md := eval.Markdown(baseline, cards, eval.DefaultPolicy) + toolReports
	fmt.Print(md)
	return os.WriteFile(filepath.Join(*outDir, "leaderboard"+suffix+".md"), []byte(md), 0o644)
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

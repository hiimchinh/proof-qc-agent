package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
)

// budget stops new calls once the spend so far passes the cap.
type budget struct {
	mu    sync.Mutex
	spent float64
	max   float64
}

func (b *budget) add(usd float64) {
	b.mu.Lock()
	b.spent += usd
	b.mu.Unlock()
}

func (b *budget) exhausted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spent >= b.max
}

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
	if *models != "" {
		b := &budget{max: *maxUSD}
		for _, model := range strings.Split(*models, ",") {
			model = strings.TrimSpace(model)
			path := filepath.Join(*outDir, strings.ReplaceAll(model, "/", "__")+".json")
			if *reuse {
				outs, err := readOutcomes(path)
				if err != nil {
					return err
				}
				cards = append(cards, eval.Score(model, outs))
				continue
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			outs := reviewAll(agent.Agent{Model: model, LLM: client}, cases, *workers, b)
			s := eval.Score(model, outs)
			cards = append(cards, s)
			fmt.Fprintf(os.Stderr, "%-36s %2d/%d  $%.4f/proof  spent $%.2f\n", model, s.Correct, s.Cases, s.CostPerProof(), b.spent)
			if err := writeJSON(path, outs); err != nil {
				return err
			}
		}
	}

	md := eval.Markdown(baseline, cards, eval.DefaultPolicy)
	fmt.Print(md)
	return os.WriteFile(filepath.Join(*outDir, "leaderboard.md"), []byte(md), 0o644)
}

func reviewAll(a agent.Agent, cases []eval.Case, workers int, b *budget) []eval.Outcome {
	outs := make([]eval.Outcome, len(cases))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i, c := range cases {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			outs[i] = eval.Outcome{Case: c}
			if b.exhausted() {
				outs[i].Err = "budget exhausted"
				return
			}
			img, err := loadImage(c.File)
			if err != nil {
				outs[i].Err = err.Error()
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			res, err := a.Review(ctx, img, c.Order)
			b.add(res.CostUSD)
			outs[i].Result = res
			if err != nil {
				outs[i].Err = err.Error()
			}
		}()
	}
	wg.Wait()
	return outs
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

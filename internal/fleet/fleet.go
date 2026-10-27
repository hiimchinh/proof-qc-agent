// Package fleet evaluates measured model runs without making network calls.
package fleet

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

const MinSamples = 30

type Score struct {
	Model         string        `json:"model"`
	N             int           `json:"n"`
	Accuracy      float64       `json:"accuracy"`
	NotesPassRate float64       `json:"notes_pass_rate"`
	CostPerOrder  float64       `json:"cost_per_order"`
	P50           time.Duration `json:"p50_ns"`
}

// scorecards retain eval's answered-call latency while counting every failed
// call in the accuracy and note-quality denominators.
func scorecards(runs []store.AgentRun) map[string]eval.Scorecard {
	groups := map[string][]store.AgentRun{}
	for _, run := range runs {
		groups[run.Model] = append(groups[run.Model], run)
	}
	cards := make(map[string]eval.Scorecard, len(groups))
	for model, group := range groups {
		outcomes := make([]eval.Outcome, 0, len(group))
		correct, clean := 0, 0
		for _, run := range group {
			result := run.Result
			result.CostUSD = run.CostUSD
			result.Latency = run.Latency
			outcomes = append(outcomes, eval.Outcome{Case: eval.Case{Want: run.Expected}, Result: result, Err: run.Error})
			if run.Error == "" {
				if run.Correct {
					correct++
				}
				if run.NotesPass {
					clean++
				}
			}
		}
		card := eval.Score(model, outcomes)
		card.Correct, card.LintClean, card.Answered = correct, clean, len(group)
		cards[model] = card
	}
	return cards
}
func score(card eval.Scorecard) Score {
	return Score{Model: card.Model, N: card.Cases, Accuracy: card.Accuracy(), NotesPassRate: card.LintPassRate(), CostPerOrder: card.CostPerProof(), P50: card.P50()}
}

// Scoreboard is stable by model name; errors still consume a sample and budget.
func Scoreboard(runs []store.AgentRun) []Score {
	cards := scorecards(runs)
	out := make([]Score, 0, len(cards))
	for _, card := range cards {
		out = append(out, score(card))
	}
	slices.SortFunc(out, func(a, b Score) int { return strings.Compare(a.Model, b.Model) })
	return out
}

// Evaluate uses the rules score measured on the stream supplied by the caller.
// It returns changes for Store.ApplyFleet to persist together under the tick lock.
func Evaluate(state store.FleetState, runs []store.AgentRun, baseline eval.Scorecard) (store.FleetState, []store.FleetEvent) {
	cards := scorecards(runs)
	var events []store.FleetEvent
	championScore, challengerScore := score(cards[state.Champion]), score(cards[state.Challenger])
	policy := eval.DefaultPolicy
	event := func(kind, model, reason string) {
		numbers, _ := json.Marshal(map[string]any{
			"model": score(cards[model]), "champion": championScore, "challenger": challengerScore,
			"baseline": map[string]any{"n": baseline.Cases, "accuracy": baseline.Accuracy()},
			"policy": map[string]any{
				"min_samples": MinSamples, "min_accuracy": policy.MinAccuracy,
				"min_gain_over_baseline": policy.MinGainOverBaseline, "min_notes_pass": policy.MinLintPass,
				"max_cost_per_order": policy.MaxCostPerProof, "max_p50_ns": policy.MaxP50,
			},
		})
		events = append(events, store.FleetEvent{Kind: kind, Model: model, Reason: reason, Numbers: numbers})
	}
	for _, role := range []*string{&state.Champion, &state.Challenger} {
		model := *role
		card := cards[model]
		if model == "" || card.Cases < MinSamples {
			continue
		}
		keep, reasons := policy.Verdict(card, baseline)
		if !keep {
			event("retired", model, strings.Join(reasons, "; "))
			*role = ""
		}
	}
	challenger := cards[state.Challenger]
	if state.Challenger == "" || challenger.Cases < MinSamples {
		return state, events
	}
	champion := cards[state.Champion]
	// Only compare measured, sufficiently sampled models, unless no champion remains.
	if state.Champion != "" && champion.Cases < MinSamples {
		return state, events
	}
	better := state.Champion == "" || challenger.Accuracy() > champion.Accuracy() ||
		(challenger.Accuracy() == champion.Accuracy() && challenger.CostPerProof() < champion.CostPerProof())
	if better {
		event("promoted", state.Challenger, fmt.Sprintf("qualified challenger replaces %q by accuracy then cost", state.Champion))
		state.Champion, state.Challenger = state.Challenger, ""
	}
	return state, events
}

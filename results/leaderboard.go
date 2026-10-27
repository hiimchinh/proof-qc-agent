package results

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/eval"
)

// Variant says where the order-change rules lived during a run.
type Variant string

const (
	RulesInCode Variant = "rules in code"
	PromptOnly  Variant = "prompt only"
)

// variantDirs lists the run directories the leaderboard reads, in display order.
var variantDirs = []variantDir{{"v2", RulesInCode}, {"v2-prompt-only", PromptOnly}}

type variantDir struct {
	dir     string
	variant Variant
}

const baselineModel = "baseline-rules"

// Score is one reviewer's pooled result over every complete run of a variant.
type Score struct {
	Model        string
	Variant      Variant
	Runs         int
	Cases        int
	Accuracy     float64
	AccuracyLow  float64
	AccuracyHigh float64
	LintPass     float64
	CostPerProof float64
	P50          time.Duration
	Keep         bool
	Verdict      string // "reference" for the rules baseline, else keep or retire with reasons
}

// Leaderboard scores every saved run in fsys against today's labels. Partial
// runs are left out; an empty fsys gives an empty leaderboard.
func Leaderboard(fsys fs.FS, cases []eval.Case) ([]Score, error) {
	var out []Score
	var reference *eval.Scorecard
	for _, vd := range variantDirs {
		cards, err := scoreDir(fsys, vd.dir, cases)
		if err != nil {
			return nil, err
		}
		if b, ok := cards[baselineModel]; ok && reference == nil {
			reference = &b
		}
		for model, card := range cards {
			s := newScore(card, vd.variant)
			switch {
			case model == baselineModel:
				s.Verdict = "reference"
			case reference == nil:
				s.Verdict = "no rules baseline to compare against"
			default:
				keep, reasons := eval.DefaultPolicy.Verdict(card, *reference)
				s.Keep, s.Verdict = keep, "keep"
				if !keep {
					s.Verdict = "retire: " + strings.Join(reasons, "; ")
				}
			}
			out = append(out, s)
		}
	}
	order := func(v Variant) int {
		return slices.IndexFunc(variantDirs, func(d variantDir) bool { return d.variant == v })
	}
	slices.SortFunc(out, func(a, b Score) int {
		return cmp.Or(
			cmp.Compare(order(a.Variant), order(b.Variant)),
			cmp.Compare(b.Accuracy, a.Accuracy),
			cmp.Compare(a.CostPerProof, b.CostPerProof),
			cmp.Compare(a.Model, b.Model),
		)
	})
	return out, nil
}

// scoreDir merges every complete run file in dir, keyed by model.
func scoreDir(fsys fs.FS, dir string, cases []eval.Case) (map[string]eval.Scorecard, error) {
	names, err := fs.Glob(fsys, dir+"/*.json")
	if err != nil {
		return nil, err
	}
	runs := map[string][]eval.Scorecard{}
	for _, name := range names {
		base := path.Base(name)
		if strings.HasSuffix(base, ".partial.json") {
			continue
		}
		model := modelName(base)
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var outs []eval.Outcome
		if err := json.Unmarshal(raw, &outs); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if err := eval.Relabel(outs, cases); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		runs[model] = append(runs[model], eval.Score(model, outs))
	}
	cards := make(map[string]eval.Scorecard, len(runs))
	for model, rs := range runs {
		cards[model] = eval.Merge(rs...)
	}
	return cards, nil
}

// modelName turns "x-ai__grok-4.7.run2.json" into "x-ai/grok-4.7".
func modelName(file string) string {
	stem := strings.TrimSuffix(file, ".json")
	if i := strings.LastIndex(stem, ".run"); i > 0 {
		stem = stem[:i]
	}
	return strings.ReplaceAll(stem, "__", "/")
}

func newScore(c eval.Scorecard, v Variant) Score {
	lo, hi := c.AccuracyRange()
	return Score{
		Model: c.Model, Variant: v, Runs: c.Runs(), Cases: c.Cases,
		Accuracy: c.Accuracy(), AccuracyLow: lo, AccuracyHigh: hi,
		LintPass: c.LintPassRate(), CostPerProof: c.CostPerProof(), P50: c.P50(),
	}
}

// Package eval scores reviewers on hand-labelled orders and decides which ones
// earn their keep.
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
)

// Case is one labelled order. Want comes from the labelling rules in the README,
// written by hand, never from running a reviewer.
type Case struct {
	ID    string         `json:"id"`
	File  string         `json:"file"`
	Order artwork.Order  `json:"order"`
	Want  agent.Decision `json:"want"`
	Why   string         `json:"why"`
}

// LoadCases reads cases.json and resolves artwork paths next to it.
func LoadCases(path string) ([]Case, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cs []Case
	if err := json.Unmarshal(raw, &cs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for i := range cs {
		if err := cs[i].Order.Validate(); err != nil {
			return nil, fmt.Errorf("%s: case %s: %w", path, cs[i].ID, err)
		}
		cs[i].File = filepath.Join(filepath.Dir(path), cs[i].File)
	}
	return cs, nil
}

// Outcome is one reviewer's answer to one case.
type Outcome struct {
	Case   Case         `json:"case"`
	Result agent.Result `json:"result"`
	Err    string       `json:"error,omitempty"`
	// Skipped means the case never ran (budget spent); it is neither right nor wrong.
	Skipped bool `json:"skipped,omitempty"`
}

// Relabel replaces each saved outcome's expected answer with the one in the
// current cases, so saved runs are always scored against today's labels.
func Relabel(outs []Outcome, cases []Case) error {
	want := make(map[string]Case, len(cases))
	for _, c := range cases {
		want[c.ID] = c
	}
	for i := range outs {
		c, ok := want[outs[i].Case.ID]
		if !ok {
			return fmt.Errorf("saved outcome %q is not in the current cases", outs[i].Case.ID)
		}
		outs[i].Case.Want, outs[i].Case.Why = c.Want, c.Why
	}
	return nil
}

// Miss is a case the reviewer got wrong.
type Miss struct {
	ID   string         `json:"id"`
	Want agent.Decision `json:"want"`
	Got  agent.Decision `json:"got"`
	Err  string         `json:"error,omitempty"`
}

// Scorecard summarises one reviewer over all cases.
type Scorecard struct {
	Model     string  `json:"model"`
	Cases     int     `json:"cases"`
	Correct   int     `json:"correct"`
	Errors    int     `json:"errors"`
	Skipped   int     `json:"skipped"`
	Answered  int     `json:"answered"`
	LintClean int     `json:"lint_clean"`
	CostUSD   float64 `json:"cost_usd"`
	Misses    []Miss  `json:"misses"`
	latencies []time.Duration
}

// Score builds a scorecard; a failed call counts as a wrong answer.
func Score(model string, outs []Outcome) Scorecard {
	s := Scorecard{Model: model, Cases: len(outs)}
	for _, o := range outs {
		if o.Skipped {
			s.Skipped++
			continue
		}
		s.CostUSD += o.Result.CostUSD
		if o.Err != "" {
			s.Errors++
			s.Misses = append(s.Misses, Miss{ID: o.Case.ID, Want: o.Case.Want, Err: o.Err})
			continue
		}
		s.Answered++
		s.latencies = append(s.latencies, o.Result.Latency)
		if len(o.Result.Lint) == 0 {
			s.LintClean++
		}
		if o.Result.Decision == o.Case.Want {
			s.Correct++
		} else {
			s.Misses = append(s.Misses, Miss{ID: o.Case.ID, Want: o.Case.Want, Got: o.Result.Decision})
		}
	}
	return s
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// Accuracy is correct decisions over all cases.
func (s Scorecard) Accuracy() float64 { return ratio(s.Correct, s.Cases) }

// LintPassRate is the share of answered notes that follow every support principle.
func (s Scorecard) LintPassRate() float64 { return ratio(s.LintClean, s.Answered) }

// CostPerProof is total spend, failed calls included, over the cases that ran.
func (s Scorecard) CostPerProof() float64 {
	ran := s.Cases - s.Skipped
	if ran == 0 {
		return 0
	}
	return s.CostUSD / float64(ran)
}

// P50 is the median latency of answered cases.
func (s Scorecard) P50() time.Duration {
	if len(s.latencies) == 0 {
		return 0
	}
	l := slices.Clone(s.latencies)
	slices.Sort(l)
	return l[(len(l)-1)/2]
}

// Policy is the bar an agent has to clear to stay in production.
type Policy struct {
	MinGainOverBaseline float64       // accuracy points over the rules-only reviewer
	MinLintPass         float64       // share of customer notes that follow the support principles
	MaxCostPerProof     float64       // USD
	MaxP50              time.Duration // a designer should not wait on the agent
}

// DefaultPolicy keeps an agent only if it is clearly better than rules, polite, cheap and fast.
var DefaultPolicy = Policy{
	MinGainOverBaseline: 0.10,
	MinLintPass:         0.95,
	MaxCostPerProof:     0.01,
	MaxP50:              30 * time.Second,
}

// Verdict says whether to keep a reviewer, with every reason it fails.
func (p Policy) Verdict(s, baseline Scorecard) (keep bool, reasons []string) {
	if s.Skipped > 0 {
		return false, []string{fmt.Sprintf("incomplete run: %d of %d cases skipped", s.Skipped, s.Cases)}
	}
	if gain := s.Accuracy() - baseline.Accuracy(); gain < p.MinGainOverBaseline {
		reasons = append(reasons, fmt.Sprintf("only %+.0f pts over the rules baseline (need %+.0f)", gain*100, p.MinGainOverBaseline*100))
	}
	if r := s.LintPassRate(); r < p.MinLintPass {
		reasons = append(reasons, fmt.Sprintf("%.0f%% of notes follow the support principles (need %.0f%%)", r*100, p.MinLintPass*100))
	}
	if c := s.CostPerProof(); c > p.MaxCostPerProof {
		reasons = append(reasons, fmt.Sprintf("$%.4f per proof (max $%.2f)", c, p.MaxCostPerProof))
	}
	if l := s.P50(); l > p.MaxP50 {
		reasons = append(reasons, fmt.Sprintf("p50 %s (max %s)", l.Round(time.Second), p.MaxP50))
	}
	return len(reasons) == 0, reasons
}

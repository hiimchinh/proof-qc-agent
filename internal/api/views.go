package api

import (
	"time"

	graphql "github.com/graph-gophers/graphql-go"

	"github.com/hiimchinh/proof-qc-agent/internal/eval"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
	"github.com/hiimchinh/proof-qc-agent/results"
)

// The view types mirror schema.graphql field for field; graphql-go resolves
// fields by name, so no field needs a resolver method.

type reviewView struct {
	Decision     string
	Reasons      []string
	DesignerNote string
	CustomerNote string
	OrderChanges []string
	Preflight    preflightView
	Lint         []lintView
	Model        string
	CostUsd      float64
	LatencyMs    int32
	CreatedAt    string
}

type preflightView struct {
	WidthPx      int32
	HeightPx     int32
	EffectiveDpi float64
	Findings     []findingView
}

type findingView struct{ Flag, Message string }

type lintView struct{ Rule, Detail string }

type scoreView struct {
	Model        string
	Variant      string
	Runs         int32
	Cases        int32
	Accuracy     float64
	AccuracyLow  float64
	AccuracyHigh float64
	LintPass     float64
	CostPerProof float64
	P50Ms        int32
	Keep         bool
	Verdict      string
}

type sampleView struct {
	ID       graphql.ID
	ImageURL string
	Customer string
	Product  string
	WidthIn  float64
	HeightIn float64
	Comment  string
	Want     string
	Why      string
}

func newReviewView(r store.Review) reviewView {
	res := r.Result
	v := reviewView{
		Decision: string(res.Decision), DesignerNote: res.DesignerNote, CustomerNote: res.CustomerNote,
		Reasons: nonNil(res.Reasons), OrderChanges: nonNil(res.OrderChanges),
		Model: r.Model, CostUsd: r.CostUSD, LatencyMs: int32(res.Latency / time.Millisecond),
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
		Preflight: preflightView{
			WidthPx: int32(res.Preflight.WidthPx), HeightPx: int32(res.Preflight.HeightPx),
			EffectiveDpi: res.Preflight.EffectiveDPI, Findings: []findingView{},
		},
		Lint: []lintView{},
	}
	for _, f := range res.Preflight.Findings {
		v.Preflight.Findings = append(v.Preflight.Findings, findingView{string(f.Flag), f.Message})
	}
	for _, l := range res.Lint {
		v.Lint = append(v.Lint, lintView{string(l.Rule), l.Detail})
	}
	return v
}

func newScoreView(s results.Score) scoreView {
	return scoreView{
		Model: s.Model, Variant: string(s.Variant), Runs: int32(s.Runs), Cases: int32(s.Cases),
		Accuracy: s.Accuracy, AccuracyLow: s.AccuracyLow, AccuracyHigh: s.AccuracyHigh,
		LintPass: s.LintPass, CostPerProof: s.CostPerProof, P50Ms: int32(s.P50 / time.Millisecond),
		Keep: s.Keep, Verdict: s.Verdict,
	}
}

func newSampleView(c eval.Case) sampleView {
	o := c.Order
	return sampleView{
		ID: graphql.ID(c.ID), ImageURL: "/samples/" + c.File,
		Customer: o.Customer, Product: string(o.Product), WidthIn: o.WidthIn, HeightIn: o.HeightIn,
		Comment: o.Comment, Want: string(c.Want), Why: c.Why,
	}
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

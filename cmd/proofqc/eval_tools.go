package main

import "github.com/hiimchinh/proof-qc-agent/internal/eval"

// limitOutcomes selects the same case subset as a smoke run before relabelling.
func limitOutcomes(outs []eval.Outcome, cases []eval.Case) []eval.Outcome {
	ids := map[string]bool{}
	for _, c := range cases {
		ids[c.ID] = true
	}
	selected := []eval.Outcome{}
	for _, o := range outs {
		if ids[o.Case.ID] {
			selected = append(selected, o)
		}
	}
	return selected
}

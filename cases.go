// Package proofqcagent embeds the labelled eval cases, so one binary can show
// the sample artwork and score saved runs without the repository on disk.
package proofqcagent

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/hiimchinh/proof-qc-agent/internal/eval"
)

//go:embed testdata/cases/cases.json testdata/cases/*.png
var casesFS embed.FS

// Cases returns the labelled cases and the artwork they name, keyed by Case.File.
func Cases() ([]eval.Case, fs.FS, error) {
	art, err := fs.Sub(casesFS, "testdata/cases")
	if err != nil {
		return nil, nil, err
	}
	raw, err := fs.ReadFile(art, "cases.json")
	if err != nil {
		return nil, nil, err
	}
	cases, err := eval.ParseCases(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("embedded cases: %w", err)
	}
	return cases, art, nil
}

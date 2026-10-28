// Package impact restates the acting model's routing as what a customer waiting
// on a free proof would experience. Shadow runs never reach a customer.
package impact

import (
	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

// Window is how many of the latest champion decisions the desk summarizes.
const Window = 100

type Summary struct {
	Orders int
	// StraightToProof counts clean orders a designer could start without triage.
	StraightToProof int
	// IssuesCaught counts problems routed before the proof; each avoids a free revision round.
	IssuesCaught int
	// MissedIssues counts problems routed as ready; each costs the customer an extra round.
	MissedIssues int
	// Detours counts clean orders flagged or problems sent to the wrong fix; the proof is slower.
	Detours int
	// HandedToDesigner counts failed calls; a designer triages those orders as today.
	HandedToDesigner int
}

// Summarize reads the latest window of champion runs. Runs must be oldest first.
func Summarize(runs []store.AgentRun, window int) Summary {
	var s Summary
	for i := len(runs) - 1; i >= 0 && s.Orders < window; i-- {
		run := runs[i]
		if run.Role != "champion" {
			continue
		}
		s.Orders++
		decided := run.Result.Decision
		switch {
		case run.Error != "":
			s.HandedToDesigner++
		case decided == run.Expected && decided == agent.ReadyForProof:
			s.StraightToProof++
		case decided == run.Expected:
			s.IssuesCaught++
		case decided == agent.ReadyForProof:
			s.MissedIssues++
		default:
			s.Detours++
		}
	}
	return s
}

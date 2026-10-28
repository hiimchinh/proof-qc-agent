package impact_test

import (
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/impact"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func run(role string, decided, want agent.Decision, err string) store.AgentRun {
	return store.AgentRun{Role: role, Result: agent.Result{Decision: decided}, Expected: want, Error: err}
}

func TestSummarizeSortsChampionDecisionsByWhatTheCustomerGets(t *testing.T) {
	runs := []store.AgentRun{
		run("champion", agent.ReadyForProof, agent.ReadyForProof, ""),
		run("champion", agent.ReadyForProof, agent.ReadyForProof, ""),
		run("champion", agent.AskCustomer, agent.AskCustomer, ""),
		run("champion", agent.RecreateArtwork, agent.RecreateArtwork, ""),
		run("champion", agent.ReadyForProof, agent.AskCustomer, ""),
		run("champion", agent.ReadyForProof, agent.RecreateArtwork, ""),
		run("champion", agent.AskCustomer, agent.ReadyForProof, ""),
		run("champion", agent.RecreateArtwork, agent.AskCustomer, ""),
		run("champion", "", agent.ReadyForProof, "timeout"),
		run("champion", "", agent.AskCustomer, "status 429 rate limited"),
		// Shadow runs never reach a customer.
		run("challenger", agent.ReadyForProof, agent.AskCustomer, ""),
		run("challenger", "", agent.ReadyForProof, "timeout"),
	}
	want := impact.Summary{Orders: 10, StraightToProof: 2, IssuesCaught: 2, MissedIssues: 2, Detours: 2, HandedToDesigner: 2}
	if got := impact.Summarize(runs, 100); got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestSummarizeReadsOnlyTheLatestWindow(t *testing.T) {
	runs := []store.AgentRun{
		run("champion", agent.ReadyForProof, agent.AskCustomer, ""),
		run("champion", agent.ReadyForProof, agent.AskCustomer, ""),
		run("challenger", agent.AskCustomer, agent.AskCustomer, ""),
		run("champion", agent.AskCustomer, agent.AskCustomer, ""),
		run("champion", agent.ReadyForProof, agent.ReadyForProof, ""),
	}
	want := impact.Summary{Orders: 3, StraightToProof: 1, IssuesCaught: 1, MissedIssues: 1}
	if got := impact.Summarize(runs, 3); got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got := impact.Summarize(nil, 100); got != (impact.Summary{}) {
		t.Fatalf("empty stream: %+v", got)
	}
}

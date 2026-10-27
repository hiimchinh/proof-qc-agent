package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

func TestDeskBoundsToolExecutionsWithinOneCompletion(t *testing.T) {
	resp := llm.Response{FinishReason: "tool_calls", CostUSD: 0.001}
	for i := range 7 {
		c := toolResponse("customer_history", "{}").ToolCalls[0]
		c.ID = fmt.Sprint(i)
		resp.ToolCalls = append(resp.ToolCalls, c)
	}
	m := &toolScript{replies: []llm.Response{resp}}
	history := 0
	res, err := (Desk{LLM: m, History: func(context.Context, string) ([]HistoryEntry, error) { history++; return nil, nil }}).Handle(context.Background(), artworkImage(150), order)
	if err == nil || !strings.Contains(err.Error(), "max tool steps") || history != 0 || res.Attempts != 1 {
		t.Fatalf("calls=%d history=%d err=%v", res.Attempts, history, err)
	}
}

func TestDeskRecordsBilledNoteRepairTrace(t *testing.T) {
	m := &toolScript{replies: []llm.Response{toolResponse("measure_artwork", "{}"), toolResponse("route_order", routeArgs(AskCustomer, "Sorry!")), {FinishReason: "stop", JSON: []byte(routeArgs(AskCustomer, "Thanks Brian. Send the blue version."))}}}
	res, err := (Desk{LLM: m}).Handle(context.Background(), artworkImage(150), order)
	if err != nil || len(res.Trace) != 3 || res.Trace[2].Tool != "repair_note" || !strings.Contains(res.Trace[2].Result, "lint") {
		t.Fatalf("trace=%+v err=%v", res.Trace, err)
	}
}

package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

func TestDeskStopsAtMaxStepsIncludingDefault(t *testing.T) {
	for _, limit := range []int{0, 2} {
		calls := limit
		if calls == 0 {
			calls = 6
		}
		m := &toolScript{}
		for i := range calls {
			resp := toolResponse("measure_artwork", "{}")
			resp.ToolCalls[0].ID = fmt.Sprint(i)
			m.replies = append(m.replies, resp)
		}
		res, err := (Desk{LLM: m, MaxSteps: limit}).Handle(context.Background(), artworkImage(150), order)
		if err == nil || !strings.Contains(err.Error(), "max steps") || res.Attempts != calls || len(m.requests) != calls {
			t.Fatalf("result=%+v err=%v", res, err)
		}
	}
}

func TestDeskPropagatesHistoryErrors(t *testing.T) {
	boom := errors.New("store unavailable")
	m := &toolScript{replies: []llm.Response{toolResponse("customer_history", "{}")}}
	res, err := (Desk{LLM: m, History: func(context.Context, string) ([]HistoryEntry, error) { return nil, boom }}).Handle(context.Background(), artworkImage(150), order)
	if !errors.Is(err, boom) || res.CostUSD != 0.001 || len(res.Trace) != 1 || res.Trace[0].Result != "store unavailable" {
		t.Fatalf("result=%+v err=%v", res, err)
	}
}

func TestDeskNilHistoryReturnsEmptyArray(t *testing.T) {
	m := &toolScript{replies: []llm.Response{toolResponse("customer_history", "{}"), toolResponse("measure_artwork", "{}"), toolResponse("route_order", routeArgs(ReadyForProof, "Thanks Brian. Your proof is on the way."))}}
	_, err := (Desk{LLM: m}).Handle(context.Background(), artworkImage(150), order)
	if err != nil || m.requests[1].Messages[1].Content != "[]" {
		t.Fatalf("requests=%+v err=%v", m.requests, err)
	}
}

func TestDeskRejectsUnsupportedCallsWithoutRetry(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response llm.Response
	}{
		{"unknown", toolResponse("delete_order", "{}")},
		{"arguments", toolResponse("measure_artwork", `{"dpi":1}`)},
		{"invalid arguments", toolResponse("measure_artwork", `null`)},
		{"unsupported finish", llm.Response{FinishReason: "length", CostUSD: 0.001}},
		{"prose", llm.Response{FinishReason: "stop", CostUSD: 0.001}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &toolScript{replies: []llm.Response{tc.response}}
			res, err := (Desk{LLM: m}).Handle(context.Background(), artworkImage(150), order)
			if err == nil || len(m.requests) != 1 || res.CostUSD != 0.001 {
				t.Fatalf("result=%+v err=%v", res, err)
			}
		})
	}
}

func TestDeskCountsFailedCallUsage(t *testing.T) {
	boom := errors.New("upstream failed")
	m := &toolScript{replies: []llm.Response{{CostUSD: 0.03}}, err: boom}
	res, err := (Desk{LLM: m}).Handle(context.Background(), artworkImage(150), order)
	if !errors.Is(err, boom) || res.CostUSD != 0.03 || res.Attempts != 1 {
		t.Fatalf("result=%+v err=%v", res, err)
	}
}

func TestDeskBudgetBlocksInitialCall(t *testing.T) {
	boom := errors.New("daily cap")
	m := &toolScript{}
	res, err := (Desk{LLM: m, BeforeCall: func(context.Context) error { return boom }}).Handle(context.Background(), artworkImage(150), order)
	if !errors.Is(err, boom) || res.Attempts != 0 || len(m.requests) != 0 {
		t.Fatalf("result=%+v err=%v", res, err)
	}
}

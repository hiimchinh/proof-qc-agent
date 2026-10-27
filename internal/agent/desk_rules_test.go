package agent

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

func TestDeskRuleOverrides(t *testing.T) {
	changed := strings.Replace(routeArgs(ReadyForProof, "Thanks Brian. Your proof is on the way."), `"width_in":null`, `"width_in":5`, 1)
	for _, tc := range []struct {
		name string
		img  image.Image
		args string
		want Decision
	}{
		{"order change", artworkImage(150), changed, AskCustomer},
		{"low resolution", artworkImage(100), routeArgs(ReadyForProof, "Thanks Brian. Your proof is on the way."), RecreateArtwork},
		{"empty", image.NewNRGBA(image.Rect(0, 0, 300, 300)), routeArgs(ReadyForProof, "Thanks Brian. Your proof is on the way."), AskCustomer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &toolScript{replies: []llm.Response{toolResponse("measure_artwork", "{}"), toolResponse("route_order", tc.args)}}
			res, err := (Desk{LLM: m}).Handle(context.Background(), tc.img, order)
			if err != nil || res.Decision != tc.want || !res.OverriddenByRules || !res.Trace[1].Overridden || len(res.Lint) != 0 || len(res.Reasons) < 2 {
				t.Fatalf("result=%+v err=%v", res, err)
			}
		})
	}
}

func TestDeskRepairsOnlyNoteAndCountsEveryBilledCall(t *testing.T) {
	m := &toolScript{replies: []llm.Response{toolResponse("measure_artwork", "{}"), toolResponse("route_order", routeArgs(AskCustomer, "Sorry!")), {FinishReason: "stop", JSON: json.RawMessage(routeArgs(ReadyForProof, "Thanks Brian. Send the blue version and we'll make your proof.")), CostUSD: 0.002}}}
	before := 0
	res, err := (Desk{LLM: m, BeforeCall: func(context.Context) error { before++; return nil }}).Handle(context.Background(), artworkImage(150), order)
	if err != nil || res.Decision != AskCustomer || res.Attempts != 3 || res.CostUSD != 0.004 || before != 3 || len(res.Repaired) == 0 || len(res.Lint) != 0 {
		t.Fatalf("result=%+v err=%v before=%d", res, err, before)
	}
	if len(m.requests[2].Tools) != 0 || len(m.requests[2].Schema) == 0 {
		t.Fatal("repair must use structured Complete")
	}
}

func TestDeskBudgetBlocksRepairAndRetainsRoute(t *testing.T) {
	blocked := errors.New("daily cap")
	for _, tc := range []struct {
		name   string
		max    int
		budget bool
		want   string
	}{
		{"max calls", 2, false, "max steps"}, {"budget", 6, true, "daily cap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &toolScript{replies: []llm.Response{toolResponse("measure_artwork", "{}"), toolResponse("route_order", routeArgs(AskCustomer, "Sorry!"))}}
			before := 0
			res, err := (Desk{LLM: m, MaxSteps: tc.max, BeforeCall: func(context.Context) error {
				before++
				if tc.budget && before == 3 {
					return blocked
				}
				return nil
			}}).Handle(context.Background(), artworkImage(150), order)
			if err != nil || res.Decision != AskCustomer || res.Attempts != 2 || !strings.Contains(res.RepairError, tc.want) || len(res.Lint) == 0 || len(m.requests) != 2 {
				t.Fatalf("result=%+v err=%v", res, err)
			}
		})
	}
}

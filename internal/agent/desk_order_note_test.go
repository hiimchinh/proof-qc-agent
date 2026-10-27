package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

func TestDeskSuppliesOrderChangeNote(t *testing.T) {
	for _, tc := range []struct{ name, requested, change string }{
		{"size down", `{"width_in":1.5,"height_in":1.5,"shape":"none","material":""}`, "a 1.5x1.5 in size instead of 3x3 in"},
		{"shape", `{"width_in":null,"height_in":null,"shape":"rectangle","material":""}`, "a rectangle shape instead of die cut"},
		{"material", `{"width_in":null,"height_in":null,"shape":"none","material":"holographic"}`, "holographic instead of standard white vinyl"},
	} {
		for _, note := range []string{"null", `""`} {
			t.Run(tc.name+"/"+note, func(t *testing.T) {
				o := artwork.Order{Customer: "Brian Smith", Product: artwork.DieCut, WidthIn: 3, HeightIn: 3, Comment: "Actually I need them 1.5x1.5 inches."}
				args := fmt.Sprintf(`{"decision":"ask_customer","reasons":["requested change"],"designer_note":"Confirm order first.","customer_note":%s,"requested":%s}`, note, tc.requested)
				m := &toolScript{replies: []llm.Response{toolResponse("measure_artwork", "{}"), toolResponse("route_order", args)}}
				res, err := (Desk{LLM: m}).Handle(context.Background(), artworkImage(150), o)
				if err != nil {
					t.Fatal(err)
				}
				want := "Thanks Brian. Your comment asks for " + tc.change + ", which changes your order. Reply to confirm and we'll update the order and send your proof."
				if res.Decision != AskCustomer || res.CustomerNote != want || len(res.Lint) != 0 || res.Attempts != 2 || len(m.requests) != 2 {
					t.Fatalf("result=%+v", res)
				}
				if !res.OverriddenByRules || len(res.Trace) != 2 || !res.Trace[1].Overridden || !strings.Contains(res.Trace[1].Result, "customer note supplied by rules") || string(res.Trace[1].Args) != args {
					t.Fatalf("trace=%+v", res.Trace)
				}
			})
		}
	}
}

func TestDeskRejectsNullNoteWithoutRule(t *testing.T) {
	args := strings.Replace(routeArgs(AskCustomer, "placeholder"), `"customer_note":"placeholder"`, `"customer_note":null`, 1)
	m := &toolScript{replies: []llm.Response{toolResponse("measure_artwork", "{}"), toolResponse("route_order", args)}}
	res, err := (Desk{LLM: m}).Handle(context.Background(), artworkImage(150), order)
	if err == nil || !strings.Contains(err.Error(), "customer_note") || res.Decision != "" || len(m.requests) != 2 {
		t.Fatalf("result=%+v err=%v", res, err)
	}
}

func TestDeskRepairsEmptyNoteWithoutRule(t *testing.T) {
	for _, note := range []string{"", " \t"} {
		m := &toolScript{replies: []llm.Response{
			toolResponse("measure_artwork", "{}"),
			toolResponse("route_order", routeArgs(AskCustomer, note)),
			{FinishReason: "stop", JSON: []byte(routeArgs(AskCustomer, "Thanks Brian. Send the blue version and we'll make your proof."))},
		}}
		res, err := (Desk{LLM: m}).Handle(context.Background(), artworkImage(150), order)
		if err != nil || res.Decision != AskCustomer || res.Attempts != 3 || res.CustomerNote != "Thanks Brian. Send the blue version and we'll make your proof." || len(res.Lint) != 0 || len(res.Repaired) == 0 || len(res.Trace) != 3 || res.Trace[2].Tool != "repair_note" {
			t.Fatalf("result=%+v err=%v", res, err)
		}
	}
}

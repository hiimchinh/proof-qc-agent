package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

func TestDeskRejectsMalformedRouteSchema(t *testing.T) {
	valid := routeArgs(ReadyForProof, "Thanks Brian. Your proof is on the way.")
	for _, args := range []string{
		`{"decision":"ready_for_proof"}`,
		strings.Replace(valid, `"shape":"none"`, `"shape":"circle"`, 1),
		strings.Replace(valid, `"width_in":null`, `"width_in":-1`, 1),
		strings.Replace(valid, `"customer_note":`, `"unexpected":true,"customer_note":`, 1),
		strings.Replace(valid, `"material":""`, `"material":null`, 1),
	} {
		m := &toolScript{replies: []llm.Response{toolResponse("measure_artwork", "{}"), toolResponse("route_order", args)}}
		res, err := (Desk{LLM: m}).Handle(context.Background(), artworkImage(150), order)
		if err == nil || res.Decision != "" || len(m.requests) != 2 {
			t.Fatalf("args=%s decision=%s err=%v", args, res.Decision, err)
		}
	}
}

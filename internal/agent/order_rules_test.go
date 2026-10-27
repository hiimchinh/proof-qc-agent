package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
)

func answerWithRequest(decision string, req map[string]any) string {
	full := map[string]any{"width_in": nil, "height_in": nil, "shape": "none", "material": ""}
	for k, v := range req {
		full[k] = v
	}
	b, _ := json.Marshal(map[string]any{
		"decision": decision, "reasons": []string{"r"}, "designer_note": "d",
		"customer_note": "Thanks Olivia. Your proof is on the way.", "requested": full,
	})
	return string(b)
}

var squareOrder = artwork.Order{Customer: "Olivia Brown", Product: artwork.DieCut, WidthIn: 3, HeightIn: 3, Comment: "keep it as a square"}

func review(t *testing.T, o artwork.Order, ans string) Result {
	t.Helper()
	res, err := Agent{Model: "m", LLM: &scripted{answers: []string{ans}}}.Review(context.Background(), artworkImage(300), o)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestOrderChangesOverrideTheModel(t *testing.T) {
	cases := []struct {
		name string
		o    artwork.Order
		req  map[string]any
	}{
		{"square on a die-cut order", squareOrder, map[string]any{"shape": "square"}},
		{"rectangle on a die-cut order", squareOrder, map[string]any{"shape": "rectangle"}},
		{"die cut on a rectangle order", artwork.Order{Customer: "Olivia", Product: artwork.Rectangle, WidthIn: 3, HeightIn: 2}, map[string]any{"shape": "die_cut"}},
		{"bigger size", squareOrder, map[string]any{"width_in": 5, "height_in": 5}},
		{"one side named", squareOrder, map[string]any{"width_in": 5}},
		{"holographic", squareOrder, map[string]any{"material": "holographic vinyl"}},
	}
	for _, c := range cases {
		res := review(t, c.o, answerWithRequest("ready_for_proof", c.req))
		if res.Decision != AskCustomer || len(res.OrderChanges) == 0 {
			t.Errorf("%s: decision = %s changes = %v, want ask_customer", c.name, res.Decision, res.OrderChanges)
		}
		if len(res.Lint) != 0 || !strings.Contains(res.CustomerNote, "Olivia") {
			t.Errorf("%s: customer note %q breaks %v", c.name, res.CustomerNote, res.Lint)
		}
	}
}

func TestRequestsThatMatchTheOrderChangeNothing(t *testing.T) {
	cases := []struct {
		name string
		o    artwork.Order
		req  map[string]any
	}{
		{"nothing requested", squareOrder, nil},
		{"same size", squareOrder, map[string]any{"width_in": 3, "height_in": 3}},
		{"one matching side named", artwork.Order{Customer: "Olivia", Product: artwork.Rectangle, WidthIn: 4, HeightIn: 2}, map[string]any{"width_in": 2}},
		{"rotated size", artwork.Order{Customer: "Olivia", Product: artwork.Rectangle, WidthIn: 4, HeightIn: 2}, map[string]any{"width_in": 2, "height_in": 4}},
		{"same shape", squareOrder, map[string]any{"shape": "die_cut"}},
		{"standard material", squareOrder, map[string]any{"material": "white vinyl"}},
	}
	for _, c := range cases {
		res := review(t, c.o, answerWithRequest("ready_for_proof", c.req))
		if res.Decision != ReadyForProof || len(res.OrderChanges) != 0 {
			t.Errorf("%s: decision = %s changes = %v, want ready_for_proof", c.name, res.Decision, res.OrderChanges)
		}
	}
}

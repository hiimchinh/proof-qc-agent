package agent

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
)

// scripted answers each call with the next canned JSON and records the prompts it saw.
type scripted struct {
	answers []string // "ERR" makes that call fail
	prompts []string
}

func (s *scripted) Complete(_ context.Context, r llm.Request) (llm.Response, error) {
	s.prompts = append(s.prompts, r.Text)
	a := s.answers[0]
	s.answers = s.answers[1:]
	if a == "ERR" {
		return llm.Response{}, errors.New("502 upstream")
	}
	return llm.Response{JSON: json.RawMessage(a), CostUSD: 0.001}, nil
}

func cannedAnswer(decision, customerNote string) string {
	b, _ := json.Marshal(map[string]any{
		"decision": decision, "reasons": []string{"r"}, "designer_note": "d", "customer_note": customerNote,
	})
	return string(b)
}

func artworkImage(dpi int) image.Image {
	n := dpi * 3
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := n / 4; y < 3*n/4; y++ {
		for x := n / 4; x < 3*n/4; x++ {
			img.Set(x, y, color.NRGBA{200, 30, 40, 255})
		}
	}
	return img
}

var order = artwork.Order{Customer: "Brian Smith", Product: artwork.DieCut, WidthIn: 3, HeightIn: 3, Comment: "I'll email the blue version tomorrow"}

func TestReviewUsesTheModelDecision(t *testing.T) {
	m := &scripted{answers: []string{cannedAnswer("ask_customer", "Thanks Brian. Send the blue version and we'll make your proof.")}}
	res, err := Agent{Model: "m", LLM: m}.Review(context.Background(), artworkImage(300), order)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != AskCustomer || len(res.Lint) != 0 || res.Attempts != 1 {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(m.prompts[0], "I'll email the blue version tomorrow") || !strings.Contains(m.prompts[0], "effective_dpi") {
		t.Fatalf("prompt lacks the comment or the preflight: %s", m.prompts[0])
	}
}

func TestReviewRepairsANoteThatBreaksSupportPrinciples(t *testing.T) {
	m := &scripted{answers: []string{
		cannedAnswer("ask_customer", "Sorry, we need the blue version."),
		cannedAnswer("ask_customer", "Thanks Brian. Send the blue version and we'll make your proof."),
	}}
	res, err := Agent{Model: "m", LLM: m}.Review(context.Background(), artworkImage(300), order)
	if err != nil {
		t.Fatal(err)
	}
	if res.Attempts != 2 || len(res.Lint) != 0 || res.CostUSD != 0.002 {
		t.Fatalf("result = %+v", res)
	}
	// The first draft says sorry and leaves out Brian's name.
	if len(res.Repaired) != 2 || res.Repaired[0].Rule != "negative" || res.Repaired[1].Rule != "no_name" {
		t.Fatalf("repaired = %v, want negative and no_name from the first draft", res.Repaired)
	}
	if !strings.Contains(m.prompts[1], "negative") {
		t.Fatalf("repair prompt does not name the broken rule: %s", m.prompts[1])
	}
}

func TestReviewKeepsTheLastNoteWhenRepairFails(t *testing.T) {
	m := &scripted{answers: []string{cannedAnswer("ready_for_proof", "Sorry!"), cannedAnswer("ready_for_proof", "Sorry again!")}}
	res, err := Agent{Model: "m", LLM: m}.Review(context.Background(), artworkImage(300), order)
	if err != nil {
		t.Fatal(err)
	}
	if res.Attempts != 2 || len(res.Lint) == 0 {
		t.Fatalf("result = %+v, want lint violations reported", res)
	}
}

func TestReviewRejectsAnUnknownDecision(t *testing.T) {
	m := &scripted{answers: []string{cannedAnswer("ship_it", "Thanks Brian.")}}
	if _, err := (Agent{Model: "m", LLM: m}).Review(context.Background(), artworkImage(300), order); err == nil {
		t.Fatal("want an error for an unknown decision")
	}
}

func TestBaselineFollowsOnlyThePreflight(t *testing.T) {
	cases := []struct {
		dpi  int
		want Decision
	}{{300, ReadyForProof}, {100, RecreateArtwork}}
	for _, c := range cases {
		res := Baseline(artworkImage(c.dpi), order)
		if res.Decision != c.want {
			t.Errorf("%d dpi: decision = %s, want %s", c.dpi, res.Decision, c.want)
		}
		if vs := res.Lint; len(vs) != 0 {
			t.Errorf("%d dpi: baseline note breaks %v", c.dpi, vs)
		}
	}
}

func TestAFailedRepairKeepsTheFirstAnswer(t *testing.T) {
	m := &scripted{answers: []string{cannedAnswer("ask_customer", "Sorry, send the blue version."), "ERR"}}
	res, err := Agent{Model: "m", LLM: m}.Review(context.Background(), artworkImage(300), order)
	if err != nil {
		t.Fatalf("err = %v; a valid decision must survive a failed note repair", err)
	}
	if res.Decision != AskCustomer || len(res.Lint) == 0 || res.RepairError == "" {
		t.Fatalf("result = %+v", res)
	}
}

func TestARepairCannotChangeTheDecision(t *testing.T) {
	m := &scripted{answers: []string{
		cannedAnswer("ask_customer", "Sorry, send the blue version."),
		cannedAnswer("ready_for_proof", "Thanks Brian. Your proof is on the way."),
	}}
	res, err := Agent{Model: "m", LLM: m}.Review(context.Background(), artworkImage(300), order)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != AskCustomer || res.CustomerNote != "Thanks Brian. Your proof is on the way." {
		t.Fatalf("result = %+v; only the note may change on repair", res)
	}
}

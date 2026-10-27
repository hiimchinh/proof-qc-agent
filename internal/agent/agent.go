// Package agent decides what happens to an order's artwork before a designer
// makes the proof, and writes the note the customer reads.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/notelint"
)

// Decision is the next step for the order.
type Decision string

const (
	ReadyForProof   Decision = "ready_for_proof"  // a designer can make the proof now
	RecreateArtwork Decision = "recreate_artwork" // a designer redraws low-resolution art first
	AskCustomer     Decision = "ask_customer"     // only the customer can unblock the proof
)

func (d Decision) valid() bool {
	return d == ReadyForProof || d == RecreateArtwork || d == AskCustomer
}

// Result is the agent's answer for one order.
type Result struct {
	Decision     Decision             `json:"decision"`
	Reasons      []string             `json:"reasons"`
	DesignerNote string               `json:"designer_note"`
	CustomerNote string               `json:"customer_note"`
	Preflight    artwork.Report       `json:"preflight"`
	Lint         []notelint.Violation `json:"lint,omitempty"`         // rules the final note still breaks
	Repaired     []notelint.Violation `json:"repaired,omitempty"`     // rules the first draft broke
	RepairError  string               `json:"repair_error,omitempty"` // the repair call failed; the first answer stands
	Model        string               `json:"model"`
	Attempts     int                  `json:"attempts"`
	CostUSD      float64              `json:"cost_usd"`
	Latency      time.Duration        `json:"latency_ns"`
}

// Completer is the one model call the agent needs.
type Completer interface {
	Complete(context.Context, llm.Request) (llm.Response, error)
}

// Agent reviews artwork with one model.
type Agent struct {
	Model string
	LLM   Completer
}

type answer struct {
	Decision     Decision `json:"decision"`
	Reasons      []string `json:"reasons"`
	DesignerNote string   `json:"designer_note"`
	CustomerNote string   `json:"customer_note"`
}

// Review runs the preflight, asks the model for a decision and checks the customer note.
func (a Agent) Review(ctx context.Context, img image.Image, o artwork.Order) (Result, error) {
	report := artwork.Preflight(img, o)
	png, err := previewPNG(img)
	if err != nil {
		return Result{}, err
	}
	prompt, err := userPrompt(o, report)
	if err != nil {
		return Result{}, err
	}

	res := Result{Preflight: report, Model: a.Model}
	call := func(text string) (answer, error) {
		res.Attempts++
		resp, err := a.LLM.Complete(ctx, llm.Request{
			Model: a.Model, System: systemPrompt, Text: text, ImagePNG: png,
			Schema: answerSchema, SchemaName: "proof_decision", MaxTokens: 2000,
		})
		res.CostUSD += resp.CostUSD
		res.Latency += resp.Latency
		if err != nil {
			return answer{}, err
		}
		var ans answer
		if err := json.Unmarshal(resp.JSON, &ans); err != nil {
			return answer{}, fmt.Errorf("%s: %w", a.Model, err)
		}
		if !ans.Decision.valid() {
			return answer{}, fmt.Errorf("%s: unknown decision %q", a.Model, ans.Decision)
		}
		return ans, nil
	}

	ans, err := call(prompt)
	if err != nil {
		return res, err
	}
	res.Decision, res.Reasons = ans.Decision, ans.Reasons
	res.DesignerNote, res.CustomerNote = ans.DesignerNote, ans.CustomerNote
	res.Lint = notelint.Check(ans.CustomerNote, o.Customer)
	if len(res.Lint) == 0 {
		return res, nil
	}

	// One repair, for the note only: the decision was already valid and stays.
	res.Repaired = res.Lint
	fixed, err := call(prompt + repairPrompt(ans.CustomerNote, res.Lint))
	if err != nil {
		res.RepairError = err.Error()
		return res, nil
	}
	res.CustomerNote = fixed.CustomerNote
	res.Lint = notelint.Check(fixed.CustomerNote, o.Customer)
	return res, nil
}

// Baseline is the rules-only reviewer every model has to beat. It reads the
// preflight but never the customer's comment.
func Baseline(img image.Image, o artwork.Order) Result {
	report := artwork.Preflight(img, o)
	name := firstName(o.Customer)
	res := Result{Preflight: report, Model: "baseline-rules", Attempts: 1}
	switch {
	case report.Has(artwork.EmptyArtwork):
		res.Decision = AskCustomer
		res.CustomerNote = fmt.Sprintf("Thanks %s. Upload your artwork and we'll make your proof.", name)
	case report.Has(artwork.LowResolution):
		res.Decision = RecreateArtwork
		res.CustomerNote = fmt.Sprintf("Thanks %s. We're redrawing your artwork so it prints sharp. Your proof follows soon.", name)
	default:
		res.Decision = ReadyForProof
		res.CustomerNote = fmt.Sprintf("Thanks %s. Your proof is on the way.", name)
	}
	for _, f := range report.Findings {
		res.Reasons = append(res.Reasons, f.Message)
	}
	res.Lint = notelint.Check(res.CustomerNote, o.Customer)
	return res
}

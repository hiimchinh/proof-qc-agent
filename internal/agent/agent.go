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
	Lint         []notelint.Violation `json:"lint,omitempty"`     // rules the final note still breaks
	Repaired     []notelint.Violation `json:"repaired,omitempty"` // rules earlier drafts broke
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

// maxAttempts allows one repair when the customer note breaks a support principle.
const maxAttempts = 2

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
	for res.Attempts < maxAttempts {
		res.Attempts++
		resp, err := a.LLM.Complete(ctx, llm.Request{
			Model: a.Model, System: systemPrompt, Text: prompt, ImagePNG: png,
			Schema: answerSchema, SchemaName: "proof_decision", MaxTokens: 2000,
		})
		res.CostUSD += resp.CostUSD
		res.Latency += resp.Latency
		if err != nil {
			return res, err
		}
		var ans answer
		if err := json.Unmarshal(resp.JSON, &ans); err != nil {
			return res, fmt.Errorf("%s: %w", a.Model, err)
		}
		if !ans.Decision.valid() {
			return res, fmt.Errorf("%s: unknown decision %q", a.Model, ans.Decision)
		}
		res.Decision, res.Reasons = ans.Decision, ans.Reasons
		res.DesignerNote, res.CustomerNote = ans.DesignerNote, ans.CustomerNote
		res.Lint = notelint.Check(ans.CustomerNote, o.Customer)
		if len(res.Lint) == 0 {
			break
		}
		if res.Attempts < maxAttempts {
			res.Repaired = append(res.Repaired, res.Lint...)
		}
		prompt += repairPrompt(ans.CustomerNote, res.Lint)
	}
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

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/notelint"
)

func (r *deskRun) route(o artwork.Order, ans routeArguments) error {
	res := r.result
	res.Decision, res.Reasons = ans.Decision, ans.Reasons
	res.DesignerNote, res.CustomerNote = ans.DesignerNote, ans.CustomerNote
	if changes := orderChanges(o, ans.Requested); len(changes) > 0 {
		res.OrderChanges = changes
		res.Decision = AskCustomer
		res.Reasons = append(res.Reasons, "order change: "+strings.Join(changes, "; "))
		res.CustomerNote = orderChangeNote(o.Customer, changes)
		res.OverriddenByRules = true
	}
	decision := res.Decision
	switch {
	case res.Preflight.Has(artwork.EmptyArtwork):
		res.Decision = AskCustomer
	case res.Preflight.Has(artwork.LowResolution) && decision != AskCustomer:
		res.Decision = RecreateArtwork
	}
	if res.Decision != decision {
		res.OverriddenByRules = true
		// Physical rules use the same customer wording as the rules-only reviewer.
		if res.Decision == AskCustomer {
			res.CustomerNote = fmt.Sprintf("Thanks %s. Upload your artwork and we'll make your proof.", firstName(o.Customer))
		} else {
			res.CustomerNote = fmt.Sprintf("Thanks %s. We're redrawing your artwork so it prints sharp. Your proof follows soon.", firstName(o.Customer))
		}
		for _, finding := range res.Preflight.Findings {
			res.Reasons = append(res.Reasons, finding.Message)
		}
	}
	if ans.customerNoteNull && !res.OverriddenByRules {
		return fmt.Errorf("customer_note is required and must not be null unless supplied by rules")
	}
	res.Lint = notelint.Check(res.CustomerNote, o.Customer)
	r.result = res
	return nil
}

func (r *deskRun) repair(ctx context.Context, o artwork.Order) {
	res := &r.result
	if len(res.Lint) == 0 {
		return
	}
	res.Repaired = res.Lint
	prompt, err := userPrompt(o, res.Preflight)
	if err != nil {
		res.RepairError = err.Error()
		return
	}
	args, _ := json.Marshal(map[string]any{"customer_note": res.CustomerNote, "violations": res.Lint})
	attempts := res.Attempts
	defer func() {
		if res.Attempts == attempts {
			return
		}
		result := fmt.Sprintf("note repair; lint violations: %d", len(res.Lint))
		if res.RepairError != "" {
			result = res.RepairError
		}
		res.Trace = append(res.Trace, TraceStep{Tool: "repair_note", Args: args, Result: result})
	}()
	resp, err := r.call(ctx, llm.Request{Model: r.desk.Model, System: systemPrompt, Text: prompt + repairPrompt(res.CustomerNote, res.Lint), Schema: answerSchema, SchemaName: "proof_decision", MaxTokens: 2000})
	if err != nil {
		res.RepairError = err.Error()
		return
	}
	if resp.FinishReason != "stop" {
		res.RepairError = fmt.Sprintf("expected stop, got finish_reason %q", resp.FinishReason)
		return
	}
	var fixed answer
	if err := json.Unmarshal(resp.JSON, &fixed); err != nil {
		res.RepairError = err.Error()
		return
	}
	// A repair can change only the note, never routing or extracted order changes.
	res.CustomerNote = fixed.CustomerNote
	res.Lint = notelint.Check(res.CustomerNote, o.Customer)
}

package eval

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolScorecard checks observable tool discipline separately from decision accuracy.
type ToolScorecard struct {
	Cases               int              `json:"cases"`
	MeasuredBeforeRoute int              `json:"measured_before_route"`
	RuleOverrides       int              `json:"rule_overrides"`
	ConsistentOverrides int              `json:"consistent_overrides"`
	BoundedSteps        int              `json:"bounded_steps"`
	TotalCalls          int              `json:"total_calls"`
	Violations          []TraceViolation `json:"violations,omitempty"`
}

// TraceViolation names the case and failed trace contracts.
type TraceViolation struct {
	ID      string   `json:"id"`
	Reasons []string `json:"reasons"`
}

// ScoreTools counts only cases that ran. A failed order cannot pass trace checks.
func ScoreTools(outs []Outcome, maxSteps int) ToolScorecard {
	if maxSteps <= 0 {
		maxSteps = 6
	}
	var s ToolScorecard
	for _, o := range outs {
		if o.Skipped {
			continue
		}
		s.Cases++
		s.TotalCalls += o.Result.Attempts
		reasons := []string{}
		measured, routed, override, valid := false, false, false, true
		routeCount := 0
		for i, step := range o.Result.Trace {
			if !json.Valid(step.Args) || step.Result == "" {
				valid = false
			}
			switch step.Tool {
			case "measure_artwork":
				if !routed {
					measured = true
				}
			case "customer_history":
				if routed {
					valid = false
				}
			case "repair_note":
				if i != len(o.Result.Trace)-1 || i == 0 || o.Result.Trace[i-1].Tool != "route_order" {
					valid = false
				}
			case "route_order":
				routeCount++
				terminal := i == len(o.Result.Trace)-1 || (i == len(o.Result.Trace)-2 && o.Result.Trace[i+1].Tool == "repair_note")
				routed = measured && terminal
				override = step.Overridden
			default:
				valid = false
			}
		}
		if o.Err != "" {
			reasons = append(reasons, "order failed: "+o.Err)
		}
		if routeCount == 1 && routed && o.Err == "" {
			s.MeasuredBeforeRoute++
		} else {
			reasons = append(reasons, "measure_artwork must precede the terminal route_order")
		}
		if o.Result.OverriddenByRules {
			s.RuleOverrides++
		}
		if routeCount == 1 && override == o.Result.OverriddenByRules && o.Err == "" {
			s.ConsistentOverrides++
		} else {
			reasons = append(reasons, "override flags disagree or route is missing")
		}
		if o.Result.Attempts > 0 && o.Result.Attempts <= maxSteps && len(o.Result.Trace) <= maxSteps {
			s.BoundedSteps++
		} else {
			reasons = append(reasons, "call count exceeds bounds")
		}
		if !valid {
			reasons = append(reasons, "invalid tool trace")
		}
		if len(reasons) > 0 {
			s.Violations = append(s.Violations, TraceViolation{ID: o.Case.ID, Reasons: reasons})
		}
	}
	return s
}

// ToolsMarkdown adds tool checks without changing the structured eval report format.
func ToolsMarkdown(model string, s ToolScorecard) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n### Tool trace: %s\n\n", model)
	fmt.Fprintf(&b, "| Measured before route | Consistent overrides | Within call limit | Rule overrides | Total calls |\n")
	fmt.Fprintf(&b, "|---:|---:|---:|---:|---:|\n| %d/%d | %d/%d | %d/%d | %d | %d |\n", s.MeasuredBeforeRoute, s.Cases, s.ConsistentOverrides, s.Cases, s.BoundedSteps, s.Cases, s.RuleOverrides, s.TotalCalls)
	for _, v := range s.Violations {
		fmt.Fprintf(&b, "\n- %s: %s\n", v.ID, strings.Join(v.Reasons, "; "))
	}
	return b.String()
}

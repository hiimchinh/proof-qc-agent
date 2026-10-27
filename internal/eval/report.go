package eval

import (
	"fmt"
	"strings"
	"time"
)

// Markdown renders the leaderboard with a keep/retire verdict per model.
func Markdown(baseline Scorecard, cards []Scorecard, p Policy) string {
	var b strings.Builder
	fmt.Fprintf(&b, "| Reviewer | Accuracy | Notes follow support principles | $ / proof | p50 latency | Verdict |\n")
	fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---|\n")
	row := func(s Scorecard, verdict string) {
		acc := fmt.Sprintf("%d/%d (%.0f%%)", s.Correct, s.Cases, s.Accuracy()*100)
		if s.Runs() > 1 {
			lo, hi := s.AccuracyRange()
			acc = fmt.Sprintf("%.0f%% (%.0f–%.0f)", s.Accuracy()*100, lo*100, hi*100)
		}
		fmt.Fprintf(&b, "| `%s` | %s | %.0f%% | $%.4f | %s | %s |\n",
			s.Model, acc, s.LintPassRate()*100,
			s.CostPerProof(), s.P50().Round(100*time.Millisecond), verdict)
	}
	row(baseline, "reference")
	for _, s := range cards {
		keep, reasons := p.Verdict(s, baseline)
		v := "**keep**"
		if !keep {
			v = "retire: " + strings.Join(reasons, "; ")
		}
		row(s, v)
	}

	b.WriteString("\n<details><summary>Misses</summary>\n\n")
	for _, s := range append([]Scorecard{baseline}, cards...) {
		if len(s.Misses) == 0 {
			continue
		}
		fmt.Fprintf(&b, "**%s**\n\n", s.Model)
		for _, m := range s.Misses {
			if m.Err != "" {
				fmt.Fprintf(&b, "- `%s`: error: %s\n", m.ID, m.Err)
				continue
			}
			fmt.Fprintf(&b, "- `%s`: wanted %s, got %s\n", m.ID, m.Want, m.Got)
		}
		b.WriteString("\n")
	}
	b.WriteString("</details>\n")
	return b.String()
}

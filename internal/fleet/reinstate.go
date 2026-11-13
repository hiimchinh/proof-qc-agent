package fleet

import (
	"fmt"
	"strings"

	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

// failureStreakReason marks a retirement by MaxConsecutiveFailures in its event reason.
const failureStreakReason = " consecutive failed calls"

// Reinstate undoes a failure-streak retirement that the current classifier
// would not make, such as provider credit errors an older build counted as
// model failures. latest holds each model's most recent event. A model goes
// back only to the seat it last ran in, and only while that seat is empty; the
// next Evaluate then applies the usual policy to it.
func Reinstate(state store.FleetState, runs []store.AgentRun, latest []store.FleetEvent) (store.FleetState, []store.FleetEvent) {
	var events []store.FleetEvent
	for _, e := range latest {
		if e.Kind != "retired" || !strings.Contains(e.Reason, failureStreakReason) || e.Model == state.Champion || e.Model == state.Challenger {
			continue
		}
		if failures, _ := consecutiveFailures(runs, e.Model); failures >= MaxConsecutiveFailures {
			continue
		}
		role := lastRole(runs, e.Model)
		var seat *string
		switch role {
		case "champion":
			seat = &state.Champion
		case "challenger":
			seat = &state.Challenger
		}
		if seat == nil || *seat != "" {
			continue
		}
		*seat = e.Model
		events = append(events, store.FleetEvent{Kind: "reinstated", Model: e.Model,
			Reason: fmt.Sprintf("its failed calls were provider errors, not model failures; back in the %s seat", role)})
	}
	return state, events
}

func lastRole(runs []store.AgentRun, model string) string {
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].Model == model {
			return runs[i].Role
		}
	}
	return ""
}

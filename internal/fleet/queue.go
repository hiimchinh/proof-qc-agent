package fleet

import "github.com/hiimchinh/proof-qc-agent/internal/store"

const DefaultQueue = "google/gemma-4-31b-it,qwen/qwen3-vl-235b-a22b-instruct,x-ai/grok-4.7"

// RemainingQueue excludes the current seats and every model in persisted history.
// History is unbounded: even models retired long ago must never return.
func RemainingQueue(state store.FleetState, history, queue []string) []string {
	seen := map[string]bool{"": true, state.Champion: true, state.Challenger: true}
	for _, model := range history {
		seen[model] = true
	}
	remaining := make([]string, 0, len(queue))
	for _, model := range queue {
		if !seen[model] {
			remaining = append(remaining, model)
			seen[model] = true
		}
	}
	return remaining
}

// SeatNext is pure; the caller persists its event with any promotion/retirement.
func SeatNext(state store.FleetState, history, queue []string) (store.FleetState, []store.FleetEvent) {
	if state.Challenger != "" {
		return state, nil
	}
	remaining := RemainingQueue(state, history, queue)
	if len(remaining) == 0 {
		return state, nil
	}
	state.Challenger = remaining[0]
	return state, []store.FleetEvent{{Kind: "added", Model: state.Challenger, Reason: "next candidate from the queue"}}
}

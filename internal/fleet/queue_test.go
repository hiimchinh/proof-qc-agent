package fleet_test

import (
	"reflect"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

func TestQueueSkipsEveryPreviouslySeenModel(t *testing.T) {
	state := store.FleetState{Champion: "champion", Challenger: "challenger"}
	queue := []string{"retired", "champion", "added-before", "challenger", "google/gemma-3-27b-it", "google/gemma-3-27b-it", "x-ai/grok-4.7"}
	history := []string{"retired", "added-before"}
	want := []string{"google/gemma-3-27b-it", "x-ai/grok-4.7"}
	if got := fleet.RemainingQueue(state, history, queue); !reflect.DeepEqual(got, want) {
		t.Fatalf("queue=%v; want %v", got, want)
	}
	if queue[0] != "retired" || history[0] != "retired" {
		t.Fatal("selection mutated its inputs")
	}
}

func TestSeatNextOnlyFillsAnEmptyChallengerSeat(t *testing.T) {
	for _, tc := range []struct {
		name           string
		state          store.FleetState
		history, queue []string
		want           store.FleetState
		added          string
	}{
		{"vacant", store.FleetState{Champion: "a"}, []string{"old"}, []string{"old", "a", "b", "c"}, store.FleetState{Champion: "a", Challenger: "b"}, "b"},
		{"both vacant", store.FleetState{}, []string{"old"}, []string{"old", "b"}, store.FleetState{Challenger: "b"}, "b"},
		{"occupied", store.FleetState{Champion: "a", Challenger: "b"}, nil, []string{"c"}, store.FleetState{Champion: "a", Challenger: "b"}, ""},
		{"exhausted", store.FleetState{Champion: "a"}, []string{"old"}, []string{"a", "old"}, store.FleetState{Champion: "a"}, ""},
		{"empty", store.FleetState{}, nil, nil, store.FleetState{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, events := fleet.SeatNext(tc.state, tc.history, tc.queue)
			if got != tc.want {
				t.Fatalf("state=%+v; want %+v", got, tc.want)
			}
			if tc.added == "" {
				if len(events) != 0 {
					t.Fatalf("unexpected events %+v", events)
				}
				return
			}
			if len(events) != 1 || events[0].Kind != "added" || events[0].Model != tc.added || events[0].Reason != "next candidate from the queue" {
				t.Fatalf("events=%+v", events)
			}
		})
	}
}

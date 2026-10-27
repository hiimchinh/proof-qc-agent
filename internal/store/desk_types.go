package store

import (
	"encoding/json"
	"time"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
)

type DeskOrder struct {
	ID                   int64
	CaseID               string
	Order                artwork.Order
	Expected             agent.Decision
	Status               string
	CreatedAt, UpdatedAt time.Time
	Runs                 []AgentRun
}

type AgentRun struct {
	ID, OrderID        int64
	Role, Model        string
	Result             agent.Result
	Expected           agent.Decision
	Correct, NotesPass bool
	CostUSD            float64
	Latency            time.Duration
	Trace              json.RawMessage
	OverriddenByRules  bool
	Error              string
	CreatedAt          time.Time
}

type FleetState struct{ Champion, Challenger string }
type FleetEvent struct {
	ID                  int64
	Kind, Model, Reason string
	Numbers             json.RawMessage
	CreatedAt           time.Time
}

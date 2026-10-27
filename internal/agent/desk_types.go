package agent

import (
	"context"
	"encoding/json"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
)

// HistoryEntry is a previous successfully completed order for this customer.
type HistoryEntry struct {
	Order    artwork.Order `json:"order"`
	Decision Decision      `json:"decision"`
}

// TraceStep records a tool's supplied arguments and its observed result.
type TraceStep struct {
	Tool       string          `json:"tool"`
	Args       json.RawMessage `json:"args"`
	Result     string          `json:"result"`
	Overridden bool            `json:"overridden"`
}

// Desk triages an order through a bounded tool conversation.
type Desk struct {
	Model      string
	LLM        Completer
	History    func(context.Context, string) ([]HistoryEntry, error)
	BeforeCall func(context.Context) error
	MaxSteps   int
}

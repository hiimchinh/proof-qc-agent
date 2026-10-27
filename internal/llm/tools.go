package llm

import (
	"encoding/json"
	"fmt"
)

// Message follows the OpenAI chat message contract, including tool responses.
type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool advertises one callable function to the model.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a tool's JSON arguments.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall is the assistant's request to execute a function.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the function name and its JSON-encoded argument string.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func validateToolCalls(calls []ToolCall, tools []Tool) error {
	ids := map[string]bool{}
	for _, c := range calls {
		if c.Type != "function" || c.ID == "" || ids[c.ID] || !json.Valid([]byte(c.Function.Arguments)) {
			return fmt.Errorf("invalid tool call %q", c.ID)
		}
		ids[c.ID] = true
		found := false
		for _, t := range tools {
			if t.Type == "function" && t.Function.Name == c.Function.Name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unsupported tool %q", c.Function.Name)
		}
	}
	return nil
}

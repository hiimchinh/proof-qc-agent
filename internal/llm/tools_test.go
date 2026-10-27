package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCompleteToolConversation(t *testing.T) {
	var seen map[string]any
	c := fakeServer(t, 200, `{"choices":[{"finish_reason":"tool_calls","message":{"content":null,"tool_calls":[{"id":"m1","type":"function","function":{"name":"measure_artwork","arguments":"{}"}}]}}],"usage":{"prompt_tokens":12,"completion_tokens":5,"cost":0.002}}`, &seen)
	res, err := c.Complete(context.Background(), Request{Model: "offline", System: "triage", Text: "order", ImagePNG: []byte("png"), Tools: []Tool{{Type: "function", Function: ToolFunction{Name: "measure_artwork", Parameters: json.RawMessage(`{"type":"object"}`)}}}, Messages: []Message{{Role: "assistant", ToolCalls: []ToolCall{{ID: "old", Type: "function", Function: FunctionCall{Name: "measure_artwork", Arguments: "{}"}}}}, {Role: "tool", Content: "measured", ToolCallID: "old"}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinishReason != "tool_calls" || len(res.ToolCalls) != 1 || res.ToolCalls[0].Function.Name != "measure_artwork" || res.CostUSD != 0.002 {
		t.Fatalf("response = %+v", res)
	}
	msgs := seen["messages"].([]any)
	if len(msgs) != 4 || msgs[3].(map[string]any)["tool_call_id"] != "old" {
		t.Fatalf("messages = %#v", msgs)
	}
	if len(seen["tools"].([]any)) != 1 || seen["response_format"] != nil {
		t.Fatalf("request = %#v", seen)
	}
	provider, ok := seen["provider"].(map[string]any)
	if !ok || provider["require_parameters"] != true {
		t.Fatalf("tool providers must honour requested parameters: %#v", seen["provider"])
	}
}

func TestCompleteKeepsUsageForRejectedResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"http", `{"error":{"message":"rate limited"},"usage":{"cost":0.01,"prompt_tokens":7}}`, 429},
		{"no choices", `{"choices":[],"usage":{"cost":0.01,"prompt_tokens":7}}`, 200},
		{"filter", `{"choices":[{"finish_reason":"content_filter","message":{"content":""}}],"usage":{"cost":0.01,"prompt_tokens":7}}`, 200},
		{"unknown finish", `{"choices":[{"finish_reason":"surprise","message":{"content":"{}"}}],"usage":{"cost":0.01,"prompt_tokens":7}}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fakeServer(t, tc.status, tc.body, nil)
			res, err := c.Complete(context.Background(), Request{Model: "offline"})
			if err == nil || res.CostUSD != 0.01 || res.InputTokens != 7 {
				t.Fatalf("response=%+v err=%v", res, err)
			}
		})
	}
}

func TestCompleteRejectsUnknownToolWithoutRetry(t *testing.T) {
	c := fakeServer(t, 200, `{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"x","type":"function","function":{"name":"delete_order","arguments":"{}"}}]}}],"usage":{"cost":0.01}}`, nil)
	res, err := c.Complete(context.Background(), Request{Model: "offline", Tools: []Tool{{Type: "function", Function: ToolFunction{Name: "measure_artwork", Parameters: json.RawMessage(`{}`)}}}})
	if err == nil || !strings.Contains(err.Error(), "tool") || res.CostUSD != 0.01 {
		t.Fatalf("response=%+v err=%v", res, err)
	}
}

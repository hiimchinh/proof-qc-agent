package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCompleteReportsProviderErrorDetail(t *testing.T) {
	for _, status := range []int{200, 502} {
		for _, raw := range []any{
			`{"error":{"message":"tool calling is unsupported"}}`,
			map[string]any{"error": map[string]any{"message": "tool calling is unsupported"}},
		} {
			body, err := json.Marshal(map[string]any{
				"error": map[string]any{"message": "Provider returned error", "metadata": map[string]any{"provider_name": "ExampleProvider", "raw": raw}},
				"usage": map[string]any{"cost": 0.001, "prompt_tokens": 9},
			})
			if err != nil {
				t.Fatal(err)
			}
			c := fakeServer(t, status, string(body), nil)
			res, err := c.Complete(context.Background(), Request{Model: "google/gemma-3-27b-it", Tools: []Tool{{Type: "function", Function: ToolFunction{Name: "measure_artwork", Parameters: json.RawMessage(`{}`)}}}})
			if err == nil || !strings.Contains(err.Error(), "ExampleProvider") || !strings.Contains(err.Error(), "tool calling is unsupported") || !strings.Contains(err.Error(), "google/gemma-3-27b-it: Provider returned error") || res.CostUSD != 0.001 || res.InputTokens != 9 {
				t.Fatalf("response=%+v err=%v", res, err)
			}
		}
	}
}

func TestCompleteBoundsAndRedactsProviderError(t *testing.T) {
	secret := "sk-or-test-secret"
	body, err := json.Marshal(map[string]any{"error": map[string]any{
		"message": "Provider returned error " + secret,
		"metadata": map[string]any{
			"provider_name": "ExampleProvider " + secret,
			"raw":           "tool calling unsupported\nAuthorization: Bearer upstream-secret api_key=other-secret " + secret + strings.Repeat("x", 2000),
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	c := fakeServer(t, 502, string(body), nil)
	c.APIKey = secret
	_, err = c.Complete(context.Background(), Request{Model: "offline"})
	if err == nil {
		t.Fatal("expected provider error")
	}
	msg := err.Error()
	for _, forbidden := range []string{secret, "upstream-secret", "other-secret", "\n"} {
		if strings.Contains(msg, forbidden) {
			t.Fatalf("error leaked a secret or control character")
		}
	}
	if !strings.Contains(msg, "tool calling unsupported") || !strings.Contains(msg, "...") || len(msg) > 1000 {
		t.Fatalf("missing or unbounded detail: %s", msg)
	}
}

func TestCompleteRedactsQuotedUpstreamCredentials(t *testing.T) {
	detail := map[string]any{"message": "tools unsupported", "password": "prefix,secret-tail with spaces", "Authorization": "Basic dXNlcjpwYXNz"}
	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []any{detail, string(encoded)} {
		body, err := json.Marshal(map[string]any{"error": map[string]any{"message": "Provider returned error", "metadata": map[string]any{"raw": raw}}})
		if err != nil {
			t.Fatal(err)
		}
		c := fakeServer(t, 502, string(body), nil)
		_, err = c.Complete(context.Background(), Request{Model: "offline"})
		if err == nil {
			t.Fatal("expected provider error")
		}
		msg := err.Error()
		if !strings.Contains(msg, "tools unsupported") || strings.Contains(msg, "secret-tail") || strings.Contains(msg, "dXNlcjpwYXNz") {
			t.Fatal("upstream credentials leaked or diagnostic message lost")
		}
	}
}

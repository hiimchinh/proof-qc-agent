package llm

import (
	"context"
	"testing"
)

// openRouterCreditError is the error persisted when OpenRouter ran out of
// credits on 2026-11-06 and the champion was retired for it.
const openRouterCreditError = "google/gemma-4-31b-it: This request requires more credits, or fewer max_tokens. You requested up to 2000 tokens, but can only afford 1380. To increase, visit ht..."

func TestIsAccountCreditError(t *testing.T) {
	for _, text := range []string{
		"HTTP 402: Payment Required", "request failed: status 402", "HTTP 402 Payment Required api_key=secret",
		"x-ai/grok-4.7: This request would exceed your available credits given your current in-flight requests",
		"INSUFFICIENT CREDITS", "Your credit balance is too low", openRouterCreditError,
	} {
		if !IsAccountCreditError(text) {
			t.Errorf("not classified: %s", text)
		}
		if got := ShortErrorCause(text); got != "provider account is out of credits" {
			t.Errorf("cause=%q", got)
		}
	}
	for _, text := range []string{"", `expected tool_calls, got finish_reason "stop"`, "status 429 rate limited", "timeout", "HTTP 4020", "status 4021"} {
		if IsAccountCreditError(text) {
			t.Errorf("misclassified: %s", text)
		}
	}
}

func TestCompleteKeepsTheCreditStatusInTheError(t *testing.T) {
	// The message alone names no credit phrase, so only the status can classify it.
	body := `{"error":{"code":402,"message":"Insufficient balance"}}`
	for name, status := range map[string]int{"HTTP 402": 402, "402 in the body of an HTTP 200": 200} {
		t.Run(name, func(t *testing.T) {
			_, err := fakeServer(t, status, body, nil).Complete(context.Background(), Request{Model: "google/gemma-4-31b-it"})
			if err == nil || !IsAccountCreditError(err.Error()) {
				t.Fatalf("err=%v, want an account credit error", err)
			}
		})
	}
}

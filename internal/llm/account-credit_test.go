package llm

import "testing"

func TestIsAccountCreditError(t *testing.T) {
	for _, text := range []string{
		"HTTP 402: Payment Required", "request failed: status 402", "HTTP 402 Payment Required api_key=secret",
		"x-ai/grok-4.7: This request would exceed your available credits given your current in-flight requests",
		"INSUFFICIENT CREDITS", "Your credit balance is too low",
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

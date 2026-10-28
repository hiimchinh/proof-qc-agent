package llm

import "testing"

func TestIsRateLimitError(t *testing.T) {
	for _, text := range []string{"HTTP 429", "HTTP/1.1 429 Too Many Requests", "status 429", "status: 429", "RATE LIMIT exceeded", "temporarily rate-limited upstream", "Rate limit exceeded: free-models-per-day"} {
		if !IsRateLimitError(text) {
			t.Errorf("not classified: %s", text)
		}
		if got := ShortErrorCause(text); got != "rate-limited upstream" {
			t.Errorf("cause=%q", got)
		}
	}
	for _, text := range []string{"", `expected tool_calls, got finish_reason "stop"`, "timeout", "HTTP 402", "insufficient credits", "HTTP 4290", "status 4291"} {
		if IsRateLimitError(text) {
			t.Errorf("misclassified: %s", text)
		}
	}
}

func TestRateLimitCausePreservesProvider(t *testing.T) {
	if got := ShortErrorCause("Provider returned error; provider: DeepInfra; upstream: temporarily rate-limited upstream"); got != "provider DeepInfra rate-limited upstream" {
		t.Fatalf("cause=%q", got)
	}
}

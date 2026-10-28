package llm

import (
	"encoding/json"
	"regexp"
	"strings"
)

type providerError struct {
	Message  string `json:"message"`
	Metadata struct {
		Raw          json.RawMessage `json:"raw"`
		ProviderName string          `json:"provider_name"`
	} `json:"metadata"`
}

func (e *providerError) detail(apiKey string) string {
	msg := safeErrorText(e.Message, apiKey, 300)
	if provider := safeErrorText(e.Metadata.ProviderName, apiKey, 100); provider != "" {
		msg += "; provider: " + provider
	}
	raw := e.Metadata.Raw
	if len(raw) != 0 && string(raw) != "null" {
		text := string(raw)
		// OpenRouter usually wraps the upstream JSON in a string, but some
		// providers return an object instead.
		_ = json.Unmarshal(raw, &text)
		if text = safeErrorText(text, apiKey, 300); text != "" {
			msg += "; upstream: " + text
		}
	}
	return msg
}

var bearerSecret = regexp.MustCompile(`(?i)(bearer\s+)[^\s"',;}]+`)
var namedSecret = regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|token|password|authorization)["']?\s*[:=]\s*)(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s"',;}]+)`)

func safeErrorText(text, apiKey string, limit int) string {
	// Redact before truncating so a cut-off credential cannot escape matching.
	if apiKey != "" {
		text = strings.ReplaceAll(text, apiKey, "[redacted]")
	}
	text = bearerSecret.ReplaceAllString(text, "${1}[redacted]")
	text = namedSecret.ReplaceAllString(text, "${1}[redacted]")
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > limit {
		text = string(runes[:limit]) + "..."
	}
	return text
}

// ShortErrorCause summarizes persisted call errors for fleet events and the desk.
// Provider errors are already key-redacted by Client; redact named credentials
// again before truncation so older or externally supplied errors stay safe.
func ShortErrorCause(text string) string {
	if IsAccountCreditError(text) {
		return "provider account is out of credits"
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "rate-limited") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "status 429") {
		provider := ""
		if _, rest, ok := strings.Cut(text, "; provider: "); ok {
			provider, _, _ = strings.Cut(rest, ";")
		}
		if provider != "" {
			return "provider " + safeErrorText(provider, "", 60) + " rate-limited upstream"
		}
		return "rate-limited upstream"
	}
	return safeErrorText(text, "", 160)
}

var accountCreditStatus = regexp.MustCompile(`(?i)\b(?:http(?:/\d(?:\.\d)?)?|status)\s*:?\s*402\b`)

// IsAccountCreditError identifies account billing failures, not model failures.
func IsAccountCreditError(text string) bool {
	lower := strings.ToLower(text)
	return accountCreditStatus.MatchString(text) ||
		strings.Contains(lower, "exceed your available credits") ||
		strings.Contains(lower, "insufficient credits") ||
		strings.Contains(lower, "your credit balance is too low")
}

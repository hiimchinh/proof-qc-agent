package llm

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

type providerError struct {
	// Code is OpenRouter's HTTP-style error code; a number, or a string from some providers.
	Code     json.RawMessage `json:"code"`
	Message  string          `json:"message"`
	Metadata struct {
		Raw          json.RawMessage `json:"raw"`
		ProviderName string          `json:"provider_name"`
	} `json:"metadata"`
}

// status prefers the code OpenRouter puts in the body, which can differ from
// the HTTP status (errors raised after streaming starts arrive with HTTP 200).
func (e *providerError) status(httpStatus int) int {
	code, err := strconv.Atoi(strings.Trim(string(e.Code), `"`))
	if err != nil || code < 100 || code > 599 {
		return httpStatus
	}
	return code
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
	if IsRateLimitError(text) {
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
		strings.Contains(lower, "requires more credits") ||
		strings.Contains(lower, "your credit balance is too low")
}

var rateLimitStatus = regexp.MustCompile(`(?i)\b(?:http(?:/\d(?:\.\d)?)?|status)\s*:?\s*429\b`)

// IsRateLimitError identifies provider capacity failures, not model failures.
func IsRateLimitError(text string) bool {
	if IsAccountCreditError(text) {
		return false
	}
	lower := strings.ToLower(text)
	return rateLimitStatus.MatchString(text) ||
		strings.Contains(lower, "rate-limited") ||
		strings.Contains(lower, "rate limit")
}

// IsProviderOutage keeps account and capacity failures out of model quality.
func IsProviderOutage(text string) bool {
	return IsAccountCreditError(text) || IsRateLimitError(text)
}

// Package notelint checks a customer note against the support principles in the
// Sticker Mule Guide (https://www.stickermule.com/guide#support-principles).
package notelint

import (
	"regexp"
	"strconv"
	"strings"
)

// Rule names one support principle.
type Rule string

const (
	Negative    Rule = "negative"    // avoid sorry, can't, won't, unfortunately
	Patronizing Rule = "patronizing" // "appreciate" often reads as insincere
	Exclamation Rule = "exclamation" // use exclamation points cautiously
	NoName      Rule = "no_name"     // use the customer's name
	TooLong     Rule = "too_long"    // respond in the fewest words possible
)

// MaxWords is the longest note a customer should have to read about a proof.
const MaxWords = 60

// Violation is one broken rule with the text that broke it.
type Violation struct {
	Rule   Rule   `json:"rule"`
	Detail string `json:"detail"`
}

var (
	negative    = regexp.MustCompile(`(?i)\b(sorry|unfortunately|can['’]t|cannot|won['’]t|unable|not able)\b`)
	patronizing = regexp.MustCompile(`(?i)\bappreciate[sd]?\b`)
)

// Check returns the broken rules in a fixed order.
func Check(note, customer string) []Violation {
	var vs []Violation
	if m := negative.FindString(note); m != "" {
		vs = append(vs, Violation{Negative, m})
	}
	if m := patronizing.FindString(note); m != "" {
		vs = append(vs, Violation{Patronizing, m})
	}
	if n := strings.Count(note, "!"); n > 1 {
		vs = append(vs, Violation{Exclamation, strings.Repeat("!", n)})
	}
	if first := firstName(customer); first != "" && !containsWord(note, first) {
		vs = append(vs, Violation{NoName, first})
	}
	if n := len(strings.Fields(note)); n > MaxWords {
		vs = append(vs, Violation{TooLong, strconv.Itoa(n) + " words"})
	}
	return vs
}

func firstName(full string) string {
	if f := strings.Fields(full); len(f) > 0 {
		return f[0]
	}
	return ""
}

func containsWord(text, word string) bool {
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`).MatchString(text)
}

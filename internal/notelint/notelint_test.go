package notelint

import (
	"slices"
	"strings"
	"testing"
)

func rules(vs []Violation) []Rule {
	var out []Rule
	for _, v := range vs {
		out = append(out, v.Rule)
	}
	return out
}

func TestCleanNotePasses(t *testing.T) {
	note := "Thanks Brian! We removed the white box so the cut follows your logo. Approve the proof to print."
	if vs := Check(note, "Brian Smith"); len(vs) != 0 {
		t.Fatalf("violations = %v, want none", vs)
	}
}

func TestSupportPrinciples(t *testing.T) {
	cases := []struct {
		note string
		want []Rule
	}{
		{"Sorry Brian, the file is blurry.", []Rule{Negative}},
		{"Brian, unfortunately we cannot print this.", []Rule{Negative}},
		{"Brian, we can't use this file.", []Rule{Negative}},
		{"Brian, we appreciate your patience.", []Rule{Patronizing}},
		{"Thanks Brian! Proof ready! Approve it!", []Rule{Exclamation}},
		{"Thanks! Your proof is ready.", []Rule{NoName}},
	}
	for _, c := range cases {
		if got := rules(Check(c.note, "Brian")); !slices.Equal(got, c.want) {
			t.Errorf("%q: rules = %v, want %v", c.note, got, c.want)
		}
	}
}

func TestTooLong(t *testing.T) {
	note := "Brian"
	for range 70 {
		note += " word"
	}
	if got := rules(Check(note, "Brian")); !slices.Equal(got, []Rule{TooLong}) {
		t.Fatalf("rules = %v, want too_long", got)
	}
}

func TestWordBoundaries(t *testing.T) {
	// Banned words inside a longer word are fine: "Cannotti" is a bakery, not "cannot".
	if vs := Check("Thanks Brian, your Cannotti Bakery proof is ready.", "Brian"); len(vs) != 0 {
		t.Fatalf("violations = %v, want none", vs)
	}
	// A first name inside a longer word does not count as using it.
	if got := rules(Check("Thanks! The Briana design is ready.", "Brian")); !slices.Equal(got, []Rule{NoName}) {
		t.Fatalf("rules = %v, want no_name", got)
	}
}

func TestNonASCIINames(t *testing.T) {
	for _, name := range []string{"José García", "Łukasz Nowak", "Zoë Hill", "Özil", "Nguyễn Văn An"} {
		first := strings.Fields(name)[0]
		if vs := Check("Thanks "+first+". Your proof is ready.", name); len(vs) != 0 {
			t.Errorf("%s: violations = %v, want none", name, vs)
		}
	}
}

package notelint

import (
	"slices"
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
	// "sorry" inside another word and "cannot" style words must match whole words only.
	if vs := Check("Thanks Brian, your proof is ready for the cantina sign.", "Brian"); len(vs) != 0 {
		t.Fatalf("violations = %v, want none", vs)
	}
}

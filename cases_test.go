package proofqcagent

import (
	"io/fs"
	"testing"
)

func TestCasesShipWithTheirArtwork(t *testing.T) {
	cases, art, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 26 {
		t.Fatalf("%d cases, want the full labelled set", len(cases))
	}
	for _, c := range cases {
		if _, err := fs.Stat(art, c.File); err != nil {
			t.Errorf("case %s: %v", c.ID, err)
		}
	}
}

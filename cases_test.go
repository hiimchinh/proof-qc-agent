package proofqcagent

import (
	"image/png"
	"io/fs"
	"strings"
	"testing"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/eval"
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

func TestRealTierComesFromPublicReviews(t *testing.T) {
	cases, art, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	wantWide := map[string]bool{
		"real-wide-3x3-quiet": true, "real-wide-3x3-expects-tall": true,
		"real-wide-4x4-width-ok": true, "real-wide-2x2-bigger": true,
	}
	real := 0
	for _, c := range cases {
		if eval.TierOf(c.ID) != eval.TierReal {
			if c.Source != "" {
				t.Errorf("synthetic case %s has a source", c.ID)
			}
			continue
		}
		real++
		if !strings.HasPrefix(c.Source, "https://www.stickermule.com/reviews/") {
			t.Errorf("case %s: source %q", c.ID, c.Source)
		}
		f, err := art.Open(c.File)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		// A wide die-cut design prints smaller than its box; nothing else in the tier should.
		if got := artwork.Preflight(img, c.Order).Has(artwork.PrintsSmaller); got != wantWide[c.ID] {
			t.Errorf("case %s: prints_smaller = %v, want %v", c.ID, got, wantWide[c.ID])
		}
	}
	if real != 17 {
		t.Fatalf("%d real cases, want 17", real)
	}
}
